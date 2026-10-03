//go:build integration

package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	blobRetailPricesURL   = "https://prices.azure.com/api/retail/prices"
	blobReferenceAttempts = 4
	blobReferenceTimeout  = 30 * time.Second
	blobQuoteTimeout      = 60 * time.Second
	blobPriceTolerance    = 0.01
)

// blobReferenceRow holds the Retail Prices API fields the reference reads.
// It is separate from the plugin's types so the reference does not reuse the
// plugin's selection code.
type blobReferenceRow struct {
	ProductName      string  `json:"productName"`
	SkuName          string  `json:"skuName"`
	MeterName        string  `json:"meterName"`
	UnitOfMeasure    string  `json:"unitOfMeasure"`
	RetailPrice      float64 `json:"retailPrice"`
	TierMinimumUnits float64 `json:"tierMinimumUnits"`
}

type blobReferencePage struct {
	Items        []blobReferenceRow `json:"Items"`
	NextPageLink string             `json:"NextPageLink"`
}

// TestGetProjectedCost_BlobHotZRS_MatchesLiveReference quotes a Hot ZRS blob
// in eastus. The legacy Blob Storage product sells no ZRS, so this checks the
// quote reads General Block Blob v2. The reference is the first-band
// "Hot ZRS Data Stored" row, read live when the test runs, so a price change
// cannot fail it.
func TestGetProjectedCost_BlobHotZRS_MatchesLiveReference(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)

	const (
		region = "eastus"
		sku    = "Hot ZRS"
		sizeGB = 100.0
	)

	perGB := fetchBlobReferencePrice(t, region, sku)
	want := perGB * sizeGB

	calc, _ := newTestCalculator(t)
	ctx, cancel := context.WithTimeout(context.Background(), blobQuoteTimeout)
	defer cancel()

	resp, err := calc.GetProjectedCost(ctx, &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/BlobStorage",
			Region:       region,
			Sku:          sku,
			Tags:         map[string]string{"size_gb": strconv.FormatFloat(sizeGB, 'f', -1, 64)},
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}

	got := resp.GetCostPerMonth()
	t.Logf("blob %s %.0f GB %s: plugin %.4f, live reference %.4f (%.6f per GB)",
		sku, sizeGB, region, got, want, perGB)
	if got <= 0 {
		t.Fatalf("cost_per_month = %v, want a positive cost", got)
	}
	if resp.GetCurrency() != "USD" {
		t.Fatalf("currency = %q, want USD", resp.GetCurrency())
	}
	if math.Abs(got-want) > want*blobPriceTolerance {
		t.Fatalf("cost_per_month = %.4f, want %.4f within %.0f%%", got, want, blobPriceTolerance*100)
	}
}

// fetchBlobReferencePrice reads the first-band Data Stored price for sku from
// General Block Blob v2, retrying throttled responses.
func fetchBlobReferencePrice(t *testing.T, region, sku string) float64 {
	t.Helper()

	filter := fmt.Sprintf(
		"armRegionName eq '%s' and serviceName eq 'Storage' and productName eq 'General Block Blob v2' "+
			"and skuName eq '%s' and priceType eq 'Consumption'",
		region, sku,
	)
	page := getBlobReferencePage(t, blobRetailPricesURL+"?$filter="+url.QueryEscape(filter))
	if page.NextPageLink != "" {
		t.Fatalf("reference query returned more than one page; tighten the filter")
	}

	meter := sku + " Data Stored"
	var matches []blobReferenceRow
	for _, row := range page.Items {
		if row.MeterName == meter && row.UnitOfMeasure == "1 GB/Month" && row.TierMinimumUnits == 0 {
			matches = append(matches, row)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("reference rows for %q in %s = %d, want exactly 1", meter, region, len(matches))
	}
	return matches[0].RetailPrice
}

func getBlobReferencePage(t *testing.T, endpoint string) blobReferencePage {
	t.Helper()

	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), blobReferenceTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			t.Fatalf("build reference request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			t.Fatalf("reference request: %v", err)
		}

		throttled := resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusServiceUnavailable
		if throttled && attempt < blobReferenceAttempts {
			wait := blobRetryAfter(resp.Header.Get("Retry-After"), attempt)
			resp.Body.Close()
			cancel()
			time.Sleep(wait)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			t.Fatalf("reference request: HTTP %d", resp.StatusCode)
		}

		var page blobReferencePage
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		cancel()
		if err != nil {
			t.Fatalf("decode reference page: %v", err)
		}
		return page
	}
}

// blobRetryAfter honours a Retry-After in seconds, capped at 30s, and
// otherwise waits two seconds per attempt.
func blobRetryAfter(header string, attempt int) time.Duration {
	const maxWait = 30 * time.Second
	if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
		return min(time.Duration(secs)*time.Second, maxWait)
	}
	return min(time.Duration(attempt)*2*time.Second, maxWait)
}
