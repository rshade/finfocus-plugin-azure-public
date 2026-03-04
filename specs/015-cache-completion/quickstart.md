# Quickstart: Cache Layer Completion

## Configuring Cache TTL

Override the default 24-hour cache TTL via environment variable:

```bash
# Set cache TTL to 1 hour
export FINFOCUS_CACHE_TTL=1h

# Disable caching entirely (every request hits Azure API)
export FINFOCUS_CACHE_TTL=0s

# Use default (24 hours) — just don't set the variable
unset FINFOCUS_CACHE_TTL
```

Supported duration formats: `10s`, `5m`, `1h`, `24h`, `500ms`.

Invalid values log a warning and fall back to 24 hours:

<!-- markdownlint-disable MD013 -->

```text
{"level":"warn","value":"banana","error":"time: invalid duration ...",
 "message":"invalid FINFOCUS_CACHE_TTL, using default"}
```

<!-- markdownlint-enable MD013 -->

## Viewing Eviction Logs

Eviction events are logged at **debug** level. Enable debug logging:

```bash
export FINFOCUS_LOG_LEVEL=debug
```

Example eviction log entries:

<!-- markdownlint-disable MD013 -->

```json
{"level":"debug","cache_key":"eastus|standard_b1s|||usd",
 "eviction_reason":"expired","message":"cache entry evicted"}
{"level":"debug","cache_key":"westus2|standard_d2s_v3|||usd",
 "eviction_reason":"lru","message":"cache entry evicted"}
```

<!-- markdownlint-enable MD013 -->

## Verifying Cache Behavior

Run tests with race detector:

```bash
make test
```

Check cache stats in logs (emitted every 1000 requests or 5 minutes):

<!-- markdownlint-disable MD013 -->

```json
{"level":"info","cache_hits":850,"cache_misses":150,
 "cache_hit_ratio":0.85,"cache_size":150,"message":"cache stats"}
```

<!-- markdownlint-enable MD013 -->
