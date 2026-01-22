# Quickstart: Developer Environment Setup

## Prerequisites
- Go 1.25.5 installed
- Git installed
- Internet connection

## Initialization

1. **Initialize Module** (if not exists):
   ```bash
   go mod init github.com/rshade/finfocus-plugin-azure-public
   ```

2. **Add Dependencies**:
   ```bash
   go get github.com/rshade/finfocus-spec@v0.5.4
   go get github.com/hashicorp/go-retryablehttp@v0.7.7
   go get github.com/rs/zerolog@v1.33.0
   ```

3. **Verify**:
   ```bash
   go mod tidy
   go mod verify
   go list -m all
   ```

## Validation
Run a build to ensure everything resolves:
```bash
go build ./...
```
