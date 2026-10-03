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
	"strings"
	"testing"
	"time"
)

const (
	retailPricesURL = "https://prices.azure.com/api/retail/prices"

	// liveTolerance only absorbs rounding: the reference and the plugin read
	// the same Retail Prices rows within seconds of each other, so any larger
	// gap means the plugin selected a different meter.
	liveTolerance = 0.01

	referenceAttempts   = 4
	referenceRetryFloor = 2 * time.Second
	referenceRetryCap   = 30 * time.Second
)

type retailRow struct {
	Type          string  `json:"type"`
	ProductName   string  `json:"productName"`
	MeterName     string  `json:"meterName"`
	UnitOfMeasure string  `json:"unitOfMeasure"`
	CurrencyCode  string  `json:"currencyCode"`
	RetailPrice   float64 `json:"retailPrice"`
}

// liveRetailRate reads the Retail Prices API directly, without the plugin's
// client, and returns the one row that keep selects. The selection rules live
// in each test so a plugin meter-selection bug cannot also change the reference.
func liveRetailRate(t *testing.T, filter string, keep func(retailRow) bool) retailRow {
	t.Helper()

	page := fetchReferencePage(t, retailPricesURL+"?$filter="+url.QueryEscape(filter))
	if page.NextPageLink != "" {
		t.Fatalf("reference filter %q is too broad: more than one page", filter)
	}

	var matches []retailRow
	for _, row := range page.Items {
		if keep(row) {
			matches = append(matches, row)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("reference filter %q selected %d rows, want 1: %s", filter, len(matches), describeRows(matches))
	}
	if matches[0].RetailPrice <= 0 {
		t.Fatalf("reference row %s has no positive price", describeRows(matches))
	}
	return matches[0]
}

type referencePage struct {
	Items        []retailRow `json:"Items"`
	NextPageLink string      `json:"NextPageLink"`
}

// fetchReferencePage retries HTTP 429 and 503 a bounded number of times,
// honouring Retry-After, because the reference uses a plain HTTP client
// without the plugin's retry policy.
func fetchReferencePage(t *testing.T, requestURL string) referencePage {
	t.Helper()

	for attempt := 1; ; attempt++ {
		page, status, retryAfter := getReferencePage(t, requestURL)
		if status == http.StatusOK {
			return page
		}
		retryable := status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable
		if !retryable || attempt == referenceAttempts {
			t.Fatalf("reference request: HTTP %d after %d attempt(s) for %s", status, attempt, requestURL)
		}
		wait := max(retryAfter, referenceRetryFloor*time.Duration(attempt))
		wait = min(wait, referenceRetryCap)
		t.Logf("reference request: HTTP %d, retrying in %s", status, wait)
		time.Sleep(wait)
	}
}

func getReferencePage(t *testing.T, requestURL string) (referencePage, int, time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		t.Fatalf("build reference request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("reference request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var retryAfter time.Duration
		if secs, convErr := strconv.Atoi(resp.Header.Get("Retry-After")); convErr == nil && secs > 0 {
			retryAfter = time.Duration(secs) * time.Second
		}
		return referencePage{}, resp.StatusCode, retryAfter
	}

	var page referencePage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		t.Fatalf("decode reference response: %v", err)
	}
	return page, http.StatusOK, 0
}

// linuxOnDemandVMRate is the Linux pay-as-you-go hourly row a person would
// pick on the pricing page: Consumption, no Windows product, no Spot or Low
// Priority meter.
func linuxOnDemandVMRate(t *testing.T, region, armSKU string) retailRow {
	t.Helper()

	filter := fmt.Sprintf(
		"serviceName eq 'Virtual Machines' and armRegionName eq '%s' and armSkuName eq '%s'"+
			" and priceType eq 'Consumption'",
		region, armSKU)
	return liveRetailRate(t, filter, func(row retailRow) bool {
		return row.Type == "Consumption" &&
			row.UnitOfMeasure == "1 Hour" &&
			!strings.Contains(row.ProductName, "Windows") &&
			!strings.Contains(row.MeterName, "Spot") &&
			!strings.Contains(row.MeterName, "Low Priority")
	})
}

// managedDiskRate is the monthly disk row for one size tier, for example
// product "Premium SSD Managed Disks" and meter "P10 LRS Disk". Mount and
// operations meters are separate rows and are not the disk price.
func managedDiskRate(t *testing.T, region, product, tierSKU string) retailRow {
	t.Helper()

	filter := fmt.Sprintf(
		"serviceName eq 'Storage' and armRegionName eq '%s' and productName eq '%s' and skuName eq '%s'",
		region, product, tierSKU)
	return liveRetailRate(t, filter, func(row retailRow) bool {
		return row.Type == "Consumption" &&
			row.UnitOfMeasure == "1/Month" &&
			row.MeterName == tierSKU+" Disk"
	})
}

func assertMatchesLive(t *testing.T, actual, reference float64) {
	t.Helper()
	if math.Abs(actual-reference) > reference*liveTolerance {
		t.Errorf("value %.4f does not match live reference %.4f (±%.0f%%)", actual, reference, liveTolerance*100)
	}
}

func describeRows(rows []retailRow) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("{%s %q %q %s %g}",
			row.Type, row.ProductName, row.MeterName, row.UnitOfMeasure, row.RetailPrice))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
