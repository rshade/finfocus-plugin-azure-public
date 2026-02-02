# Research: gRPC Server with Port Discovery

**Feature**: 002-grpc-server-port
**Date**: 2026-01-22
**Status**: Complete

## Research Tasks

### 1. Plugin SDK Server Lifecycle Pattern

**Decision**: Use `pluginsdk.Serve(ctx, config)` for server lifecycle management

**Rationale**:

- SDK handles PORT= output to stdout automatically
- SDK manages gRPC server creation, health checks, and graceful shutdown
- Consistent pattern across all FinFocus plugins (AWS, Azure, GCP)
- Reduces boilerplate and potential for implementation errors

**Alternatives Considered**:

- Manual gRPC server setup: Rejected because SDK provides tested, consistent implementation
- Custom port announcement: Rejected because SDK already outputs PORT= to stdout

**Reference Implementation**: `finfocus-plugin-aws-public/cmd/finfocus-plugin-aws-public/main.go`

### 2. Logger Configuration Pattern

**Decision**: Use `pluginsdk.NewPluginLogger(name, version, level, nil)` for structured logging

**Rationale**:

- Outputs JSON to stderr (required for PORT= stdout separation)
- Includes standard fields (plugin name, version, timestamp)
- Integrates with zerolog for consistent structured logging
- SDK utility handles stderr configuration automatically

**Alternatives Considered**:

- Direct zerolog setup: Rejected because SDK provides consistent plugin logging format
- Console writer: Rejected because JSON format required for log aggregation

**Configuration Pattern**:

```go
// Log level precedence: FINFOCUS_LOG_LEVEL > LOG_LEVEL > info (default)
level := zerolog.InfoLevel
if lvl := pluginsdk.GetLogLevel(); lvl != "" {
    if parsed, err := zerolog.ParseLevel(lvl); err == nil {
        level = parsed
    }
}
logger := pluginsdk.NewPluginLogger("azure-public", version, level, nil)
```

### 3. Port Configuration Pattern

**Decision**: Use `pluginsdk.GetPort()` with ephemeral fallback

**Rationale**:

- SDK checks `FINFOCUS_PLUGIN_PORT` environment variable
- Returns 0 for ephemeral port if not set
- Consistent with plugin port discovery protocol

**Configuration**:

| Env Var | Behavior |
|---------|----------|
| `FINFOCUS_PLUGIN_PORT=8080` | Listen on port 8080 |
| Not set | Listen on OS-assigned ephemeral port |
| Invalid value | SDK returns 0, use ephemeral |

**Note**: AWS plugin also supports deprecated `PORT` env var for backward compatibility. Azure plugin (new) should only support `FINFOCUS_PLUGIN_PORT`.

### 4. Signal Handling Pattern

**Decision**: Context cancellation on SIGTERM/SIGINT

**Rationale**:

- Standard Unix signal handling pattern
- `pluginsdk.Serve` respects context cancellation for graceful shutdown
- SDK handles request draining during shutdown

**Implementation Pattern**:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
go func() {
    <-sigChan
    logger.Info().Msg("received shutdown signal")
    cancel()
}()
```

### 5. ServeConfig Structure

**Decision**: Use full `ServeConfig` with `PluginInfo` for version negotiation

**Rationale**:

- `PluginInfo` enables `GetPluginInfo` RPC for Core to discover plugin capabilities
- Required for plugin version negotiation and compatibility checking

**Structure**:

```go
config := pluginsdk.ServeConfig{
    Plugin: azurePlugin,
    Port:   port,
    PluginInfo: &pluginsdk.PluginInfo{
        Name:        "finfocus-plugin-azure-public",
        Version:     version,
        SpecVersion: pluginsdk.SpecVersion,
        Providers:   []string{"azure"},
        Metadata:    map[string]string{"type": "public-pricing-fallback"},
    },
}
```

### 6. Error Handling Strategy

**Decision**: Return errors from `run()` function, exit with code 1 on error

**Rationale**:

- Ensures all defer statements execute before process exit
- Clear error logging to stderr before exit
- Non-zero exit code signals failure to orchestrators

**Pattern**:

```go
func main() {
    if err := run(); err != nil {
        os.Exit(1)
    }
}

func run() error {
    // All initialization and server logic
    // Return err on failure (already logged)
}
```

## Key Findings Summary

| Topic | Decision | Source |
|-------|----------|--------|
| Server lifecycle | `pluginsdk.Serve(ctx, config)` | AWS plugin reference |
| Logging | `pluginsdk.NewPluginLogger` | SDK documentation |
| Port config | `pluginsdk.GetPort()` | SDK env handling |
| Signal handling | Context cancellation | Go best practices |
| Error handling | `run()` pattern with exit codes | AWS plugin reference |

## No Unresolved Items

All technical decisions are resolved based on:

1. AWS plugin reference implementation
2. Plugin SDK documentation
3. Constitution requirements
4. Go best practices
