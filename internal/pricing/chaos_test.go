package pricing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
	"github.com/rshade/finfocus-plugin-azure-public/internal/estimation"
)

// chaosPriceJSON is the Linux B1s Consumption row from
// testdata/retail/regions/standard_b1s_eastus.json. retailPrice is hourly.
const chaosPriceJSON = `{
  "BillingCurrency": "USD",
  "CustomerEntityId": "Default",
  "CustomerEntityType": "Retail",
  "Items": [{
    "currencyCode": "USD",
    "tierMinimumUnits": 0.0,
    "retailPrice": 0.0104,
    "unitPrice": 0.0104,
    "armRegionName": "eastus",
    "location": "US East",
    "effectiveStartDate": "2025-10-01T00:00:00Z",
    "meterId": "f4d7a5a5-1b67-45ea-b1a0-282fbdd34b05",
    "meterName": "B1s",
    "productId": "DZH318Z0BQ35",
    "skuId": "DZH318Z0BQ35/00SV",
    "productName": "Virtual Machines BS Series",
    "skuName": "B1s",
    "serviceName": "Virtual Machines",
    "serviceId": "DZH313Z7MMC8",
    "serviceFamily": "Compute",
    "unitOfMeasure": "1 Hour",
    "type": "Consumption",
    "isPrimaryMeterRegion": true,
    "armSkuName": "Standard_B1s"
  }],
  "NextPageLink": "",
  "Count": 1
}`

const chaosRetailPrice = 0.0104

func TestChaos(t *testing.T) {
	t.Run("Timeout", func(t *testing.T) {
		const clientTimeout = 200 * time.Millisecond
		items, requests, err := chaosGetPrices(t, clientTimeout, func(w http.ResponseWriter, r *http.Request, _ int) {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-r.Context().Done():
			case <-timer.C:
				http.Error(w, "handler delay expired before the client", http.StatusOK)
			}
		})
		// Client.Timeout is shorter than the handler delay, and RetryMax is 1,
		// so the timed-out attempt is retried. The stdlib timeout error matches
		// context.DeadlineExceeded, which MapToGRPCStatus checks before
		// ErrRequestFailed.
		assertChaosError(t, items, err, requests, false, codes.DeadlineExceeded)
	})

	t.Run("HTTP429", func(t *testing.T) {
		items, requests, err := chaosGetPrices(t, 2*time.Second, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
		})
		assertChaosError(t, items, err, requests, false, codes.ResourceExhausted)
	})

	t.Run("HTTP500", func(t *testing.T) {
		items, requests, err := chaosGetPrices(t, 2*time.Second, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		assertChaosError(t, items, err, requests, true, codes.Internal)
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		items, requests, err := chaosGetPrices(t, 2*time.Second, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("this is not json"))
		})
		assertChaosError(t, items, err, requests, true, codes.Internal)
	})

	t.Run("FailThenSucceed", func(t *testing.T) {
		items, requests, err := chaosGetPrices(
			t,
			2*time.Second,
			func(w http.ResponseWriter, _ *http.Request, attempt int) {
				if attempt == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(chaosPriceJSON))
			},
		)
		if requests != 2 {
			t.Fatalf("requests = %d, want exactly 2", requests)
		}
		if err != nil {
			t.Fatalf("GetPrices error = %v, want nil", err)
		}
		monthly, produced := chaosMonthly(items)
		if !produced || monthly <= 0 {
			t.Fatalf("monthly cost produced=%t value=%v, want a positive price", produced, monthly)
		}
		if len(items) != 1 || items[0].RetailPrice != chaosRetailPrice {
			t.Fatalf("items = %+v, want one row at retailPrice %v", items, chaosRetailPrice)
		}
		t.Logf(
			"requests=%d grpc=nil error produced_price=true retail=%v monthly=%v",
			requests,
			items[0].RetailPrice,
			monthly,
		)
	})
}

// chaosGetPrices points a client at handler. RetryMax is 1. Both retry waits
// are one millisecond, so only an honored Retry-After can sleep longer.
// attempt is the 1-based count of requests the server has accepted.
func chaosGetPrices(
	t *testing.T,
	timeout time.Duration,
	handler func(w http.ResponseWriter, r *http.Request, attempt int),
) ([]azureclient.PriceItem, int, error) {
	t.Helper()

	var seen atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, int(seen.Add(1)))
	}))
	t.Cleanup(server.Close)

	cfg := azureclient.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.RetryMax = 1
	cfg.RetryWaitMin = time.Millisecond
	cfg.RetryWaitMax = time.Millisecond
	cfg.Timeout = timeout
	cfg.Logger = zerolog.Nop()

	client, err := azureclient.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	items, err := client.GetPrices(context.Background(), azureclient.PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
	})
	return items, int(seen.Load()), err
}

// assertChaosError checks a failure. exact means the server must see one
// request. Otherwise RetryMax 1 must produce more than one request.
// An error is never turned into a monthly price, including a price of zero.
func assertChaosError(
	t *testing.T,
	items []azureclient.PriceItem,
	err error,
	requests int,
	exact bool,
	want codes.Code,
) {
	t.Helper()

	if exact {
		if requests != 1 {
			t.Fatalf("requests = %d, want exactly 1", requests)
		}
	} else if requests <= 1 {
		t.Fatalf("requests = %d, want more than 1", requests)
	}

	if err == nil {
		t.Fatal("GetPrices returned a price for a failed call")
	}
	if want == codes.OK {
		t.Fatal("error case must not expect codes.OK")
	}

	monthly, produced := chaosMonthly(items)
	if len(items) != 0 || produced {
		t.Fatalf("error produced monthly cost %v from %d rows; stop instead of pricing zero", monthly, len(items))
	}

	st := MapToGRPCStatus(err)
	if st.Code() == codes.OK {
		t.Fatalf("MapToGRPCStatus returned OK for %v", err)
	}
	if st.Code() != want {
		t.Fatalf("grpc code = %s, want %s (err: %v)", st.Code(), want, err)
	}
	t.Logf("requests=%d grpc=%s produced_price=false", requests, st.Code())
}

// chaosMonthly is the monthly figure a caller would report for a positive
// retail price. Empty or non-positive rows produce no price.
func chaosMonthly(items []azureclient.PriceItem) (float64, bool) {
	if len(items) == 0 || items[0].RetailPrice <= 0 {
		return 0, false
	}
	return estimation.HourlyToMonthly(items[0].RetailPrice), true
}
