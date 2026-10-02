package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const goldenLiveTolerance = 0.01

func TestMain(m *testing.M) {
	if flag.Lookup("update-golden") == nil {
		flag.Bool(
			"update-golden",
			false,
			"rewrite testdata/golden/live from the live Retail Prices API",
		)
	}
	os.Exit(m.Run())
}

type goldenLiveCase struct {
	name      string
	projected func() *finfocusv1.GetProjectedCostRequest
	estimate  func(t *testing.T) *finfocusv1.EstimateCostRequest
}

type goldenCapture struct {
	RawQuery string          `json:"raw_query"`
	Status   int             `json:"status"`
	Body     json.RawMessage `json:"body"`
}

func goldenLiveCases() []goldenLiveCase {
	return []goldenLiveCase{
		{
			name: "vm_standard_b1s_eastus",
			projected: func() *finfocusv1.GetProjectedCostRequest {
				return vmProjectedRequest("eastus", "Standard_B1s", "")
			},
			estimate: func(t *testing.T) *finfocusv1.EstimateCostRequest {
				t.Helper()
				return newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
					"location": "eastus",
					"vmSize":   "Standard_B1s",
				})
			},
		},
		{
			name: "vm_standard_d2s_v3_eastus",
			projected: func() *finfocusv1.GetProjectedCostRequest {
				return vmProjectedRequest("eastus", "Standard_D2s_v3", "")
			},
			estimate: func(t *testing.T) *finfocusv1.EstimateCostRequest {
				t.Helper()
				return newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
					"location": "eastus",
					"vmSize":   "Standard_D2s_v3",
				})
			},
		},
		{
			name: "disk_standard_lrs_128gb",
			projected: func() *finfocusv1.GetProjectedCostRequest {
				return goldenDiskRequest("Standard_LRS", "128")
			},
			estimate: func(t *testing.T) *finfocusv1.EstimateCostRequest {
				t.Helper()
				return newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
					"location":  "eastus",
					"disk_type": "Standard_LRS",
					"size_gb":   128,
				})
			},
		},
		{
			name: "disk_premium_ssd_256gb",
			projected: func() *finfocusv1.GetProjectedCostRequest {
				return goldenDiskRequest("Premium_SSD_LRS", "256")
			},
			estimate: func(t *testing.T) *finfocusv1.EstimateCostRequest {
				t.Helper()
				return newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
					"location":  "eastus",
					"disk_type": "Premium_SSD_LRS",
					"size_gb":   256,
				})
			},
		},
	}
}

func TestGoldenLiveSnapshots(t *testing.T) {
	t.Parallel()

	for _, tc := range goldenLiveCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := goldenLiveDir(tc.name)
			want := readGoldenCost(t, filepath.Join(dir, "expected.txt"))
			server := replayGoldenServer(t, filepath.Join(dir, "capture.json"))
			t.Cleanup(server.Close)
			calc := newCalculatorTestCachedClient(t, server.URL)
			t.Cleanup(func() { calc.Close() })
			quote := NewCalculator(zerolog.Nop(), calc)

			projected, err := quote.GetProjectedCost(context.Background(), tc.projected())
			if err != nil {
				t.Fatalf("GetProjectedCost: %v", err)
			}
			estimated, err := quote.EstimateCost(context.Background(), tc.estimate(t))
			if err != nil {
				t.Fatalf("EstimateCost: %v", err)
			}
			assertGoldenLiveCost(t, "GetProjectedCost", projected.GetCostPerMonth(), want)
			assertGoldenLiveCost(t, "EstimateCost", estimated.GetCostMonthly(), want)
		})
	}
}

func TestGoldenLiveToleranceBounds(t *testing.T) {
	t.Parallel()

	if math.Abs(1.02-1) <= goldenLiveTolerance {
		t.Fatal("a 0.02 difference must fail the golden tolerance")
	}
	if math.Abs(1.005-1) > goldenLiveTolerance {
		t.Fatal("a 0.005 difference must pass the golden tolerance")
	}
}

func goldenDiskRequest(diskType, sizeGB string) *finfocusv1.GetProjectedCostRequest {
	return &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/ManagedDisk",
			Region:       "eastus",
			Sku:          diskType,
			Tags:         map[string]string{"size_gb": sizeGB},
		},
	}
}

func goldenLiveDir(name string) string {
	return filepath.Join("testdata", "golden", "live", name)
}

func assertGoldenLiveCost(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > goldenLiveTolerance {
		t.Fatalf("%s cost = %v, want %v (tolerance %v)", label, got, want, goldenLiveTolerance)
	}
}

func replayGoldenServer(t *testing.T, path string) *httptest.Server {
	t.Helper()

	pages := readGoldenCapture(t, path)
	byQuery := make(map[string]goldenCapture, len(pages))
	for _, page := range pages {
		byQuery[page.RawQuery] = page
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, ok := byQuery[r.URL.RawQuery]
		if !ok {
			http.Error(w, "no golden page for "+r.URL.RawQuery, http.StatusInternalServerError)
			return
		}
		body := []byte(page.Body)
		if page.Status == http.StatusOK {
			rewritten, err := rewriteGoldenNextLink(body, server.URL)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			body = rewritten
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(page.Status)
		_, _ = w.Write(body)
	}))
	return server
}

func rewriteGoldenNextLink(body []byte, base string) ([]byte, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return body, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	raw, ok := payload["NextPageLink"]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return body, nil
	}
	var link string
	if err := json.Unmarshal(raw, &link); err != nil {
		return nil, err
	}
	if link == "" {
		return body, nil
	}
	parsed, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	rewritten, err := json.Marshal(base + "?" + parsed.RawQuery)
	if err != nil {
		return nil, err
	}
	payload["NextPageLink"] = rewritten
	return json.Marshal(payload)
}

func readGoldenCapture(t *testing.T, path string) []goldenCapture {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var pages []goldenCapture
	if err := json.Unmarshal(data, &pages); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(pages) == 0 {
		t.Fatalf("%s has no pages", path)
	}
	return pages
}
