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

## Recent Changes
- 002-grpc-server-port: Added Go 1.25.5
