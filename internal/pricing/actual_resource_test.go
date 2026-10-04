package pricing

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// actualWithResource is a GetActualCostRequest over hours that carries the
// v0.7.4 descriptor and the cloud tags a host sends beside it.
func actualWithResource(
	resource *finfocusv1.ResourceDescriptor,
	cloudTags map[string]string,
	hours float64,
) *finfocusv1.GetActualCostRequest {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return &finfocusv1.GetActualCostRequest{
		ResourceId:       "res-1",
		Start:            timestamppb.New(start),
		End:              timestamppb.New(start.Add(time.Duration(hours * float64(time.Hour)))),
		Tags:             cloudTags,
		Resource:         resource,
		BillingAccountId: "ba-actual-resource",
	}
}

func d2sv5Rows() []azureclient.PriceItem {
	return []azureclient.PriceItem{vmRow("Standard_D2s_v5", "Virtual Machines Dsv5 Series", "D2s v5", 0.115)}
}

func TestGetActualCost_ScaleSetCapacityInResource_PricesInstances(t *testing.T) {
	t.Parallel()

	resource := vmRequest("azure-native:compute:VirtualMachineScaleSet", "Standard_D2s_v5", nil).GetResource()
	resource.Attributes = mustAttributes(t, map[string]any{
		"sku": map[string]any{"name": "Standard_D2s_v5", "capacity": 3},
	})
	calc := newPricingCalc(t, d2sv5Rows())

	resp, err := calc.GetActualCost(context.Background(), actualWithResource(resource, nil, 730))
	if err != nil {
		t.Fatalf("GetActualCost() error = %v", err)
	}
	if want := 0.115 * 730 * 3; math.Abs(resp.GetResults()[0].GetCost()-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", resp.GetResults()[0].GetCost(), want)
	}
	record := resp.GetResults()[0].GetFocusRecord()
	if record == nil {
		t.Fatal("FocusRecord is nil")
	}
	if record.GetServiceName() != "Virtual Machine Scale Sets" || record.GetRegionId() != "westeurope" {
		t.Fatalf("FOCUS service %q region %q, want Virtual Machine Scale Sets in westeurope",
			record.GetServiceName(), record.GetRegionId())
	}
	if record.GetConsumedQuantity() != 730*3 {
		t.Fatalf("consumed quantity = %v, want %v instance-hours", record.GetConsumedQuantity(), 730*3)
	}
}

func TestGetActualCost_ResourceTagsWithoutAttributes_AreRead(t *testing.T) {
	t.Parallel()

	resource := vmRequest("azure-native:compute:VirtualMachineScaleSet", "Standard_D2s_v5",
		map[string]string{"sku.capacity": "2"}).GetResource()
	calc := newPricingCalc(t, d2sv5Rows())

	resp, err := calc.GetActualCost(context.Background(), actualWithResource(resource, nil, 730))
	if err != nil {
		t.Fatalf("GetActualCost() error = %v", err)
	}
	if want := 0.115 * 730 * 2; math.Abs(resp.GetResults()[0].GetCost()-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", resp.GetResults()[0].GetCost(), want)
	}
}

func TestGetActualCost_DiskSizeInResource_PicksTier(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		{
			ProductName: "Premium SSD Managed Disks", SkuName: "P10 LRS", MeterName: "P10 LRS Disk",
			RetailPrice: 19.71, CurrencyCode: "USD", UnitOfMeasure: "1/Month",
		},
		{
			ProductName: "Premium SSD Managed Disks", SkuName: "P15 LRS", MeterName: "P15 LRS Disk",
			RetailPrice: 38.01, CurrencyCode: "USD", UnitOfMeasure: "1/Month",
		},
	})
	resource := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "azure-native:compute:Disk",
		Region:       "eastus",
		Attributes: mustAttributes(t, map[string]any{
			"sku":        map[string]any{"name": "Premium_LRS"},
			"diskSizeGB": 200,
		}),
	}

	resp, err := calc.GetActualCost(context.Background(), actualWithResource(resource, nil, 730))
	if err != nil {
		t.Fatalf("GetActualCost() error = %v", err)
	}
	if got := resp.GetResults()[0].GetCost(); math.Abs(got-38.01) > 1e-9 {
		t.Fatalf("cost = %v, want the P15 price 38.01", got)
	}
}

// When resource is set, a cloud tag named region, sku, provider or
// resource_type is a label. It never reaches the Azure query.
func TestGetActualCost_ResourceSet_CloudTagsAreNotPricingInputs(t *testing.T) {
	t.Parallel()

	calc, filters := newCapturingPricingCalc(t, d2sv5Rows())
	resource := vmRequest("azure:compute/linuxVirtualMachine:LinuxVirtualMachine", "Standard_D2s_v5", nil).
		GetResource()
	cloudTags := map[string]string{
		"region":        "prod-eu",
		"sku":           "gold",
		"provider":      "aws",
		"resource_type": "aws:ec2/instance:Instance",
		"instances":     "9",
	}

	resp, err := calc.GetActualCost(context.Background(), actualWithResource(resource, cloudTags, 24))
	if err != nil {
		t.Fatalf("GetActualCost() error = %v", err)
	}
	if want := 0.115 * 24; math.Abs(resp.GetResults()[0].GetCost()-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", resp.GetResults()[0].GetCost(), want)
	}
	for _, filter := range filters() {
		if strings.Contains(filter, "prod-eu") || strings.Contains(filter, "gold") {
			t.Fatalf("filter %q carries a cloud tag", filter)
		}
	}
}

func TestGetActualCost_ResourceUnknownOrOversize_MatchesProjected(t *testing.T) {
	t.Parallel()

	oversize := &structpb.Struct{Fields: map[string]*structpb.Value{
		"blob": structpb.NewStringValue(strings.Repeat("x", 70_000)),
	}}
	tests := []struct {
		name  string
		attrs *structpb.Struct
	}{
		{name: "unknown location", attrs: mustAttributes(t, map[string]any{
			"location":        pulumiUnknownValue,
			"hardwareProfile": map[string]any{"vmSize": "Standard_D2s_v5"},
		})},
		{name: "oversize", attrs: oversize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc, filters := newCapturingPricingCalc(t, d2sv5Rows())
			resource := &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure-native:compute:VirtualMachine",
				Attributes:   tt.attrs,
			}
			_, projErr := calc.GetProjectedCost(context.Background(),
				&finfocusv1.GetProjectedCostRequest{Resource: resource})
			_, actErr := calc.GetActualCost(context.Background(), actualWithResource(resource, nil, 24))
			if status.Code(actErr) != codes.InvalidArgument || status.Code(actErr) != status.Code(projErr) {
				t.Fatalf("actual error %v, projected error %v, want both InvalidArgument", actErr, projErr)
			}
			if got := filters(); len(got) != 0 {
				t.Fatalf("Azure was queried: %v", got)
			}
		})
	}
}

func TestGetActualCost_ResourceSet_RequestUnmodified(t *testing.T) {
	t.Parallel()

	resource := vmRequest("azure-native:compute:VirtualMachineScaleSet", "Standard_D2s_v5", nil).GetResource()
	resource.Attributes = mustAttributes(t, map[string]any{"sku": map[string]any{"capacity": 3}})
	req := actualWithResource(resource, map[string]string{"team": "a"}, 24)
	calc := newPricingCalc(t, d2sv5Rows())

	if _, err := calc.GetActualCost(context.Background(), req); err != nil {
		t.Fatalf("GetActualCost() error = %v", err)
	}
	if req.GetResource().GetTags() != nil || req.GetResource().GetAttributes() == nil || len(req.GetTags()) != 1 {
		t.Fatalf("request was modified: %v", req)
	}
}
