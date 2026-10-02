package pricing

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// projectedFocusFieldsFilled are the FOCUS names a successful projected-cost
// response fills: cost_per_month, currency, billing_detail, unit_price, and
// pricing_category. provider_name is not one of them.
func projectedFocusFieldsFilled() []string {
	return []string{
		"billed_cost",
		"billing_currency",
		"charge_description",
		"list_unit_price",
		"pricing_category",
	}
}

func TestHandleDryRunNilResource(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	tests := []struct {
		name string
		req  *finfocusv1.DryRunRequest
	}{
		{name: "nil request", req: nil},
		{name: "nil resource", req: &finfocusv1.DryRunRequest{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.HandleDryRun(context.Background(), tt.req)
			if resp != nil {
				t.Fatalf("HandleDryRun() response = %v, want nil", resp)
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("HandleDryRun() code = %s, want InvalidArgument (%v)", status.Code(err), err)
			}

			delegated, delegatedErr := calc.DryRun(context.Background(), tt.req)
			if delegated != nil {
				t.Fatalf("DryRun() response = %v, want nil", delegated)
			}
			if status.Code(delegatedErr) != codes.InvalidArgument {
				t.Fatalf("DryRun() code = %s, want InvalidArgument (%v)", status.Code(delegatedErr), delegatedErr)
			}
		})
	}
}

func TestDryRunDelegatesToHandleDryRun(t *testing.T) {
	t.Parallel()

	calc := newDryRunCalc(t)
	req := &finfocusv1.DryRunRequest{
		Resource: dryRunDescriptors()["compute/VirtualMachine"],
	}

	delegated, delegatedErr := calc.DryRun(context.Background(), req)
	direct, directErr := calc.HandleDryRun(context.Background(), req)
	if delegatedErr != nil || directErr != nil {
		t.Fatalf("DryRun() err = %v, HandleDryRun() err = %v", delegatedErr, directErr)
	}
	if !proto.Equal(delegated, direct) {
		t.Fatalf("DryRun() = %v, HandleDryRun() = %v", delegated, direct)
	}
	assertSupportedDryRun(t, direct)
}

func TestHandleDryRunMissingFields(t *testing.T) {
	t.Parallel()

	calc := newDryRunCalc(t)
	resp, err := calc.HandleDryRun(context.Background(), &finfocusv1.DryRunRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "compute/VirtualMachine",
		},
	})
	if err != nil {
		t.Fatalf("HandleDryRun() error = %v", err)
	}
	if !resp.GetResourceTypeSupported() {
		t.Fatal("resource_type_supported = false")
	}
	if resp.GetConfigurationValid() {
		t.Fatal("configuration_valid = true")
	}
	joined := strings.Join(resp.GetConfigurationErrors(), " ")
	if !strings.Contains(joined, "region") || !strings.Contains(joined, "sku") {
		t.Fatalf("configuration_errors = %q, want region and sku", joined)
	}
	assertProjectedFieldMappings(t, resp.GetFieldMappings())
	assertResponseOmitsFilter(t, resp)
}

func TestHandleDryRunUnsupported(t *testing.T) {
	t.Parallel()

	calc := newDryRunCalc(t)
	tests := []struct {
		name string
		desc *finfocusv1.ResourceDescriptor
	}{
		{
			name: "unknown type",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "compute/VirtualMachineScaleSet",
				Region:       "eastus",
				Sku:          "Standard_B1s",
			},
		},
		{
			name: "non-azure provider",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "aws",
				ResourceType: "compute/VirtualMachine",
				Region:       "eastus",
				Sku:          "Standard_B1s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.HandleDryRun(context.Background(), &finfocusv1.DryRunRequest{
				Resource: tt.desc,
			})
			if err != nil {
				t.Fatalf("HandleDryRun() error = %v", err)
			}
			if resp.GetResourceTypeSupported() {
				t.Fatal("resource_type_supported = true")
			}
			if !resp.GetConfigurationValid() {
				t.Fatalf("configuration_valid = false, errors %v", resp.GetConfigurationErrors())
			}
			if len(resp.GetConfigurationErrors()) != 0 {
				t.Fatalf("configuration_errors = %v", resp.GetConfigurationErrors())
			}
			if len(resp.GetFieldMappings()) != 0 {
				t.Fatalf("field mappings = %d, want empty", len(resp.GetFieldMappings()))
			}
		})
	}
}

func TestHandleDryRunEverySupportedType(t *testing.T) {
	t.Parallel()

	descriptors := dryRunDescriptors()
	seen := make(map[string]struct{}, len(descriptors))
	for _, resourceType := range SupportedResourceTypes() {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()

			desc, ok := descriptors[resourceType]
			if !ok {
				t.Fatalf("no projected-cost descriptor for %s", resourceType)
			}
			if desc.GetResourceType() != resourceType {
				t.Fatalf("descriptor type = %q, want %q", desc.GetResourceType(), resourceType)
			}

			calc := newDryRunCalc(t)
			resp, err := calc.HandleDryRun(context.Background(), &finfocusv1.DryRunRequest{
				Resource: desc,
			})
			if err != nil {
				t.Fatalf("HandleDryRun() error = %v", err)
			}
			assertSupportedDryRun(t, resp)
		})
		seen[resourceType] = struct{}{}
	}
	if len(seen) != len(SupportedResourceTypes()) || len(seen) != len(descriptors) {
		t.Fatalf(
			"covered %d types, supported %d, descriptors %d",
			len(seen),
			len(SupportedResourceTypes()),
			len(descriptors),
		)
	}
}

func TestDryRunOverGRPCDoesNotCallHTTP(t *testing.T) {
	t.Parallel()

	calc := newDryRunCalc(t)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	finfocusv1.RegisterCostSourceServiceServer(server, pluginsdk.NewServer(calc))
	go func() {
		_ = server.Serve(lis)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///"+lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	client := finfocusv1.NewCostSourceServiceClient(conn)
	descriptors := dryRunDescriptors()
	for _, resourceType := range SupportedResourceTypes() {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()
			resp, err := client.DryRun(context.Background(), &finfocusv1.DryRunRequest{
				Resource: descriptors[resourceType],
			})
			if err != nil {
				t.Fatalf("DryRun() error = %v", err)
			}
			assertSupportedDryRun(t, resp)
		})
	}
}

func TestEstimateCostEveryMappedTypeOverGRPC(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, nil)
	client := dialPricingClient(t, calc)
	descriptors := dryRunDescriptors()
	for _, resourceType := range SupportedResourceTypes() {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()
			attrs := estimateAttrsFromDescriptor(t, descriptors[resourceType])
			_, err := client.EstimateCost(context.Background(), &finfocusv1.EstimateCostRequest{
				ResourceType: resourceType,
				Attributes:   attrs,
			})
			if status.Code(err) == codes.Unimplemented {
				t.Fatalf("EstimateCost unimplemented for %s: %v", resourceType, err)
			}
			if err != nil && status.Code(err) != codes.NotFound {
				t.Fatalf("EstimateCost code = %s, err %v", status.Code(err), err)
			}
		})
	}
}

func estimateAttrsFromDescriptor(t *testing.T, desc *finfocusv1.ResourceDescriptor) *structpb.Struct {
	t.Helper()

	values := map[string]any{}
	if desc.GetRegion() != "" {
		values["location"] = desc.GetRegion()
	}
	if desc.GetSku() != "" {
		values["sku"] = desc.GetSku()
	}
	for key, value := range desc.GetTags() {
		values[key] = value
	}
	attrs, err := structpb.NewStruct(values)
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	return attrs
}

func assertSupportedDryRun(t *testing.T, resp *finfocusv1.DryRunResponse) {
	t.Helper()

	if resp == nil {
		t.Fatal("response is nil")
	}
	if !resp.GetResourceTypeSupported() {
		t.Fatal("resource_type_supported = false")
	}
	if !resp.GetConfigurationValid() {
		t.Fatalf("configuration_valid = false, errors %v", resp.GetConfigurationErrors())
	}
	if len(resp.GetConfigurationErrors()) != 0 {
		t.Fatalf("configuration_errors = %v", resp.GetConfigurationErrors())
	}
	assertProjectedFieldMappings(t, resp.GetFieldMappings())
	assertResponseOmitsFilter(t, resp)
}

func assertResponseOmitsFilter(t *testing.T, resp *finfocusv1.DryRunResponse) {
	t.Helper()

	text := resp.String()
	for _, fragment := range []string{"armRegionName", "priceType eq", "$filter"} {
		if strings.Contains(text, fragment) {
			t.Fatalf("DryRunResponse contains OData filter fragment %q: %s", fragment, text)
		}
	}
}

func assertProjectedFieldMappings(t *testing.T, mappings []*finfocusv1.FieldMapping) {
	t.Helper()

	names := pluginsdk.FocusFieldNames()
	if len(names) == 0 {
		t.Fatal("FocusFieldNames() is empty")
	}
	focus := make(map[string]struct{}, len(names))
	for _, name := range names {
		focus[name] = struct{}{}
	}
	if _, ok := focus["billed_cost"]; !ok {
		t.Fatal("FocusFieldNames() does not include billed_cost")
	}
	if _, ok := focus["provider_name"]; !ok {
		t.Fatal("FocusFieldNames() does not include provider_name")
	}
	if len(mappings) != len(names) {
		t.Fatalf("field mappings = %d, FocusFieldNames = %d", len(mappings), len(names))
	}

	got := make(map[string]finfocusv1.FieldSupportStatus, len(mappings))
	supported := 0
	supportedStatus := finfocusv1.FieldSupportStatus_FIELD_SUPPORT_STATUS_SUPPORTED
	unsupportedStatus := finfocusv1.FieldSupportStatus_FIELD_SUPPORT_STATUS_UNSUPPORTED
	for _, mapping := range mappings {
		name := mapping.GetFieldName()
		if _, ok := focus[name]; !ok {
			t.Fatalf("mapping %q is not a FOCUS field", name)
		}
		if _, dup := got[name]; dup {
			t.Fatalf("duplicate mapping %q", name)
		}
		got[name] = mapping.GetSupportStatus()
		if mapping.GetConditionDescription() != "" {
			t.Fatalf("condition_description on %s = %q", name, mapping.GetConditionDescription())
		}
		if mapping.GetSupportStatus() == supportedStatus {
			supported++
		}
	}
	for _, name := range names {
		if _, ok := got[name]; !ok {
			t.Fatalf("missing mapping %q", name)
		}
	}
	if supported == 0 || supported == len(mappings) {
		t.Fatalf("supported fields = %d, want a non-empty subset of %d", supported, len(mappings))
	}
	for _, name := range projectedFocusFieldsFilled() {
		if got[name] != supportedStatus {
			t.Fatalf("%s status = %s, want SUPPORTED", name, got[name])
		}
	}
	for _, name := range []string{"provider_name", "invoice_id", "effective_cost", "service_name", "region_id"} {
		if got[name] != unsupportedStatus {
			t.Fatalf("%s status = %s, want UNSUPPORTED", name, got[name])
		}
	}
	if supported != len(projectedFocusFieldsFilled()) {
		t.Fatalf("supported fields = %d, want %d", supported, len(projectedFocusFieldsFilled()))
	}
}

func newDryRunCalc(t *testing.T) *Calculator {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("DryRun called %s %s", r.Method, r.URL.String())
		http.Error(w, "dry run must not call prices", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	return NewCalculator(zerolog.Nop(), cached)
}

// dryRunDescriptors reuses the descriptor shapes the projected-cost tests accept.
func dryRunDescriptors() map[string]*finfocusv1.ResourceDescriptor {
	return map[string]*finfocusv1.ResourceDescriptor{
		"compute/VirtualMachine": {
			Provider:     "azure",
			ResourceType: "compute/VirtualMachine",
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
		"storage/ManagedDisk": {
			Provider:     "azure",
			ResourceType: "storage/ManagedDisk",
			Region:       "eastus",
			Sku:          "Premium_SSD_LRS",
			Tags:         map[string]string{"size_gb": "100"},
		},
		"storage/BlobStorage": {
			Provider:     "azure",
			ResourceType: "storage/BlobStorage",
			Region:       "eastus",
			Sku:          "Hot LRS",
			Tags:         map[string]string{"size_gb": "100"},
		},
		"storage/StorageAccount": {
			Provider:     "azure",
			ResourceType: "storage/StorageAccount",
			Region:       "eastus",
			Sku:          "Hot LRS",
			Tags:         map[string]string{"size_gb": "100"},
		},
		"web/AppServicePlan": {
			Provider:     "azure",
			ResourceType: "web/AppServicePlan",
			Region:       "eastus",
			Sku:          "P1v3",
		},
		"web/FunctionApp": {
			Provider:     "azure",
			ResourceType: "web/FunctionApp",
			Region:       "eastus",
			Sku:          "Standard",
			Tags: map[string]string{
				"executions": strconv.Itoa(testFreeExecutions),
				"gb_seconds": strconv.Itoa(testFreeGBSeconds),
			},
		},
		"containerservice/KubernetesCluster": {
			Provider:     "azure",
			ResourceType: "containerservice/KubernetesCluster",
			Region:       "eastus",
			Sku:          "Standard",
			Tags:         twoPoolTags("Standard_D2s_v3", nil),
		},
		"sql/Database": {
			Provider:     "azure",
			ResourceType: "sql/Database",
			Region:       "eastus",
			Sku:          "GP_Gen5_2",
			Tags:         map[string]string{"size_gb": "100"},
		},
		"cosmosdb/Account": {
			Provider:     "azure",
			ResourceType: "cosmosdb/Account",
			Region:       "eastus",
			Tags:         map[string]string{"ru_per_second": "400", "size_gb": "10"},
		},
		"network/LoadBalancer": {
			Provider:     "azure",
			ResourceType: "network/LoadBalancer",
			Region:       "eastus",
			Sku:          "Standard",
		},
	}
}
