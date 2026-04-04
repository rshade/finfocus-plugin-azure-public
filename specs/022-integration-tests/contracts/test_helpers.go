// Package contracts defines the test helper signatures for integration tests.
//
// These are contract definitions only — not compiled into the binary.
// Exact types sourced from codebase research (2026-03-30).
package contracts

// AssertInRange verifies that actual falls within ±tolerance of reference.
// tolerance is a fraction (e.g., 0.25 for ±25%).
//
// Signature:
//
//	func assertInRange(t *testing.T, actual, reference, tolerance float64)
//
// Example:
//
//	assertInRange(t, resp.GetCostMonthly(), 7.59, 0.25) // ±25% of $7.59
//
// Calculates: low = reference * (1 - tolerance), high = reference * (1 + tolerance)
// Fails test if actual < low || actual > high.

// NewTestCalculator creates a Calculator backed by a live Azure API client
// with caching enabled. Used by all integration tests.
//
// Construction chain:
//
//	config := azureclient.DefaultConfig()  // types.go:62-73
//	config.Logger = logger
//	client, err := azureclient.NewClient(config)  // client.go:47
//	cacheConfig := azureclient.DefaultCacheConfig()  // cache.go:38-45
//	cacheConfig.Logger = logger
//	cachedClient, err := azureclient.NewCachedClient(client, cacheConfig)  // cache.go:67
//	calc := pricing.NewCalculator(logger, cachedClient)  // calculator.go:33
//
// Signature:
//
//	func newTestCalculator(t *testing.T) (*pricing.Calculator, *azureclient.CachedClient)
//
// Returns both so tests can inspect CachedClient.Stats() -> CacheStats{Hits, Misses: atomic.Int64}

// SkipIfDisabled checks SKIP_INTEGRATION env var and skips the test if set.
//
// Signature:
//
//	func skipIfDisabled(t *testing.T)
//
// Implementation: if os.Getenv("SKIP_INTEGRATION") == "true" { t.Skip("...") }

// RateLimitDelay sleeps for the configured inter-query delay (12 seconds).
// Only call after tests that make live API calls (not after cache-hit tests).
//
// Signature:
//
//	func rateLimitDelay()
//
// Implementation: time.Sleep(12 * time.Second)
