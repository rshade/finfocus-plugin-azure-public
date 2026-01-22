# CLAUDE.md - Project Context

## Commands
- **Build**: `go build ./...`
- **Test**: `go test ./...`
- **Lint**: `golangci-lint run`
- **Verify Deps**: `go mod verify`
- **Update Deps**: `go get -u ./... && go mod tidy`
- **Run Plugin**: `go run cmd/plugin/main.go`

## Development
- **Go Version**: 1.25.5
- **Dependencies**:
  - `finfocus-spec`: Plugin SDK
  - `go-retryablehttp`: HTTP Client
  - `zerolog`: Logging
  - `grpc`: RPC Framework
- **Architecture**:
  - `cmd/plugin`: Entry point
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