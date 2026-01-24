# Quickstart: Running the Azure Public Pricing Plugin

**Feature**: 002-grpc-server-port
**Date**: 2026-01-22

## Prerequisites

- Go 1.25.5 or later
- Make (for build commands)

## Building the Plugin

```bash
# Build the binary
make build

# Or manually
go build -o bin/finfocus-plugin-azure-public ./cmd/finfocus-plugin-azure-public
```

## Running the Plugin

### Basic Usage (Ephemeral Port)

```bash
./bin/finfocus-plugin-azure-public
```

**Expected stdout**:

```text
PORT=XXXXX
```

**Expected stderr** (JSON logs):

```json
{"level":"info","plugin":"azure-public","version":"0.0.1-dev","time":1737590000,"message":"plugin started"}
```

### With Configured Port

```bash
FINFOCUS_PLUGIN_PORT=8080 ./bin/finfocus-plugin-azure-public
```

**Expected stdout**:

```text
PORT=8080
```

### With Debug Logging

```bash
FINFOCUS_LOG_LEVEL=debug ./bin/finfocus-plugin-azure-public
```

## Verifying Port Discovery

### Using Bash

```bash
# Start plugin and capture port
PORT=$(./bin/finfocus-plugin-azure-public 2>/dev/null | grep -oP 'PORT=\K\d+')
echo "Plugin listening on port: $PORT"
```

### Using Go Test

```go
cmd := exec.Command("./bin/finfocus-plugin-azure-public")
stdout, _ := cmd.StdoutPipe()
cmd.Start()

scanner := bufio.NewScanner(stdout)
scanner.Scan()
line := scanner.Text() // "PORT=XXXXX"
port := strings.TrimPrefix(line, "PORT=")
```

## Verifying Log Separation

```bash
# Capture stdout and stderr separately
./bin/finfocus-plugin-azure-public >stdout.txt 2>stderr.txt &
PID=$!
sleep 1
kill $PID

# stdout should contain ONLY "PORT=XXXXX"
cat stdout.txt
# PORT=54321

# stderr should contain JSON logs
cat stderr.txt
# {"level":"info","plugin":"azure-public",...}
```

## Testing Graceful Shutdown

### SIGTERM

```bash
./bin/finfocus-plugin-azure-public &
PID=$!
sleep 1
kill -SIGTERM $PID
wait $PID
echo "Exit code: $?"
# Exit code: 0
```

### SIGINT (Ctrl+C)

```bash
./bin/finfocus-plugin-azure-public
# Press Ctrl+C
# Expected: Clean shutdown, exit code 0
```

## Troubleshooting

### Port Already in Use

**Symptom**: Error message in stderr about address already in use

**Solution**: Either:

1. Use ephemeral port (don't set `FINFOCUS_PLUGIN_PORT`)
2. Choose a different port
3. Stop the process using the configured port

### No PORT= Output

**Symptom**: No output on stdout

**Cause**: Plugin failed during initialization

**Solution**: Check stderr for error messages

### Logs Appearing on stdout

**Symptom**: JSON logs mixed with PORT= line

**Cause**: Logger misconfigured to write to stdout

**Solution**: Verify logger uses `pluginsdk.NewPluginLogger` which outputs to stderr

## Integration with FinFocus Core

FinFocus Core discovers plugins by:

1. Starting the plugin binary
2. Reading stdout for `PORT=XXXXX` line
3. Connecting to gRPC server on that port

The plugin MUST:

- Output `PORT=XXXXX` as the first (and only) stdout line
- Keep all logs on stderr
- Accept gRPC connections on the announced port
- Handle `GetPluginInfo` RPC for version negotiation
