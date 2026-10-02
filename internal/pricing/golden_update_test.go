//go:build integration

package pricing

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

type goldenCaptureStore struct {
	mu    sync.Mutex
	pages map[string]goldenCapture
}

func (s *goldenCaptureStore) put(rawQuery string, status int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pages == nil {
		s.pages = map[string]goldenCapture{}
	}
	s.pages[rawQuery] = goldenCapture{
		RawQuery: rawQuery,
		Status:   status,
		Body:     rawJSON(body),
	}
}

func (s *goldenCaptureStore) snapshot() []goldenCapture {
	s.mu.Lock()
	defer s.mu.Unlock()
	pages := make([]goldenCapture, 0, len(s.pages))
	for _, page := range s.pages {
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].RawQuery < pages[j].RawQuery
	})
	return pages
}

func TestUpdateGoldenFromLiveAPI(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("SKIP_INTEGRATION")
	}
	update := flag.Lookup("update-golden")
	if update == nil || update.Value.String() != "true" {
		t.Skip("pass -update-golden to refresh testdata/golden/live from the Retail Prices API")
	}

	for _, tc := range goldenLiveCases() {
		t.Run(tc.name, func(t *testing.T) {
			refreshGoldenLive(t, tc)
		})
	}
}

func refreshGoldenLive(t *testing.T, tc goldenLiveCase) {
	t.Helper()

	store := &goldenCaptureStore{}
	proxy := newGoldenRecordProxy(t, store)
	t.Cleanup(proxy.Close)
	calc := newGoldenLiveCalc(t, proxy.URL)
	t.Cleanup(func() { calc.Close() })
	quote := NewCalculator(zerolog.Nop(), calc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	projected, err := quote.GetProjectedCost(ctx, tc.projected())
	if err != nil {
		t.Fatalf("GetProjectedCost: %v", err)
	}
	estimated, err := quote.EstimateCost(ctx, tc.estimate(t))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	assertGoldenLiveCost(t, "EstimateCost", estimated.GetCostMonthly(), projected.GetCostPerMonth())

	dir := goldenLiveDir(tc.name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeGoldenCapture(t, filepath.Join(dir, "capture.json"), store.snapshot())
	writeGoldenExpected(t, filepath.Join(dir, "expected.txt"), estimated.GetCostMonthly())
}

func newGoldenLiveCalc(t *testing.T, baseURL string) *azureclient.CachedClient {
	t.Helper()

	cfg := azureclient.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.RetryMax = 2
	cfg.Timeout = time.Minute
	cfg.Logger = zerolog.Nop()
	client, err := azureclient.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	cacheCfg := azureclient.DefaultCacheConfig()
	cacheCfg.TTL = time.Hour
	cacheCfg.Logger = zerolog.Nop()
	cached, err := azureclient.NewCachedClient(client, cacheCfg)
	if err != nil {
		t.Fatalf("NewCachedClient: %v", err)
	}
	return cached
}

func newGoldenRecordProxy(t *testing.T, store *goldenCaptureStore) *httptest.Server {
	t.Helper()

	upstream := &http.Client{Timeout: time.Minute}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, status, err := forwardGolden(r.Context(), upstream, r.URL.RawQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		store.put(r.URL.RawQuery, status, body)
		served := body
		if status == http.StatusOK {
			rewritten, rewriteErr := rewriteGoldenNextLink(body, server.URL)
			if rewriteErr != nil {
				http.Error(w, rewriteErr.Error(), http.StatusBadGateway)
				return
			}
			served = rewritten
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(served)
	}))
	return server
}

func forwardGolden(ctx context.Context, upstream *http.Client, rawQuery string) ([]byte, int, error) {
	target := azureclient.DefaultBaseURL
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := upstream.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

func writeGoldenCapture(t *testing.T, path string, pages []goldenCapture) {
	t.Helper()
	if len(pages) == 0 {
		t.Fatal("live capture has no pages")
	}
	data, err := json.MarshalIndent(pages, "", "  ")
	if err != nil {
		t.Fatalf("encode capture: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write capture: %v", err)
	}
}

func writeGoldenExpected(t *testing.T, path string, cost float64) {
	t.Helper()
	text := strconv.FormatFloat(cost, 'f', -1, 64) + "\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write expected: %v", err)
	}
}

func rawJSON(body []byte) json.RawMessage {
	if json.Valid(body) {
		return json.RawMessage(body)
	}
	encoded, err := json.Marshal(string(body))
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}
