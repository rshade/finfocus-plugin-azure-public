package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
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

// TestSupports_ValidVM_ReturnsTrue verifies Supports() returns true for a valid VM descriptor.
func TestSupports_ValidVM_ReturnsTrue(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	req := &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "compute/VirtualMachine",
			Sku:          "Standard_B1s",
			Region:       "eastus",
		},
	}
	resp, err := calc.Supports(context.Background(), req)
	if err != nil {
		t.Fatalf("Supports failed: %v", err)
	}

	if !resp.GetSupported() {
		t.Errorf("expected supported=true, got false (reason: %s)", resp.GetReason())
	}
}

// TestSupports_ManagedDisk_ReturnsTrue verifies Supports() returns true for a ManagedDisk descriptor.
func TestSupports_ManagedDisk_ReturnsTrue(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	req := &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/ManagedDisk",
			Sku:          "Premium_LRS",
			Region:       "westus2",
		},
	}
	resp, err := calc.Supports(context.Background(), req)
	if err != nil {
		t.Fatalf("Supports failed: %v", err)
	}

	if !resp.GetSupported() {
		t.Errorf("expected supported=true, got false (reason: %s)", resp.GetReason())
	}
}

// TestSupports_IncompleteDescriptor_ReturnsFalse verifies Supports() returns false
// with a reason when the descriptor is missing required fields.
func TestSupports_IncompleteDescriptor_ReturnsFalse(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	req := &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "compute/VirtualMachine",
			// Missing SKU and Region
		},
	}
	resp, err := calc.Supports(context.Background(), req)
	if err != nil {
		t.Fatalf("Supports failed: %v", err)
	}

	if resp.GetSupported() {
		t.Error("expected supported=false for incomplete descriptor, got true")
	}

	if resp.GetReason() == "" {
		t.Error("expected reason to be populated, got empty string")
	}
}

// TestSupports_UnsupportedType_ReturnsFalse verifies Supports() returns false
// with a reason that includes the unsupported type name.
func TestSupports_UnsupportedType_ReturnsFalse(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	calc := NewCalculator(logger)

	req := &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "network/LoadBalancer",
			Sku:          "Standard",
			Region:       "eastus",
		},
	}
	resp, err := calc.Supports(context.Background(), req)
	if err != nil {
		t.Fatalf("Supports failed: %v", err)
	}

	if resp.GetSupported() {
		t.Error("expected supported=false for unsupported type, got true")
	}

	if !strings.Contains(resp.GetReason(), "network/LoadBalancer") {
		t.Errorf("expected reason to contain type name, got: %s", resp.GetReason())
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

	if resp.GetReason() == "" {
		t.Error("expected reason to be populated for nil request, got empty string")
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

	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})
	_, err := calc.EstimateCost(context.Background(), req)
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

func TestEstimateQueryFromRequest_ValidInput_ReturnsQuery(t *testing.T) {
	t.Parallel()

	req := newEstimateCostRequest(t, "", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	query, err := estimateQueryFromRequest(req)
	if err != nil {
		t.Fatalf("estimateQueryFromRequest() returned error: %v", err)
	}

	if query.ArmRegionName != "eastus" {
		t.Fatalf("expected region eastus, got %q", query.ArmRegionName)
	}
	if query.ArmSkuName != "Standard_B1s" {
		t.Fatalf("expected sku Standard_B1s, got %q", query.ArmSkuName)
	}
	if query.ServiceName != defaultServiceName {
		t.Fatalf("expected default service %q, got %q", defaultServiceName, query.ServiceName)
	}
	if query.CurrencyCode != "USD" {
		t.Fatalf("expected default currency USD, got %q", query.CurrencyCode)
	}
}

func TestEstimateQueryFromRequest_MissingRegion_ReturnsError(t *testing.T) {
	t.Parallel()

	req := newEstimateCostRequest(t, "", map[string]any{
		"vmSize": "Standard_B1s",
	})

	_, err := estimateQueryFromRequest(req)
	if err == nil {
		t.Fatal("expected missing region error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "region") {
		t.Fatalf("expected error to mention region, got %q", err.Error())
	}
}

func TestEstimateQueryFromRequest_MissingBoth_ReturnsError(t *testing.T) {
	t.Parallel()

	req := &finfocusv1.EstimateCostRequest{}
	_, err := estimateQueryFromRequest(req)
	if err == nil {
		t.Fatal("expected missing fields error, got nil")
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "region") || !strings.Contains(msg, "sku") {
		t.Fatalf("expected error to mention both region and sku, got %q", err.Error())
	}
}

func TestEstimateQueryFromRequest_AliasHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attrs      map[string]any
		wantRegion string
		wantSKU    string
		wantCurr   string
		wantErr    string // empty means no error expected
	}{
		{
			name:       "location and vmSize",
			attrs:      map[string]any{"location": "eastus", "vmSize": "Standard_B1s"},
			wantRegion: "eastus",
			wantSKU:    "Standard_B1s",
			wantCurr:   "USD",
		},
		{
			name:       "region alias",
			attrs:      map[string]any{"region": "westus2", "vmSize": "Standard_D2s_v3"},
			wantRegion: "westus2",
			wantSKU:    "Standard_D2s_v3",
			wantCurr:   "USD",
		},
		{
			name:       "sku alias",
			attrs:      map[string]any{"location": "eastus", "sku": "Standard_B2s"},
			wantRegion: "eastus",
			wantSKU:    "Standard_B2s",
			wantCurr:   "USD",
		},
		{
			name:       "armSkuName alias",
			attrs:      map[string]any{"location": "eastus", "armSkuName": "Standard_B4ms"},
			wantRegion: "eastus",
			wantSKU:    "Standard_B4ms",
			wantCurr:   "USD",
		},
		{
			name:       "currency alias",
			attrs:      map[string]any{"location": "eastus", "vmSize": "Standard_B1s", "currency": "EUR"},
			wantRegion: "eastus",
			wantSKU:    "Standard_B1s",
			wantCurr:   "EUR",
		},
		{
			name:       "currencyCode key",
			attrs:      map[string]any{"location": "eastus", "vmSize": "Standard_B1s", "currencyCode": "GBP"},
			wantRegion: "eastus",
			wantSKU:    "Standard_B1s",
			wantCurr:   "GBP",
		},
		{
			name:    "missing region only",
			attrs:   map[string]any{"vmSize": "Standard_B1s"},
			wantErr: "missing required field(s): region",
		},
		{
			name:    "missing sku only",
			attrs:   map[string]any{"location": "eastus"},
			wantErr: "missing required field(s): sku",
		},
		{
			name:    "missing both",
			attrs:   map[string]any{},
			wantErr: "missing required field(s): region, sku",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := newEstimateCostRequest(t, "", tc.attrs)
			query, err := estimateQueryFromRequest(req)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("expected error %q, got %q", tc.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if query.ArmRegionName != tc.wantRegion {
				t.Errorf("region: got %q, want %q", query.ArmRegionName, tc.wantRegion)
			}
			if query.ArmSkuName != tc.wantSKU {
				t.Errorf("sku: got %q, want %q", query.ArmSkuName, tc.wantSKU)
			}
			if query.CurrencyCode != tc.wantCurr {
				t.Errorf("currency: got %q, want %q", query.CurrencyCode, tc.wantCurr)
			}
		})
	}
}

func TestEstimateCost_ValidRequest_SetsPricingCategoryStandard(t *testing.T) {
	t.Parallel()

	server := newPriceServer(t, []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   0.0200,
		},
	}, nil)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	resp, err := calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("EstimateCost() failed: %v", err)
	}

	if resp.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
		t.Fatalf("expected pricing category STANDARD, got %v", resp.GetPricingCategory())
	}
}

func TestEstimateCost_ValidRequest_ReturnsCostMonthlyEqualToHourlyTimes730(t *testing.T) {
	t.Parallel()

	server := newPriceServer(t, []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   0.0200,
		},
	}, nil)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	resp, err := calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("EstimateCost() failed: %v", err)
	}

	want := 0.0200 * 730.0
	if math.Abs(resp.GetCostMonthly()-want) > 0.000001 {
		t.Fatalf("expected monthly cost %.6f, got %.6f", want, resp.GetCostMonthly())
	}
}

func TestEstimateCost_UnsupportedResourceType_ReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	req := newEstimateCostRequest(t, "network/LoadBalancer", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	_, err := calc.EstimateCost(context.Background(), req)
	assertStatusCodeContains(t, err, codes.Unimplemented, "unsupported resource type")
}

func TestEstimateCost_VirtualMachineScaleSet_ReturnsUnimplemented(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	req := newEstimateCostRequest(t, "azure:compute/virtualMachineScaleSet:VirtualMachineScaleSet", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	_, err := calc.EstimateCost(context.Background(), req)
	assertStatusCodeContains(t, err, codes.Unimplemented, "unsupported resource type")
}

func TestEstimateCost_EmptyResourceType_Succeeds(t *testing.T) {
	t.Parallel()

	server := newPriceServer(t, []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   0.0200,
		},
	}, nil)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)
	req := newEstimateCostRequest(t, "", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	resp, err := calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("EstimateCost() failed: %v", err)
	}
	if resp.GetCostMonthly() <= 0 {
		t.Fatalf("expected positive monthly cost, got %f", resp.GetCostMonthly())
	}
}

func TestEstimateCost_MissingRegion_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"vmSize": "Standard_B1s",
	})

	_, err := calc.EstimateCost(context.Background(), req)
	assertStatusCodeContains(t, err, codes.InvalidArgument, "region")
}

func TestEstimateCost_MissingSKU_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
	})

	_, err := calc.EstimateCost(context.Background(), req)
	assertStatusCodeContains(t, err, codes.InvalidArgument, "sku")
}

func TestEstimateCost_MissingBothFields_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	_, err := calc.EstimateCost(context.Background(), &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
	})
	assertStatusCodeContains(t, err, codes.InvalidArgument, "region", "sku")
}

func TestEstimateCost_NotFoundSKU_ReturnsNotFound(t *testing.T) {
	t.Parallel()

	server := newPriceServer(t, []azureclient.PriceItem{}, nil)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Definitely_Not_A_Real_SKU",
	})

	_, err := calc.EstimateCost(context.Background(), req)
	assertStatusCodeContains(t, err, codes.NotFound)
}

func TestEstimateCost_MultipleItems_UsesFirstItem(t *testing.T) {
	t.Parallel()

	server := newPriceServer(t, []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   0.0200,
		},
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   0.9999,
		},
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   1.2345,
		},
	}, nil)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	resp, err := calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("EstimateCost() failed: %v", err)
	}

	want := 0.0200 * 730.0
	if math.Abs(resp.GetCostMonthly()-want) > 0.000001 {
		t.Fatalf("expected first-item monthly cost %.6f, got %.6f", want, resp.GetCostMonthly())
	}
}

func TestEstimateCost_RepeatedQuery_UsesCacheOnSecondCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := newPriceServer(t, []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   0.0200,
		},
	}, &calls)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	first, err := calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("first EstimateCost() call failed: %v", err)
	}
	second, err := calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("second EstimateCost() call failed: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one upstream request, got %d", got)
	}
	if math.Abs(first.GetCostMonthly()-second.GetCostMonthly()) > 0.000001 {
		t.Fatalf(
			"expected identical monthly cost for cache hit, first=%.6f second=%.6f",
			first.GetCostMonthly(),
			second.GetCostMonthly(),
		)
	}
}

func TestEstimateCost_CacheStats_RecordsHitAndMiss(t *testing.T) {
	t.Parallel()

	server := newPriceServer(t, []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ServiceName:   "Virtual Machines",
			CurrencyCode:  "USD",
			RetailPrice:   0.0200,
		},
	}, nil)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)
	req := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})

	_, err := calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("first EstimateCost() call failed: %v", err)
	}
	_, err = calc.EstimateCost(context.Background(), req)
	if err != nil {
		t.Fatalf("second EstimateCost() call failed: %v", err)
	}

	stats := cachedClient.Stats()
	if stats.Hits.Load() != 1 {
		t.Fatalf("expected 1 cache hit, got %d", stats.Hits.Load())
	}
	if stats.Misses.Load() != 1 {
		t.Fatalf("expected 1 cache miss, got %d", stats.Misses.Load())
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

func TestGetProjectedCostSetsExpiresAtFromCache(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		resp := azureclient.PriceResponse{
			Items: []azureclient.PriceItem{
				{
					ArmRegionName: "eastus",
					ArmSkuName:    "Standard_B1s",
					ServiceName:   "Virtual Machines",
					CurrencyCode:  "USD",
					RetailPrice:   0.0104,
				},
			},
			Count: 1,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
			http.Error(w, "encode response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)

	req := &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure:compute/virtualMachine:VirtualMachine",
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
	}

	first, err := calc.GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetProjectedCost() first call failed: %v", err)
	}
	if first.GetExpiresAt() == nil {
		t.Fatal("expected first response to include expires_at")
	}

	second, err := calc.GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetProjectedCost() second call failed: %v", err)
	}
	if second.GetExpiresAt() == nil {
		t.Fatal("expected second response to include expires_at")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected cache hit on second call (1 upstream request), got %d calls", got)
	}

	if !first.GetExpiresAt().AsTime().Equal(second.GetExpiresAt().AsTime()) {
		t.Fatalf(
			"expected cache hit to preserve original expires_at, first=%s second=%s",
			first.GetExpiresAt().AsTime(),
			second.GetExpiresAt().AsTime(),
		)
	}
}

func TestGetActualCostSetsExpiresAtFromCache(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := azureclient.PriceResponse{
			Items: []azureclient.PriceItem{
				{
					ArmRegionName: "eastus",
					ArmSkuName:    "Standard_B1s",
					ServiceName:   "Virtual Machines",
					CurrencyCode:  "USD",
					RetailPrice:   0.0104,
				},
			},
			Count: 1,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
			http.Error(w, "encode response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)

	req := &finfocusv1.GetActualCostRequest{
		ResourceId: "vm-1",
		Tags: map[string]string{
			"region":   "eastus",
			"sku":      "Standard_B1s",
			"service":  "Virtual Machines",
			"currency": "USD",
		},
	}

	resp, err := calc.GetActualCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	if len(resp.GetResults()) != 1 {
		t.Fatalf("expected 1 actual cost result, got %d", len(resp.GetResults()))
	}
	if resp.GetResults()[0].GetExpiresAt() == nil {
		t.Fatal("expected actual cost result to include expires_at")
	}
}

func TestEstimateCostUsesCachedClient(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := azureclient.PriceResponse{
			Items: []azureclient.PriceItem{
				{
					ArmRegionName: "eastus",
					ArmSkuName:    "Standard_B1s",
					ServiceName:   "Virtual Machines",
					CurrencyCode:  "USD",
					RetailPrice:   0.0200,
				},
			},
			Count: 1,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
			http.Error(w, "encode response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)

	attrs, err := structpb.NewStruct(map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})
	if err != nil {
		t.Fatalf("NewStruct() failed: %v", err)
	}

	resp, err := calc.EstimateCost(context.Background(), &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Attributes:   attrs,
	})
	if err != nil {
		t.Fatalf("EstimateCost() failed: %v", err)
	}

	if resp.GetCurrency() != "USD" {
		t.Fatalf("expected USD currency, got %q", resp.GetCurrency())
	}
	if resp.GetCostMonthly() <= 0 {
		t.Fatalf("expected positive monthly cost, got %f", resp.GetCostMonthly())
	}
}

// Phase 3: US1+US2 Tests (T010-T012)

func TestEstimateCost_Disk_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		diskType      string
		sizeGB        float64
		items         []azureclient.PriceItem
		wantCost      float64
		wantCurrency  string
	}{
		{
			name:     "Premium_SSD_LRS_P10",
			diskType: "Premium_SSD_LRS",
			sizeGB:   128,
			items: []azureclient.PriceItem{
				{MeterName: "P4", RetailPrice: 5.28, CurrencyCode: "USD"},
				{MeterName: "P10", RetailPrice: 19.71, CurrencyCode: "USD"},
				{MeterName: "P20", RetailPrice: 38.02, CurrencyCode: "USD"},
			},
			wantCost:     19.71,
			wantCurrency: "USD",
		},
		{
			name:     "Standard_LRS_S30",
			diskType: "Standard_LRS",
			sizeGB:   1024,
			items: []azureclient.PriceItem{
				{MeterName: "S20", RetailPrice: 20.48, CurrencyCode: "USD"},
				{MeterName: "S30", RetailPrice: 40.96, CurrencyCode: "USD"},
			},
			wantCost:     40.96,
			wantCurrency: "USD",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newPriceServer(t, tc.items, nil)
			defer server.Close()

			cachedClient := newCalculatorTestCachedClient(t, server.URL)
			defer cachedClient.Close()

			calc := NewCalculator(zerolog.Nop(), cachedClient)
			req := newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
				"location":  "eastus",
				"disk_type": tc.diskType,
				"size_gb":   tc.sizeGB,
			})

			resp, err := calc.EstimateCost(context.Background(), req)
			if err != nil {
				t.Fatalf("EstimateCost() failed: %v", err)
			}

			if math.Abs(resp.GetCostMonthly()-tc.wantCost) > 0.001 {
				t.Errorf("cost_monthly = %.2f, want %.2f", resp.GetCostMonthly(), tc.wantCost)
			}
			if resp.GetCurrency() != tc.wantCurrency {
				t.Errorf("currency = %q, want %q", resp.GetCurrency(), tc.wantCurrency)
			}
			if resp.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
				t.Errorf("pricing_category = %v, want STANDARD", resp.GetPricingCategory())
			}
		})
	}
}

func TestEstimateCost_Disk_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attrs   map[string]any
		wantMsg string
	}{
		{
			name:    "missing_region",
			attrs:   map[string]any{"disk_type": "Premium_SSD_LRS", "size_gb": 128},
			wantMsg: "region",
		},
		{
			name:    "missing_disk_type",
			attrs:   map[string]any{"location": "eastus", "size_gb": 128},
			wantMsg: "disk_type",
		},
		{
			name:    "missing_size_gb",
			attrs:   map[string]any{"location": "eastus", "disk_type": "Premium_SSD_LRS"},
			wantMsg: "size_gb",
		},
		{
			name:    "all_missing",
			attrs:   map[string]any{},
			wantMsg: "region, disk_type, size_gb",
		},
		{
			name:    "size_gb_zero",
			attrs:   map[string]any{"location": "eastus", "disk_type": "Premium_SSD_LRS", "size_gb": 0},
			wantMsg: "size_gb must be greater than 0",
		},
		{
			name:    "size_gb_negative",
			attrs:   map[string]any{"location": "eastus", "disk_type": "Premium_SSD_LRS", "size_gb": -1},
			wantMsg: "size_gb must be greater than 0",
		},
		{
			name:    "unsupported_disk_type",
			attrs:   map[string]any{"location": "eastus", "disk_type": "UltraSSD_LRS", "size_gb": 128},
			wantMsg: "unsupported disk type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			calc := NewCalculator(zerolog.Nop())
			req := newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", tc.attrs)

			_, err := calc.EstimateCost(context.Background(), req)
			assertStatusCodeContains(t, err, codes.InvalidArgument, tc.wantMsg)
		})
	}
}

func TestEstimateCost_Disk_ResourceTypeRouting(t *testing.T) {
	t.Parallel()

	vmItems := []azureclient.PriceItem{
		{MeterName: "B1s", RetailPrice: 0.0200, CurrencyCode: "USD",
			ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", ServiceName: "Virtual Machines"},
	}
	diskItems := []azureclient.PriceItem{
		{MeterName: "P10", RetailPrice: 19.71, CurrencyCode: "USD",
			ArmRegionName: "eastus", ArmSkuName: "Premium_LRS", ServiceName: "Managed Disks"},
	}

	// Serve both VM and disk items (server doesn't filter, test validates routing via cost)
	allItems := append(vmItems, diskItems...)
	server := newPriceServer(t, allItems, nil)
	defer server.Close()

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	defer cachedClient.Close()

	calc := NewCalculator(zerolog.Nop(), cachedClient)

	// Disk resource type routes to disk path
	diskReq := newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
		"location":  "eastus",
		"disk_type": "Premium_SSD_LRS",
		"size_gb":   128,
	})
	diskResp, err := calc.EstimateCost(context.Background(), diskReq)
	if err != nil {
		t.Fatalf("disk EstimateCost() failed: %v", err)
	}
	// Disk cost should be 19.71 (monthly, not multiplied by 730)
	if math.Abs(diskResp.GetCostMonthly()-19.71) > 0.01 {
		t.Errorf("disk cost = %.2f, want 19.71 (monthly, not hourly×730)", diskResp.GetCostMonthly())
	}

	// VM resource type still routes to VM path
	vmReq := newEstimateCostRequest(t, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})
	vmResp, err := calc.EstimateCost(context.Background(), vmReq)
	if err != nil {
		t.Fatalf("VM EstimateCost() failed: %v", err)
	}
	// VM cost should be hourly × 730
	vmExpected := 0.0200 * 730.0
	if math.Abs(vmResp.GetCostMonthly()-vmExpected) > 0.01 {
		t.Errorf("VM cost = %.2f, want %.2f (hourly×730)", vmResp.GetCostMonthly(), vmExpected)
	}

	// Empty resource type falls through to VM path (backward compat)
	emptyReq := newEstimateCostRequest(t, "", map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})
	emptyResp, err := calc.EstimateCost(context.Background(), emptyReq)
	if err != nil {
		t.Fatalf("empty resource type EstimateCost() failed: %v", err)
	}
	if math.Abs(emptyResp.GetCostMonthly()-vmExpected) > 0.01 {
		t.Errorf("empty type cost = %.2f, want %.2f", emptyResp.GetCostMonthly(), vmExpected)
	}
}

// Phase 4: US3 Tests (T017-T018)

func TestEstimateCost_Disk_AllTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		diskType string
		items    []azureclient.PriceItem
		wantCost float64
	}{
		{
			name:     "Standard_LRS",
			diskType: "Standard_LRS",
			items:    []azureclient.PriceItem{{MeterName: "S10", RetailPrice: 5.89, CurrencyCode: "USD"}},
			wantCost: 5.89,
		},
		{
			name:     "StandardSSD_LRS",
			diskType: "StandardSSD_LRS",
			items:    []azureclient.PriceItem{{MeterName: "E10", RetailPrice: 9.60, CurrencyCode: "USD"}},
			wantCost: 9.60,
		},
		{
			name:     "Premium_SSD_LRS",
			diskType: "Premium_SSD_LRS",
			items:    []azureclient.PriceItem{{MeterName: "P10", RetailPrice: 19.71, CurrencyCode: "USD"}},
			wantCost: 19.71,
		},
		{
			name:     "Standard_ZRS",
			diskType: "Standard_ZRS",
			items:    []azureclient.PriceItem{{MeterName: "S10 ZRS", RetailPrice: 7.37, CurrencyCode: "USD"}},
			wantCost: 7.37,
		},
		{
			name:     "StandardSSD_ZRS",
			diskType: "StandardSSD_ZRS",
			items:    []azureclient.PriceItem{{MeterName: "E10 ZRS", RetailPrice: 12.00, CurrencyCode: "USD"}},
			wantCost: 12.00,
		},
		{
			name:     "Premium_ZRS",
			diskType: "Premium_ZRS",
			items:    []azureclient.PriceItem{{MeterName: "P10 ZRS", RetailPrice: 24.64, CurrencyCode: "USD"}},
			wantCost: 24.64,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newPriceServer(t, tc.items, nil)
			defer server.Close()

			cachedClient := newCalculatorTestCachedClient(t, server.URL)
			defer cachedClient.Close()

			calc := NewCalculator(zerolog.Nop(), cachedClient)
			req := newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
				"location":  "eastus",
				"disk_type": tc.diskType,
				"size_gb":   128,
			})

			resp, err := calc.EstimateCost(context.Background(), req)
			if err != nil {
				t.Fatalf("EstimateCost() failed: %v", err)
			}

			if math.Abs(resp.GetCostMonthly()-tc.wantCost) > 0.001 {
				t.Errorf("cost = %.2f, want %.2f", resp.GetCostMonthly(), tc.wantCost)
			}
		})
	}
}

func TestEstimateCost_Disk_UnsupportedTypes(t *testing.T) {
	t.Parallel()

	unsupported := []string{"UltraSSD_LRS", "PremiumV2_LRS", "MadeUp_LRS"}
	for _, diskType := range unsupported {
		t.Run(diskType, func(t *testing.T) {
			t.Parallel()

			calc := NewCalculator(zerolog.Nop())
			req := newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
				"location":  "eastus",
				"disk_type": diskType,
				"size_gb":   128,
			})

			_, err := calc.EstimateCost(context.Background(), req)
			assertStatusCodeContains(t, err, codes.InvalidArgument, "unsupported disk type")
		})
	}
}

// Phase 5: US4 Tests (T022-T023)

func TestEstimateCost_Disk_SizeScaling(t *testing.T) {
	t.Parallel()

	items := []azureclient.PriceItem{
		{MeterName: "P4", RetailPrice: 5.28, CurrencyCode: "USD"},
		{MeterName: "P10", RetailPrice: 19.71, CurrencyCode: "USD"},
		{MeterName: "P15", RetailPrice: 28.57, CurrencyCode: "USD"},
		{MeterName: "P20", RetailPrice: 38.02, CurrencyCode: "USD"},
	}

	tests := []struct {
		name     string
		sizeGB   float64
		wantCost float64
		wantTier string
	}{
		{name: "exact_32_P4", sizeGB: 32, wantCost: 5.28},
		{name: "ceiling_100_P10", sizeGB: 100, wantCost: 19.71},
		{name: "exact_128_P10", sizeGB: 128, wantCost: 19.71},
		{name: "ceiling_200_P15", sizeGB: 200, wantCost: 28.57},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newPriceServer(t, items, nil)
			defer server.Close()

			cachedClient := newCalculatorTestCachedClient(t, server.URL)
			defer cachedClient.Close()

			calc := NewCalculator(zerolog.Nop(), cachedClient)
			req := newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
				"location":  "eastus",
				"disk_type": "Premium_SSD_LRS",
				"size_gb":   tc.sizeGB,
			})

			resp, err := calc.EstimateCost(context.Background(), req)
			if err != nil {
				t.Fatalf("EstimateCost() failed: %v", err)
			}

			if math.Abs(resp.GetCostMonthly()-tc.wantCost) > 0.001 {
				t.Errorf("cost = %.2f, want %.2f", resp.GetCostMonthly(), tc.wantCost)
			}
		})
	}
}

func TestEstimateCost_Disk_SizeEdgeCases(t *testing.T) {
	t.Parallel()

	items := []azureclient.PriceItem{
		{MeterName: "P1", RetailPrice: 0.60, CurrencyCode: "USD"},
		{MeterName: "P15", RetailPrice: 28.57, CurrencyCode: "USD"},
		{MeterName: "P80", RetailPrice: 3276.80, CurrencyCode: "USD"},
	}

	tests := []struct {
		name     string
		sizeGB   float64
		wantCost float64
		wantErr  bool
		errCode  codes.Code
	}{
		{name: "fractional_0.5_rounds_to_P1", sizeGB: 0.5, wantCost: 0.60},
		{name: "largest_tier_32767", sizeGB: 32767, wantCost: 3276.80},
		{name: "exceeds_max_tier", sizeGB: 99999, wantErr: true, errCode: codes.NotFound},
		{name: "fractional_128.5_rounds_to_P15", sizeGB: 128.5, wantCost: 28.57},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newPriceServer(t, items, nil)
			defer server.Close()

			cachedClient := newCalculatorTestCachedClient(t, server.URL)
			defer cachedClient.Close()

			calc := NewCalculator(zerolog.Nop(), cachedClient)
			req := newEstimateCostRequest(t, "azure:storage/managedDisk:ManagedDisk", map[string]any{
				"location":  "eastus",
				"disk_type": "Premium_SSD_LRS",
				"size_gb":   tc.sizeGB,
			})

			resp, err := calc.EstimateCost(context.Background(), req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got: %v", err)
				}
				if st.Code() != tc.errCode {
					t.Fatalf("expected code %v, got %v", tc.errCode, st.Code())
				}
				return
			}

			if err != nil {
				t.Fatalf("EstimateCost() failed: %v", err)
			}
			if math.Abs(resp.GetCostMonthly()-tc.wantCost) > 0.001 {
				t.Errorf("cost = %.2f, want %.2f", resp.GetCostMonthly(), tc.wantCost)
			}
		})
	}
}

func newEstimateCostRequest(
	t *testing.T,
	resourceType string,
	attributes map[string]any,
) *finfocusv1.EstimateCostRequest {
	t.Helper()

	req := &finfocusv1.EstimateCostRequest{
		ResourceType: resourceType,
	}
	if attributes == nil {
		return req
	}

	attrs, err := structpb.NewStruct(attributes)
	if err != nil {
		t.Fatalf("NewStruct() failed: %v", err)
	}
	req.Attributes = attrs
	return req
}

func newPriceServer(
	t *testing.T,
	items []azureclient.PriceItem,
	counter *atomic.Int32,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if counter != nil {
			counter.Add(1)
		}

		resp := azureclient.PriceResponse{
			Items: items,
			Count: len(items),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
			http.Error(w, "encode response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}))
}

func assertStatusCodeContains(
	t *testing.T,
	err error,
	wantCode codes.Code,
	substrings ...string,
) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected gRPC error %v, got nil", wantCode)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != wantCode {
		t.Fatalf("expected code %v, got %v (message: %q)", wantCode, st.Code(), st.Message())
	}

	msg := strings.ToLower(st.Message())
	for _, substr := range substrings {
		if !strings.Contains(msg, strings.ToLower(substr)) {
			t.Fatalf("expected error message %q to contain %q", st.Message(), substr)
		}
	}
}

func TestUnitPriceAndCurrencyFallbacks(t *testing.T) {
	t.Parallel()

	price, currency, err := unitPriceAndCurrency([]azureclient.PriceItem{
		{
			UnitPrice: 0.031,
		},
	})
	if err != nil {
		t.Fatalf("unitPriceAndCurrency() failed: %v", err)
	}
	if price != 0.031 {
		t.Fatalf("expected fallback to UnitPrice, got %f", price)
	}
	if currency != "USD" {
		t.Fatalf("expected default USD currency, got %q", currency)
	}
}

func newCalculatorTestCachedClient(t *testing.T, baseURL string) *azureclient.CachedClient {
	t.Helper()

	clientConfig := azureclient.DefaultConfig()
	clientConfig.BaseURL = baseURL
	clientConfig.RetryMax = 0
	clientConfig.RetryWaitMin = time.Millisecond
	clientConfig.RetryWaitMax = 5 * time.Millisecond
	clientConfig.Timeout = 3 * time.Second
	clientConfig.Logger = zerolog.Nop()

	client, err := azureclient.NewClient(clientConfig)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	cacheConfig := azureclient.DefaultCacheConfig()
	cacheConfig.MaxSize = 100
	cacheConfig.TTL = time.Hour
	cacheConfig.ExpiresAtTTL = 4 * time.Hour
	cacheConfig.Logger = zerolog.Nop()

	cachedClient, err := azureclient.NewCachedClient(client, cacheConfig)
	if err != nil {
		t.Fatalf("NewCachedClient() failed: %v", err)
	}

	return cachedClient
}
