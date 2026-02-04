# Research: HTTP Client with Retry Logic

**Feature**: 006-http-client-retry
**Date**: 2026-02-03

## Research Tasks

### 1. go-retryablehttp Best Practices

**Decision**: Use `github.com/hashicorp/go-retryablehttp` with custom retry policy

**Rationale**:
- HashiCorp's retryablehttp is battle-tested in production (used by Terraform, Vault, Consul)
- Provides hooks for custom retry logic, backoff configuration, and logging
- Compatible with standard `http.Client` via `StandardClient()` method
- Handles connection pooling automatically

**Alternatives Considered**:
- `net/http` with manual retry loop: More code, error-prone retry logic
- `cenkalti/backoff`: Lower-level, requires more boilerplate
- `avast/retry-go`: General retry library, not HTTP-specific

**Implementation Notes**:
```go
client := retryablehttp.NewClient()
client.RetryMax = 3                        // Max 3 retries (4 total attempts)
client.RetryWaitMin = 1 * time.Second      // Initial backoff
client.RetryWaitMax = 30 * time.Second     // Max backoff
client.HTTPClient.Timeout = 60 * time.Second
client.CheckRetry = CustomRetryPolicy      // Custom policy for 429/503
client.Backoff = retryablehttp.LinearJitterBackoff  // Or custom
client.Logger = &ZerologAdapter{}          // Zerolog integration
```

### 2. Azure Retail Prices API Response Format

**Decision**: Parse JSON response with `NextPageLink` for pagination

**Rationale**:
- API returns paginated results with `BillingCurrency`, `CustomerEntityId`, `CustomerEntityType`, `Items[]`, `NextPageLink`, `Count`
- Each item contains: `armRegionName`, `armSkuName`, `currencyCode`, `effectiveStartDate`, `isPrimaryMeterRegion`, `meterId`, `meterName`, `productId`, `productName`, `retailPrice`, `serviceFamily`, `serviceId`, `serviceName`, `skuId`, `skuName`, `tierMinimumUnits`, `type`, `unitOfMeasure`, `unitPrice`
- Filter queries use OData syntax: `$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s'`

**Alternatives Considered**:
- GraphQL API: Azure doesn't provide one for pricing
- Azure SDK: Requires authentication, violates architectural constraint

**Implementation Notes**:
```go
type PriceResponse struct {
    BillingCurrency    string      `json:"BillingCurrency"`
    CustomerEntityId   string      `json:"CustomerEntityId"`
    CustomerEntityType string      `json:"CustomerEntityType"`
    Items              []PriceItem `json:"Items"`
    NextPageLink       string      `json:"NextPageLink"`
    Count              int         `json:"Count"`
}

type PriceItem struct {
    ArmRegionName      string  `json:"armRegionName"`
    ArmSkuName         string  `json:"armSkuName"`
    CurrencyCode       string  `json:"currencyCode"`
    MeterName          string  `json:"meterName"`
    ProductName        string  `json:"productName"`
    RetailPrice        float64 `json:"retailPrice"`
    ServiceName        string  `json:"serviceName"`
    SkuName            string  `json:"skuName"`
    UnitOfMeasure      string  `json:"unitOfMeasure"`
    UnitPrice          float64 `json:"unitPrice"`
    // ... additional fields as needed
}
```

### 3. Retry Policy for Azure API

**Decision**: Retry on 429, 503, and network errors only

**Rationale**:
- HTTP 429 (Too Many Requests): Azure rate limiting, always retry with Retry-After
- HTTP 503 (Service Unavailable): Temporary outage, retry with backoff
- Network errors (connection refused, timeout, DNS): Transient, retry
- HTTP 4xx (except 429): Client error, don't retry (won't succeed)
- HTTP 5xx (except 503): Server error, typically persistent, don't retry
- HTTP 500: Internal server error, usually indicates a bug or persistent issue

**Alternatives Considered**:
- Retry all 5xx: Too aggressive, wastes resources on persistent errors
- No retry on 503: Misses opportunity to recover from temporary outages

**Implementation Notes**:
```go
func CustomRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
    // Don't retry if context is cancelled
    if ctx.Err() != nil {
        return false, ctx.Err()
    }

    // Retry on network errors
    if err != nil {
        return true, nil
    }

    // Retry on rate limit (429) and service unavailable (503)
    if resp.StatusCode == http.StatusTooManyRequests ||
       resp.StatusCode == http.StatusServiceUnavailable {
        return true, nil
    }

    // Don't retry other status codes
    return false, nil
}
```

### 4. Retry-After Header Handling

**Decision**: Parse Retry-After as seconds (integer) or HTTP-date, use as minimum wait

**Rationale**:
- RFC 7231 specifies Retry-After can be seconds (e.g., "120") or HTTP-date
- Azure typically uses seconds format
- If present, use as minimum wait time (don't retry sooner)
- If invalid or missing, fall back to exponential backoff

**Alternatives Considered**:
- Ignore Retry-After: Risks being blocked by Azure
- Only honor if > current backoff: Could retry too aggressively

**Implementation Notes**:
```go
func parseRetryAfter(resp *http.Response) time.Duration {
    header := resp.Header.Get("Retry-After")
    if header == "" {
        return 0 // Use default backoff
    }

    // Try parsing as seconds
    if seconds, err := strconv.Atoi(header); err == nil {
        return time.Duration(seconds) * time.Second
    }

    // Try parsing as HTTP-date
    if t, err := http.ParseTime(header); err == nil {
        return time.Until(t)
    }

    return 0 // Invalid, use default backoff
}
```

### 5. Zerolog Integration with retryablehttp

**Decision**: Implement `retryablehttp.LeveledLogger` interface adapter

**Rationale**:
- retryablehttp uses its own logger interface (LeveledLogger)
- zerolog doesn't implement this interface directly
- Adapter pattern allows seamless integration
- Log retry events with structured fields (attempt, delay, status code)

**Alternatives Considered**:
- Disable retryablehttp logging: Loses observability
- Use retryablehttp default logger: Inconsistent with rest of application

**Implementation Notes**:
```go
type ZerologAdapter struct {
    Logger zerolog.Logger
}

func (z *ZerologAdapter) Error(msg string, keysAndValues ...interface{}) {
    z.Logger.Error().Fields(toFields(keysAndValues)).Msg(msg)
}

func (z *ZerologAdapter) Info(msg string, keysAndValues ...interface{}) {
    z.Logger.Info().Fields(toFields(keysAndValues)).Msg(msg)
}

func (z *ZerologAdapter) Debug(msg string, keysAndValues ...interface{}) {
    z.Logger.Debug().Fields(toFields(keysAndValues)).Msg(msg)
}

func (z *ZerologAdapter) Warn(msg string, keysAndValues ...interface{}) {
    z.Logger.Warn().Fields(toFields(keysAndValues)).Msg(msg)
}
```

### 6. User-Agent Header

**Decision**: Include plugin name and version in User-Agent

**Rationale**:
- Identifies the client to Azure for debugging/analytics
- Follows HTTP best practices
- Format: `finfocus-plugin-azure-public/VERSION`
- Version injected at build time via LDFLAGS

**Alternatives Considered**:
- No User-Agent: Works but less professional, harder to debug
- Generic User-Agent: Less helpful for Azure-side debugging

**Implementation Notes**:
```go
const (
    BaseURL   = "https://prices.azure.com/api/retail/prices"
    UserAgent = "finfocus-plugin-azure-public/" + version
)

func (c *Client) newRequest(ctx context.Context, filter string) (*retryablehttp.Request, error) {
    req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, buildURL(filter), nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("User-Agent", UserAgent)
    return req, nil
}
```

## Dependencies to Add

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/hashicorp/go-retryablehttp` | `v0.7.7` | HTTP client with retry |

**Note**: `go-retryablehttp` was planned in 001-go-module-init but not yet added to go.mod. This feature will add it.

## Open Questions Resolved

All technical questions resolved through research:
- ✅ Retry policy: 429, 503, network errors only
- ✅ Backoff strategy: Exponential, 1s min, 30s max
- ✅ Retry-After handling: Parse as seconds or HTTP-date
- ✅ Logging integration: Zerolog adapter for LeveledLogger
- ✅ User-Agent format: `finfocus-plugin-azure-public/VERSION`
