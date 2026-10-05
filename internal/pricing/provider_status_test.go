package pricing

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestDescriptorRPCs_UnpricedInput_ReturnDocumentedCode pins the codes from
// issue #103. A provider this plugin does not price is InvalidArgument: FinFocus
// core's conformance check (internal/conformance/cost.go) sends an AWS resource
// and accepts only NotFound or InvalidArgument. An Azure type the plugin does not
// price is Unimplemented, the same as EstimateCost, whose request has no provider.
func TestDescriptorRPCs_UnpricedInput_ReturnDocumentedCode(t *testing.T) {
	t.Parallel()

	cachedClient := newCalculatorTestCachedClient(t, "http://127.0.0.1:1")
	client := newDiscoveryClient(t, NewCalculator(zerolog.Nop(), cachedClient))
	ctx := context.Background()

	tests := []struct {
		name     string
		resource *finfocusv1.ResourceDescriptor
		code     codes.Code
		message  string
		declines bool // Supports and DryRun must also decline this input
	}{
		{
			name: "core conformance AWS resource",
			resource: &finfocusv1.ResourceDescriptor{
				Provider: "aws", ResourceType: "invalid:resource", Region: "us-east-1", Sku: "non-existent",
			},
			code:     codes.InvalidArgument,
			message:  "unsupported provider: aws",
			declines: true,
		},
		{
			name: "AWS EC2 instance",
			resource: &finfocusv1.ResourceDescriptor{
				Provider: "aws", ResourceType: "ec2", Region: "us-east-1", Sku: "t3.micro",
			},
			code:     codes.InvalidArgument,
			message:  "unsupported provider: aws",
			declines: true,
		},
		{
			name: "GCP instance",
			resource: &finfocusv1.ResourceDescriptor{
				Provider: "gcp", ResourceType: "compute/Instance", Region: "us-central1", Sku: "e2-micro",
			},
			code:     codes.InvalidArgument,
			message:  "unsupported provider: gcp",
			declines: true,
		},
		{
			name: "empty provider",
			resource: &finfocusv1.ResourceDescriptor{
				ResourceType: "azure:compute/virtualMachine:VirtualMachine", Region: "eastus", Sku: "Standard_B1s",
			},
			code:    codes.InvalidArgument,
			message: "provider",
		},
		{
			name: "unknown Azure type",
			resource: &finfocusv1.ResourceDescriptor{
				Provider: providerAzure, ResourceType: "azure:foo/bar:Baz", Region: "eastus", Sku: "x",
			},
			code:     codes.Unimplemented,
			message:  "unsupported resource type",
			declines: true,
		},
	}
	for _, tt := range tests {
		calls := map[string]error{}
		_, calls["GetProjectedCost"] = client.GetProjectedCost(ctx,
			&finfocusv1.GetProjectedCostRequest{Resource: tt.resource})
		_, calls["GetActualCost"] = client.GetActualCost(ctx,
			&finfocusv1.GetActualCostRequest{ResourceId: "r1", Resource: tt.resource})
		_, calls["GetPricingSpec"] = client.GetPricingSpec(ctx,
			&finfocusv1.GetPricingSpecRequest{Resource: tt.resource})
		for rpc, err := range calls {
			st := status.Convert(err)
			if st.Code() != tt.code || !strings.Contains(st.Message(), tt.message) {
				t.Errorf("%s: %s = %v %q, want %v containing %q",
					tt.name, rpc, st.Code(), st.Message(), tt.code, tt.message)
			}
		}

		if !tt.declines {
			continue
		}
		supports, err := client.Supports(ctx, &finfocusv1.SupportsRequest{Resource: tt.resource})
		if err != nil || supports.GetSupported() || supports.GetReason() == "" {
			t.Errorf("%s: Supports = %v, %v; want unsupported with a reason", tt.name, supports, err)
		}
		dry, err := client.DryRun(ctx, &finfocusv1.DryRunRequest{Resource: tt.resource})
		if err != nil || dry.GetResourceTypeSupported() {
			t.Errorf("%s: DryRun = %v, %v; want the type unsupported", tt.name, dry, err)
		}
	}

	// The legacy actual-cost path, with no resource descriptor, reads the
	// provider from the request tags and must classify it the same way.
	_, err := client.GetActualCost(ctx, &finfocusv1.GetActualCostRequest{
		ResourceId: "r1",
		Tags: map[string]string{
			"provider": "aws", "resource_type": "ec2", "region": "us-east-1", "sku": "t3.micro",
		},
	})
	if st := status.Convert(err); st.Code() != codes.InvalidArgument ||
		!strings.Contains(st.Message(), "unsupported provider: aws") {
		t.Errorf("GetActualCost(tags only, aws) = %v %q, want InvalidArgument naming the provider",
			st.Code(), st.Message())
	}

	_, err = client.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{ResourceType: "aws:ec2/instance:Instance"})
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("EstimateCost(aws type) = %v, want Unimplemented", status.Code(err))
	}
}
