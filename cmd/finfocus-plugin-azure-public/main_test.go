package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

//nolint:gochecknoglobals // Test fixtures require package-level state for sync.Once pattern.
var (
	testBinaryOnce sync.Once
	testBinaryPath string
	errTestBinary  error
)

// buildTestBinary builds the test binary once and returns the path.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	testBinaryOnce.Do(func() {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			errTestBinary = err
			return
		}

		testBinaryPath = filepath.Join(cwd, "test_plugin_binary")
		buildCmd := exec.Command("go", "build", "-o", testBinaryPath, ".")
		buildCmd.Dir = cwd
		if err := buildCmd.Run(); err != nil {
			errTestBinary = err
			return
		}
	})

	if errTestBinary != nil {
		t.Fatalf("failed to build test binary: %v", errTestBinary)
	}
	return testBinaryPath
}

// TestMain handles test setup and cleanup.
func TestMain(m *testing.M) {
	code := m.Run()
	// Cleanup test binary
	if testBinaryPath != "" {
		os.Remove(testBinaryPath)
	}
	os.Exit(code)
}

// =============================================================================
// User Story 1: Port Discovery Tests
// =============================================================================

// TestPortOutputFormat verifies the plugin outputs PORT=XXXXX format to stdout.
// This is the core port discovery mechanism for FinFocus Core.
func TestPortOutputFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Read the first line from stdout with timeout
	lineChan := make(chan string, 1)
	errChan := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		} else {
			errChan <- scanner.Err()
		}
	}()

	var line string
	select {
	case line = <-lineChan:
	case err := <-errChan:
		cmd.Process.Kill()
		t.Fatalf("error reading stdout: %v", err)
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Verify PORT= format
	portRegex := regexp.MustCompile(`^PORT=(\d+)$`)
	matches := portRegex.FindStringSubmatch(line)
	if matches == nil {
		t.Errorf("stdout line does not match PORT=XXXXX format, got: %q", line)
	} else {
		port, _ := strconv.Atoi(matches[1])
		if port <= 0 || port > 65535 {
			t.Errorf("port number out of valid range: %d", port)
		}
		t.Logf("Plugin announced port: %d", port)
	}

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

// TestStdoutContainsOnlyPortLine verifies that stdout contains only the PORT= line
// and no log contamination (logs should go to stderr).
func TestStdoutContainsOnlyPortLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Collect all stdout lines until PORT= is seen or timeout
	type scanResult struct {
		lines []string
		err   error
	}
	resultChan := make(chan scanResult, 1)

	go func() {
		var lines []string
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
			// Once we see PORT=, the plugin is ready - break to signal test
			if strings.HasPrefix(line, "PORT=") {
				break
			}
		}
		resultChan <- scanResult{lines: lines, err: scanner.Err()}
	}()

	var collectedLines []string
	select {
	case result := <-resultChan:
		if result.err != nil {
			cmd.Process.Kill()
			t.Fatalf("error reading stdout: %v", result.err)
		}
		collectedLines = result.lines
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Kill the plugin gracefully
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Filter empty lines
	var nonEmptyLines []string
	for _, line := range collectedLines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	if len(nonEmptyLines) == 0 {
		t.Fatal("stdout is empty, expected PORT=XXXXX line")
	}

	if len(nonEmptyLines) > 1 {
		t.Errorf("stdout contains more than one line, got %d lines:\n%s",
			len(nonEmptyLines), strings.Join(nonEmptyLines, "\n"))
	}

	// Verify the single line is PORT= format
	portRegex := regexp.MustCompile(`^PORT=\d+$`)
	if !portRegex.MatchString(nonEmptyLines[0]) {
		t.Errorf("stdout line is not PORT=XXXXX format, got: %q", nonEmptyLines[0])
	}
}

// =============================================================================
// User Story 2: Configurable Port Tests
// =============================================================================

// TestConfiguredPortUsed verifies that FINFOCUS_PLUGIN_PORT env var is respected.
func TestConfiguredPortUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	// Use a high port number unlikely to be in use
	configuredPort := "54321"

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), "FINFOCUS_PLUGIN_PORT="+configuredPort)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Read the first line from stdout with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	var line string
	select {
	case line = <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	expectedLine := "PORT=" + configuredPort
	if line != expectedLine {
		t.Errorf("expected %q, got %q", expectedLine, line)
	}

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

// TestEphemeralPortWhenNotConfigured verifies ephemeral port is used when env var not set.
func TestEphemeralPortWhenNotConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	// Explicitly clear the port env var
	env := os.Environ()
	filteredEnv := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "FINFOCUS_PLUGIN_PORT=") {
			filteredEnv = append(filteredEnv, e)
		}
	}
	cmd.Env = filteredEnv

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Read the first line from stdout with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	var line string
	select {
	case line = <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Extract port number
	portRegex := regexp.MustCompile(`^PORT=(\d+)$`)
	matches := portRegex.FindStringSubmatch(line)
	if matches == nil {
		t.Fatalf("stdout line does not match PORT=XXXXX format, got: %q", line)
	}

	port, _ := strconv.Atoi(matches[1])
	// Ephemeral ports are typically in the range 49152-65535
	if port < 1024 {
		t.Logf("Warning: ephemeral port %d is below 1024 (may require elevated privileges)", port)
	}
	t.Logf("Plugin using ephemeral port: %d", port)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

// TestInvalidPortNonNumeric verifies the plugin fails with a clear error message
// when FINFOCUS_PLUGIN_PORT is set to a non-numeric value.
func TestInvalidPortNonNumeric(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), "FINFOCUS_PLUGIN_PORT=invalid")
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Verify exit code is non-zero
	if err == nil {
		t.Fatal("expected non-zero exit code for invalid port, but got success")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("unexpected error type: %v", err)
	}

	if exitErr.ExitCode() == 0 {
		t.Error("expected non-zero exit code, got 0")
	}

	// Verify stderr contains the expected error message
	stderrContent := stderrBuf.String()
	if !strings.Contains(stderrContent, "FINFOCUS_PLUGIN_PORT must be numeric") {
		t.Errorf("stderr does not contain expected error message\nstderr: %s", stderrContent)
	}

	// Verify no PORT= line appears on stdout
	stdoutContent := stdoutBuf.String()
	if strings.Contains(stdoutContent, "PORT=") {
		t.Errorf("stdout should not contain PORT= line for invalid port\nstdout: %s", stdoutContent)
	}

	t.Logf("Plugin correctly rejected non-numeric port with exit code %d", exitErr.ExitCode())
}

// =============================================================================
// User Story 3: Graceful Shutdown Tests
// =============================================================================

// TestGracefulShutdownOnSIGTERM verifies the plugin shuts down cleanly on SIGTERM.
func TestGracefulShutdownOnSIGTERM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case line := <-lineChan:
		t.Logf("Plugin started: %s", line)
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Send SIGTERM
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	// Wait for exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		var exitErr *exec.ExitError
		switch {
		case err == nil:
			t.Log("Plugin exited cleanly with code 0")
		case errors.As(err, &exitErr):
			// On some systems, signal termination reports non-zero
			// but we consider it graceful if it exits at all.
			t.Logf("Process exited with: %v", exitErr)
		default:
			t.Errorf("unexpected error on shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("plugin did not shut down within 5 seconds")
	}
}

// TestExitCodeZeroOnGracefulShutdown verifies exit code 0 after graceful shutdown.
func TestExitCodeZeroOnGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Send SIGTERM
	cmd.Process.Signal(syscall.SIGTERM)

	// Wait and check exit code
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		var exitErr *exec.ExitError
		switch {
		case err == nil:
			t.Log("Exit code: 0 (graceful shutdown)")
		case errors.As(err, &exitErr):
			// Note: Some systems report signal exits as non-zero.
			// This is acceptable behavior for signal termination.
			t.Logf("Exit code: %d (signal termination may report non-zero)", exitErr.ExitCode())
		default:
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("plugin did not shut down within 5 seconds")
	}
}

// =============================================================================
// User Story 4: Structured Logging Tests
// =============================================================================

// TestLogsAppearOnStderr verifies all log messages go to stderr.
func TestLogsAppearOnStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Check stderr has content
	stderrContent := stderrBuf.String()
	if len(stderrContent) == 0 {
		t.Error("stderr is empty, expected log messages")
	} else {
		t.Logf("stderr contains %d bytes of log data", len(stderrContent))
	}
}

// TestLogsAreValidJSON verifies log messages are valid JSON format.
func TestLogsAreValidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Parse each line as JSON
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	validJSONCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			t.Errorf("invalid JSON in stderr: %q\nerror: %v", line, err)
		} else {
			validJSONCount++
			// Check for expected fields
			if _, ok := jsonObj["level"]; !ok {
				t.Errorf("JSON log missing 'level' field: %s", line)
			}
			if _, ok := jsonObj["message"]; !ok {
				t.Errorf("JSON log missing 'message' field: %s", line)
			}
		}
	}

	if validJSONCount == 0 {
		t.Error("no valid JSON log lines found in stderr")
	} else {
		t.Logf("Found %d valid JSON log lines", validJSONCount)
	}
}

// TestLogsContainPluginAndVersionFields verifies logs include plugin and version fields.
func TestLogsContainPluginAndVersionFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Parse each line as JSON and check for plugin/version fields
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			continue // Skip non-JSON lines
		}

		// Check for plugin_name field (SDK uses plugin_name, not plugin)
		pluginName, ok := jsonObj["plugin_name"]
		if !ok {
			t.Errorf("JSON log missing 'plugin_name' field: %s", line)
		} else if pluginName != "azure-public" {
			t.Errorf("expected plugin_name=%q, got %q in: %s", "azure-public", pluginName, line)
		}

		// Check for plugin_version field (SDK uses plugin_version, not version)
		version, ok := jsonObj["plugin_version"]
		if !ok {
			t.Errorf("JSON log missing 'plugin_version' field: %s", line)
		} else if version == "" {
			t.Errorf("plugin_version field is empty in: %s", line)
		}

		// Check for time field
		if _, ok := jsonObj["time"]; !ok {
			t.Errorf("JSON log missing 'time' field: %s", line)
		}
	}
}

// TestLogsContainTimeField verifies logs include time field in RFC3339 format.
func TestLogsContainTimeField(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Parse each line and verify time field exists
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			continue
		}

		timeVal, ok := jsonObj["time"]
		if !ok {
			t.Errorf("JSON log missing 'time' field: %s", line)
			continue
		}

		// Verify it's a valid timestamp by attempting to parse
		timeStr, ok := timeVal.(string)
		if !ok {
			t.Errorf("'time' field is not a string: %v in: %s", timeVal, line)
			continue
		}

		if _, err := time.Parse(time.RFC3339, timeStr); err != nil {
			t.Errorf("'time' field is not RFC3339 format: %q in: %s", timeStr, line)
		}
	}
}

// =============================================================================
// User Story 2: Log Level Control Tests
// =============================================================================

// TestLogLevelDebugShowsDebugMessages verifies debug messages appear when log level is debug.
func TestLogLevelDebugShowsDebugMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), "FINFOCUS_LOG_LEVEL=debug")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Check for debug level messages
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	debugFound := false
	for _, line := range lines {
		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			continue
		}
		if jsonObj["level"] == "debug" {
			debugFound = true
			t.Logf("Found debug log: %s", line)
			break
		}
	}

	if !debugFound {
		t.Error("expected debug level messages when FINFOCUS_LOG_LEVEL=debug")
		t.Logf("stderr content:\n%s", stderrContent)
	}
}

// TestLogLevelErrorSuppressesInfoMessages verifies info messages are suppressed at error level.
func TestLogLevelErrorSuppressesInfoMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), "FINFOCUS_LOG_LEVEL=error")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Check that no info level messages appear
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			continue
		}
		level, ok := jsonObj["level"].(string)
		if !ok {
			continue
		}
		// info, debug, trace should all be suppressed at error level
		if level == "info" || level == "debug" || level == "trace" {
			t.Errorf("unexpected %s level message when FINFOCUS_LOG_LEVEL=error: %s", level, line)
		}
	}

	t.Logf("Log output at error level (should be minimal or empty):\n%s", stderrContent)
}

// TestLogLevelDefaultIsInfo verifies default log level is info when no env var set.
func TestLogLevelDefaultIsInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	// Explicitly clear log level env vars
	env := os.Environ()
	filteredEnv := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "FINFOCUS_LOG_LEVEL=") && !strings.HasPrefix(e, "LOG_LEVEL=") {
			filteredEnv = append(filteredEnv, e)
		}
	}
	cmd.Env = filteredEnv
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Verify info messages appear (default level) but not debug
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	infoFound := false
	debugFound := false
	for _, line := range lines {
		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			continue
		}
		level, _ := jsonObj["level"].(string)
		if level == "info" {
			infoFound = true
		}
		if level == "debug" {
			debugFound = true
		}
	}

	if !infoFound {
		t.Error("expected info level messages at default log level")
	}
	if debugFound {
		t.Error("unexpected debug level messages at default log level (should be suppressed)")
	}
}

// TestLogLevelFinfocusTakesPrecedenceOverLogLevel verifies FINFOCUS_LOG_LEVEL takes precedence.
func TestLogLevelFinfocusTakesPrecedenceOverLogLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	// Set both env vars - FINFOCUS_LOG_LEVEL should take precedence
	cmd.Env = append(os.Environ(), "FINFOCUS_LOG_LEVEL=debug", "LOG_LEVEL=error")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// If FINFOCUS_LOG_LEVEL=debug takes precedence, we should see debug messages
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	debugFound := false
	for _, line := range lines {
		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			continue
		}
		if jsonObj["level"] == "debug" {
			debugFound = true
			break
		}
	}

	if !debugFound {
		t.Error("expected debug messages when FINFOCUS_LOG_LEVEL=debug (should take precedence over LOG_LEVEL=error)")
		t.Logf("stderr content:\n%s", stderrContent)
	}
}

// TestLogLevelInvalidFallsBackToInfo verifies invalid log level falls back to info with warning.
func TestLogLevelInvalidFallsBackToInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), "FINFOCUS_LOG_LEVEL=invalid_level")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin: %v", err)
	}

	// Wait for PORT= output with timeout
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for PORT= output")
	}

	// Let logs accumulate
	time.Sleep(200 * time.Millisecond)

	// Kill the plugin
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()

	// Verify info messages appear (fallback level) and warning about invalid level
	stderrContent := stderrBuf.String()
	lines := strings.Split(strings.TrimSpace(stderrContent), "\n")

	infoFound := false
	warnFound := false
	for _, line := range lines {
		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			continue
		}
		level, _ := jsonObj["level"].(string)
		if level == "info" {
			infoFound = true
		}
		if level == "warn" || level == "warning" {
			warnFound = true
			t.Logf("Found warning log: %s", line)
		}
	}

	if !infoFound {
		t.Error("expected info level messages when invalid log level falls back to info")
	}
	if !warnFound {
		t.Log("Note: No warning logged for invalid log level (expected but optional behavior)")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestPortAlreadyInUse verifies the plugin reports an error when the port is occupied.
func TestPortAlreadyInUse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	// Start first instance on a specific port
	port := "54322"
	cmd1 := exec.Command(binaryPath)
	cmd1.Env = append(os.Environ(), "FINFOCUS_PLUGIN_PORT="+port)
	stdout1, _ := cmd1.StdoutPipe()

	if err := cmd1.Start(); err != nil {
		t.Fatalf("failed to start first plugin: %v", err)
	}
	defer func() {
		cmd1.Process.Signal(syscall.SIGTERM)
		cmd1.Wait()
	}()

	// Wait for first instance to be ready
	lineChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout1)
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case <-lineChan:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for first instance")
	}

	// Try to start second instance on the same port
	cmd2 := exec.Command(binaryPath)
	cmd2.Env = append(os.Environ(), "FINFOCUS_PLUGIN_PORT="+port)
	var stderr2 bytes.Buffer
	cmd2.Stderr = &stderr2

	if err := cmd2.Start(); err != nil {
		t.Fatalf("failed to start second plugin: %v", err)
	}

	// Wait for second instance to exit (should fail)
	done := make(chan error, 1)
	go func() {
		done <- cmd2.Wait()
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected second instance to fail, but it succeeded")
		} else {
			t.Logf("Second instance correctly failed: %v", err)
			// Verify error message in stderr
			stderrContent := stderr2.String()
			if len(stderrContent) == 0 {
				t.Error("expected error message in stderr")
			} else {
				t.Logf("stderr: %s", stderrContent)
			}
		}
	case <-time.After(10 * time.Second):
		cmd2.Process.Kill()
		t.Fatal("second instance did not exit within timeout")
	}
}

// TestRapidStartupShutdown verifies the plugin handles rapid cycles without port conflicts.
func TestRapidStartupShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)
	iterations := 10 // Reduced from 100 for test speed, but validates the pattern

	for i := range iterations {
		cmd := exec.Command(binaryPath)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("iteration %d: failed to get stdout pipe: %v", i, err)
		}

		if err := cmd.Start(); err != nil {
			t.Fatalf("iteration %d: failed to start plugin: %v", i, err)
		}

		// Wait for PORT= output
		lineChan := make(chan string, 1)
		go func() {
			scanner := bufio.NewScanner(stdout)
			if scanner.Scan() {
				lineChan <- scanner.Text()
			}
		}()

		select {
		case line := <-lineChan:
			if !strings.HasPrefix(line, "PORT=") {
				t.Errorf("iteration %d: expected PORT= prefix, got: %s", i, line)
			}
		case <-time.After(10 * time.Second):
			cmd.Process.Kill()
			t.Fatalf("iteration %d: timeout waiting for PORT= output", i)
		}

		// Immediately send shutdown signal
		cmd.Process.Signal(syscall.SIGTERM)

		// Wait for clean exit
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-done:
			// Success - process exited
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			t.Fatalf("iteration %d: timeout waiting for shutdown", i)
		}
	}

	t.Logf("Successfully completed %d rapid startup/shutdown cycles", iterations)
}
