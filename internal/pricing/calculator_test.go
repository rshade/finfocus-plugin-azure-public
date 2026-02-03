package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCalculatorName(t *testing.T) {
	logger := zerolog.Nop()
	plugin := NewCalculator(logger)
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)
	testPlugin.TestName("azure-public")
}

func TestCalculatorLogIncludesTraceID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Str("plugin_name", "azure-public").Logger()

	calc := NewCalculator(logger)

	// Create context with trace ID
	ctx := pluginsdk.ContextWithTraceID(context.Background(), "trace-abc-123")

	// Call GetPluginInfo (which should log with trace ID)
	_, err := calc.GetPluginInfo(ctx, &finfocusv1.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo failed: %v", err)
	}

	// Parse log output
	logLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	traceFound := false
	for _, line := range logLines {
		if line == "" {
			continue
		}
		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}
		if logEntry["trace_id"] == "trace-abc-123" {
			traceFound = true
			break
		}
	}

	if !traceFound {
		t.Errorf("expected trace_id in log output, got: %s", buf.String())
	}
}

func TestCalculatorLogOmitsTraceIDWhenNotInContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Str("plugin_name", "azure-public").Logger()

	calc := NewCalculator(logger)

	// Create context WITHOUT trace ID
	ctx := context.Background()

	// Call GetPluginInfo
	_, err := calc.GetPluginInfo(ctx, &finfocusv1.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo failed: %v", err)
	}

	// Parse log output - should NOT have trace_id field
	logLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for _, line := range logLines {
		if line == "" {
			continue
		}
		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}
		if _, exists := logEntry["trace_id"]; exists {
			t.Errorf("expected no trace_id field when not in context, got: %s", line)
		}
	}
}

func TestEstimateCostLogIncludesTraceID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Str("plugin_name", "azure-public").Logger()

	calc := NewCalculator(logger)

	// Create context with trace ID
	ctx := pluginsdk.ContextWithTraceID(context.Background(), "estimate-trace-456")

	// Call EstimateCost (which logs with trace ID before returning Unimplemented)
	_, _ = calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{})

	// Parse log output
	logLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	traceFound := false
	for _, line := range logLines {
		if line == "" {
			continue
		}
		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}
		if logEntry["trace_id"] == "estimate-trace-456" {
			traceFound = true
			break
		}
	}

	if !traceFound {
		t.Errorf("expected trace_id in EstimateCost log output, got: %s", buf.String())
	}
}

func TestCalculatorConcurrentRequestsMaintainSeparateTraceIDs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var mu sync.Mutex
	logger := zerolog.New(zerolog.SyncWriter(&buf)).With().Str("plugin_name", "azure-public").Logger()

	calc := NewCalculator(logger)

	// Run concurrent requests with different trace IDs
	var wg sync.WaitGroup
	traceIDs := []string{"trace-1", "trace-2", "trace-3", "trace-4", "trace-5"}

	// Use error channel to collect errors from goroutines safely
	type testError struct {
		traceID string
		err     error
	}
	errCh := make(chan testError, len(traceIDs))

	for _, traceID := range traceIDs {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			ctx := pluginsdk.ContextWithTraceID(context.Background(), tid)
			_, err := calc.GetPluginInfo(ctx, &finfocusv1.GetPluginInfoRequest{})
			if err != nil {
				errCh <- testError{traceID: tid, err: err}
			}
		}(traceID)
	}

	wg.Wait()
	close(errCh)

	// Report any errors collected from goroutines
	for te := range errCh {
		t.Errorf("GetPluginInfo failed for trace %s: %v", te.traceID, te.err)
	}

	// Verify each trace ID appears in logs
	mu.Lock()
	logContent := buf.String()
	mu.Unlock()

	logLines := strings.Split(strings.TrimSpace(logContent), "\n")
	foundTraceIDs := make(map[string]bool)

	for _, line := range logLines {
		if line == "" {
			continue
		}
		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}
		if tid, ok := logEntry["trace_id"].(string); ok {
			foundTraceIDs[tid] = true
		}
	}

	for _, expectedTID := range traceIDs {
		if !foundTraceIDs[expectedTID] {
			t.Errorf("trace_id %q not found in log output", expectedTID)
		}
	}
}

func TestProjectedCostSupported(t *testing.T) {
	t.Skip("Skipping: GetProjectedCost not implemented yet. Azure pricing lookup requires implementation.")

	logger := zerolog.Nop()
	plugin := NewCalculator(logger)
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test supported resource
	resource := pluginsdk.CreateTestResource("aws", "aws:ec2:Instance", map[string]string{
		"instanceType": "t3.micro",
		"region":       "us-east-1",
	})

	resp := testPlugin.TestProjectedCost(resource, false)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.GetCurrency() != "USD" {
		t.Errorf("Expected currency USD, got %s", resp.GetCurrency())
	}

	if resp.GetUnitPrice() <= 0 {
		t.Errorf("Expected positive unit price, got %f", resp.GetUnitPrice())
	}
}

func TestProjectedCostUnsupported(t *testing.T) {
	logger := zerolog.Nop()
	plugin := NewCalculator(logger)
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test unsupported resource
	resource := pluginsdk.CreateTestResource("unsupported", "unsupported:resource:Type", nil)
	testPlugin.TestProjectedCost(resource, true) // Expect error
}

func TestActualCost(t *testing.T) {
	logger := zerolog.Nop()
	plugin := NewCalculator(logger)
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test actual cost (should return error since not implemented)
	testPlugin.TestActualCost("resource-id-123", 1640995200, 1641081600, true) // Expect error
}

// TODO: Add Azure-specific pricing tests when Azure pricing lookup is implemented.
// The following test was removed because it tested AWS EC2 instances (incorrect for Azure plugin)
// and referenced a non-existent calculateEC2InstanceCost method.

// TestGetPluginInfoReturnsSpecVersion verifies spec_version field is populated.
func TestGetPluginInfoReturnsSpecVersion(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	resp, err := calc.GetPluginInfo(context.Background(), &finfocusv1.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo failed: %v", err)
	}

	if resp.GetSpecVersion() == "" {
		t.Error("expected spec_version to be populated, got empty string")
	}
}

// TestGetPluginInfoReturnsProviders verifies providers field is populated.
func TestGetPluginInfoReturnsProviders(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	resp, err := calc.GetPluginInfo(context.Background(), &finfocusv1.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo failed: %v", err)
	}

	providers := resp.GetProviders()
	if len(providers) == 0 {
		t.Error("expected at least one provider, got none")
	}

	// Verify azure is in the providers list
	azureFound := false
	for _, p := range providers {
		if p == "azure" {
			azureFound = true
			break
		}
	}
	if !azureFound {
		t.Errorf("expected 'azure' in providers list, got: %v", providers)
	}
}

// TestSupportsReturnsFalse verifies Supports() returns supported=false.
func TestSupportsReturnsFalse(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	req := &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "compute/VirtualMachine",
		},
	}
	resp, err := calc.Supports(context.Background(), req)
	if err != nil {
		t.Fatalf("Supports failed: %v", err)
	}

	if resp.GetSupported() {
		t.Error("expected supported=false, got true")
	}

	if resp.GetReason() == "" {
		t.Error("expected reason to be populated, got empty string")
	}
}

// TestSupportsWithNilRequest verifies Supports() handles nil request gracefully.
func TestSupportsWithNilRequest(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	resp, err := calc.Supports(context.Background(), nil)
	if err != nil {
		t.Fatalf("Supports with nil request failed: %v", err)
	}

	if resp.GetSupported() {
		t.Error("expected supported=false for nil request, got true")
	}
}

// TestSupportsLogsTraceID verifies that Supports RPC logs include trace ID.
func TestSupportsLogsTraceID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Str("plugin_name", "azure-public").Logger()

	calc := NewCalculator(logger)

	ctx := pluginsdk.ContextWithTraceID(context.Background(), "supports-trace-abc")
	_, err := calc.Supports(ctx, &finfocusv1.SupportsRequest{})
	if err != nil {
		t.Fatalf("Supports failed: %v", err)
	}

	logLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	traceFound := false
	for _, line := range logLines {
		if line == "" {
			continue
		}
		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}
		if logEntry["trace_id"] == "supports-trace-abc" {
			traceFound = true
			break
		}
	}

	if !traceFound {
		t.Errorf("expected trace_id in Supports log output, got: %s", buf.String())
	}
}

// TestEstimateCostReturnsUnimplemented verifies EstimateCost returns Unimplemented status.
func TestEstimateCostReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.EstimateCost(context.Background(), &finfocusv1.EstimateCostRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	// Verify it's an Unimplemented gRPC status
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}

// TestGetActualCostReturnsUnimplemented verifies GetActualCost returns Unimplemented status.
func TestGetActualCostReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}

// TestGetProjectedCostReturnsUnimplemented verifies GetProjectedCost returns Unimplemented status.
func TestGetProjectedCostReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}

// TestGetPricingSpecReturnsUnimplemented verifies GetPricingSpec returns Unimplemented status.
func TestGetPricingSpecReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}

// TestGetRecommendationsReturnsUnimplemented verifies GetRecommendations returns Unimplemented status.
func TestGetRecommendationsReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.GetRecommendations(context.Background(), &finfocusv1.GetRecommendationsRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}

// TestDismissRecommendationReturnsUnimplemented verifies DismissRecommendation returns Unimplemented status.
func TestDismissRecommendationReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.DismissRecommendation(context.Background(), &finfocusv1.DismissRecommendationRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}

// TestGetBudgetsReturnsUnimplemented verifies GetBudgets returns Unimplemented status.
func TestGetBudgetsReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.GetBudgets(context.Background(), &finfocusv1.GetBudgetsRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}

// TestDryRunReturnsUnimplemented verifies DryRun returns Unimplemented status.
func TestDryRunReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	_, err := calc.DryRun(context.Background(), &finfocusv1.DryRunRequest{})
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented code, got: %v", st.Code())
	}
}
