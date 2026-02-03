# CLAUDE.md - Project Context

## Commands
- **Build**: `make build`
- **Test**: `make test`
- **Lint**: `make lint`
- **Clean**: `make clean`
- **Setup**: `make ensure`
- **Help**: `make help`
- **Run Plugin**: `go run cmd/finfocus-plugin-azure-public/main.go`

## Development
- **Go Version**: 1.25.5
- **Dependencies**:
  - `finfocus-spec`: Plugin SDK
  - `go-retryablehttp`: HTTP Client
  - `zerolog`: Logging
  - `grpc`: RPC Framework
- **Architecture**:
  - `cmd/finfocus-plugin-azure-public`: Entry point
  - `internal/pricing`: Core logic
  - **No Auth**: Do not use Azure SDK auth libraries
  - **No DB**: Stateless operation only

## Code Style
- Use `gofmt` and `goimports`
- Errors: specific, wrapped, no silent failures
- Logging: `zerolog` (structured JSON) to stderr
- Output: `PORT=XXXX` to stdout ONLY

## Workflows
- **New Feature**: Run `.specify/scripts/bash/create-new-feature.sh`
- **Update Plan**: Run `.specify/scripts/bash/setup-plan.sh`
- **Check Status**: Check `ROADMAP.md`

## Active Technologies
- **Language**: Go 1.25.5 (002-grpc-server-port)
- **Storage**: N/A - stateless plugin (002-grpc-server-port)
- Go 1.25.5 + zerolog v1.34.0, finfocus-spec v0.5.4 (pluginsdk) (003-zerolog-logging)
- Go 1.25.5 + finfocus-spec v0.5.4 (pluginsdk), zerolog v1.34.0, google.golang.org/grpc (004-costsource-stubs)
- Go 1.25.5 (from go.mod) + golangci-lint (linting), actions/checkout@v6, actions/setup-go@v6 (005-ci-pipeline)
- N/A (CI workflow - no persistent storage) (005-ci-pipeline)

## Recent Changes
- 002-grpc-server-port: Added Go 1.25.5

## Zerolog

 The constant already exists in finfocus-spec at sdk/go/pluginsdk/logging.go:24-25:

  // TraceIDMetadataKey is the gRPC metadata header for trace ID propagation.
  const TraceIDMetadataKey = "x-finfocus-trace-id"

  Along with:
  - TracingUnaryServerInterceptor() - server-side interceptor
  - TraceIDFromContext(ctx) - context extraction
  - ContextWithTraceID(ctx, traceID) - context storage
