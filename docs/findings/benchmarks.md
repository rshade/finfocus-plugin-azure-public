# Benchmark baseline

These numbers are this machine's baseline, not a gate. They are not a CI
threshold.

Both runs call `GetProjectedCost` for the on-demand Virtual Machine golden
request. The fixture is
`internal/pricing/testdata/retail/spot/standard_d2s_v3_eastus.json`, served
by `httptest`. The handler returns that saved page immediately. Nothing
calls `prices.azure.com`. The cache TTL is one hour from the test config.
The run does not read `FINFOCUS_CACHE_TTL`.

## Machine

- `go env GOOS`: `linux`
- `go env GOARCH`: `amd64`
- `uname -m`: `x86_64`

The benchmark header names the CPU as AMD Ryzen AI 7 PRO 350 w/ Radeon 860M.
`BenchmarkGetProjectedCostCacheHit` ran with `GOMAXPROCS` 8.

## Cache hit rate

One warm-up call, then 50 goroutines and 8 repeats of that same query.
`Stats().Hits` and `Stats().Misses` include the warm-up. The ratio is
`hits / (hits + misses)`. The test fails when that ratio is not greater
than `0.80`, and when `hits + misses` is `0`. The ratio is computed. It is
not a hardcoded fraction.

The first real run passed because the existing cache already hits. The
`0.80` assertion stayed. The worker count is 50.

A cold `EstimateCost` loop disables the cache (`TTL` 0) and calls the
in-process fixture every iteration. `MapDescriptorToQuery` does no I/O.

```text
go test -count=1 -bench 'BenchmarkEstimateCostCold|BenchmarkMapDescriptorToQuery|BenchmarkGetProjectedCostCacheHit' -benchtime 50ms -run '^$' ./internal/pricing/
```

```text
BenchmarkGetProjectedCostCacheHit-8      19868      3051 ns/op    3763 B/op      31 allocs/op
BenchmarkEstimateCostCold-8                230    229208 ns/op   32932 B/op     186 allocs/op
BenchmarkMapDescriptorToQuery-8         186726       316.8 ns/op     120 B/op       2 allocs/op
```

The machine is the same AMD Ryzen AI 7 PRO 350 named above. These numbers
are a local baseline, not a gate.

Command:

```text
go test -count=1 ./internal/pricing/ -run TestCacheHitRate
```

```text
ok   github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.020s
```

The same test with `-v` logged:

```text
cache hits=128 misses=1 total=129 ratio=0.992248
```

`128 / 129` is `0.992248`, which is above `0.80`. The single miss is the
warm-up. The 128 repeats hit.

## Benchmark

`BenchmarkGetProjectedCostCacheHit` times that same quote. The timer starts
after one prime call, so the line is cache-hit time.

Command:

```text
go test -count=1 -bench=BenchmarkGetProjectedCostCacheHit -benchtime=200ms -run='^$' ./internal/pricing/
```

```text
BenchmarkGetProjectedCostCacheHit-8       88803       2771 ns/op     3482 B/op       30 allocs/op
```
