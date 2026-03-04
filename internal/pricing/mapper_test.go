package pricing

import (
	"errors"
	"strings"
	"testing"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// --- US1: Map VM Resource to Pricing Query ---

func TestMapDescriptorToQuery_ValidVM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		desc         *finfocusv1.ResourceDescriptor
		wantRegion   string
		wantSKU      string
		wantService  string
		wantCurrency string
	}{
		{
			name: "standard VM with region and SKU",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "compute/VirtualMachine",
				Sku:          "Standard_B1s",
				Region:       "eastus",
			},
			wantRegion:   "eastus",
			wantSKU:      "Standard_B1s",
			wantService:  "Virtual Machines",
			wantCurrency: "USD",
		},
		{
			name: "preserves full Azure SKU name without normalization",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "compute/VirtualMachine",
				Sku:          "Standard_D2s_v3",
				Region:       "westeurope",
			},
			wantRegion:   "westeurope",
			wantSKU:      "Standard_D2s_v3",
			wantService:  "Virtual Machines",
			wantCurrency: "USD",
		},
		{
			name: "defaults to USD currency",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "compute/VirtualMachine",
				Sku:          "Standard_B1s",
				Region:       "eastus",
			},
			wantRegion:   "eastus",
			wantSKU:      "Standard_B1s",
			wantService:  "Virtual Machines",
			wantCurrency: "USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			query, err := MapDescriptorToQuery(tt.desc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if query.ArmRegionName != tt.wantRegion {
				t.Errorf("ArmRegionName = %q, want %q", query.ArmRegionName, tt.wantRegion)
			}
			if query.ArmSkuName != tt.wantSKU {
				t.Errorf("ArmSkuName = %q, want %q", query.ArmSkuName, tt.wantSKU)
			}
			if query.ServiceName != tt.wantService {
				t.Errorf("ServiceName = %q, want %q", query.ServiceName, tt.wantService)
			}
			if query.CurrencyCode != tt.wantCurrency {
				t.Errorf("CurrencyCode = %q, want %q", query.CurrencyCode, tt.wantCurrency)
			}
		})
	}
}

// --- US2: Map Disk Resource to Pricing Query ---

func TestMapDescriptorToQuery_DiskResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desc        *finfocusv1.ResourceDescriptor
		wantService string
		wantSKU     string
		wantRegion  string
	}{
		{
			name: "ManagedDisk Premium_LRS maps to Managed Disks",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/ManagedDisk",
				Sku:          "Premium_LRS",
				Region:       "westus2",
			},
			wantService: "Managed Disks",
			wantSKU:     "Premium_LRS",
			wantRegion:  "westus2",
		},
		{
			name: "ManagedDisk Standard_LRS maps to Managed Disks",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/ManagedDisk",
				Sku:          "Standard_LRS",
				Region:       "eastus",
			},
			wantService: "Managed Disks",
			wantSKU:     "Standard_LRS",
			wantRegion:  "eastus",
		},
		{
			name: "BlobStorage maps to Storage",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/BlobStorage",
				Sku:          "Standard_LRS",
				Region:       "eastus",
			},
			wantService: "Storage",
			wantSKU:     "Standard_LRS",
			wantRegion:  "eastus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			query, err := MapDescriptorToQuery(tt.desc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if query.ServiceName != tt.wantService {
				t.Errorf("ServiceName = %q, want %q", query.ServiceName, tt.wantService)
			}
			if query.ArmSkuName != tt.wantSKU {
				t.Errorf("ArmSkuName = %q, want %q", query.ArmSkuName, tt.wantSKU)
			}
			if query.ArmRegionName != tt.wantRegion {
				t.Errorf("ArmRegionName = %q, want %q", query.ArmRegionName, tt.wantRegion)
			}
		})
	}
}

// --- US3: Validate Resource Description Completeness ---

func TestMapDescriptorToQuery_MissingRegion(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Sku:          "Standard_B1s",
		// Region missing, no tags
	}
	_, err := MapDescriptorToQuery(desc)
	if !errors.Is(err, ErrMissingRequiredFields) {
		t.Fatalf("expected ErrMissingRequiredFields, got: %v", err)
	}
	if !strings.Contains(err.Error(), "region") {
		t.Errorf("expected error to mention 'region', got: %v", err)
	}
}

func TestMapDescriptorToQuery_MissingSKU(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Region:       "eastus",
		// SKU missing, no tags
	}
	_, err := MapDescriptorToQuery(desc)
	if !errors.Is(err, ErrMissingRequiredFields) {
		t.Fatalf("expected ErrMissingRequiredFields, got: %v", err)
	}
	if !strings.Contains(err.Error(), "sku") {
		t.Errorf("expected error to mention 'sku', got: %v", err)
	}
}

func TestMapDescriptorToQuery_BothFieldsMissing(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		// Both region and SKU missing
	}
	_, err := MapDescriptorToQuery(desc)
	if !errors.Is(err, ErrMissingRequiredFields) {
		t.Fatalf("expected ErrMissingRequiredFields, got: %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "region") || !strings.Contains(msg, "sku") {
		t.Errorf("expected error to name both 'region' and 'sku', got: %v", err)
	}
}

func TestMapDescriptorToQuery_TagFallback(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		// Primary fields empty — use tag fallback
		Tags: map[string]string{
			"region": "eastus",
			"sku":    "Standard_B1s",
		},
	}
	query, err := MapDescriptorToQuery(desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query.ArmRegionName != "eastus" {
		t.Errorf("ArmRegionName = %q, want %q", query.ArmRegionName, "eastus")
	}
	if query.ArmSkuName != "Standard_B1s" {
		t.Errorf("ArmSkuName = %q, want %q", query.ArmSkuName, "Standard_B1s")
	}
}

func TestMapDescriptorToQuery_EmptyStringTreatedAsMissing(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Region:       "",
		Sku:          "",
		// No tags
	}
	_, err := MapDescriptorToQuery(desc)
	if !errors.Is(err, ErrMissingRequiredFields) {
		t.Fatalf("expected ErrMissingRequiredFields for empty strings, got: %v", err)
	}
}

func TestMapDescriptorToQuery_PrimaryFieldTakesPrecedence(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Region:       "eastus",
		Sku:          "Standard_B1s",
		Tags: map[string]string{
			"region": "westus2",
			"sku":    "Standard_D2s_v3",
		},
	}
	query, err := MapDescriptorToQuery(desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query.ArmRegionName != "eastus" {
		t.Errorf("ArmRegionName = %q, want %q (primary should take precedence)", query.ArmRegionName, "eastus")
	}
	if query.ArmSkuName != "Standard_B1s" {
		t.Errorf("ArmSkuName = %q, want %q (primary should take precedence)", query.ArmSkuName, "Standard_B1s")
	}
}

func TestMapDescriptorToQuery_NilDescriptor(t *testing.T) {
	t.Parallel()

	_, err := MapDescriptorToQuery(nil)
	if !errors.Is(err, ErrMissingRequiredFields) {
		t.Fatalf("expected ErrMissingRequiredFields for nil descriptor, got: %v", err)
	}
}

// --- US4: Handle Unsupported Resource Types ---

func TestMapDescriptorToQuery_UnsupportedResourceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		desc         *finfocusv1.ResourceDescriptor
		wantContains string
	}{
		{
			name: "unknown type network/LoadBalancer",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "network/LoadBalancer",
				Sku:          "Standard",
				Region:       "eastus",
			},
			wantContains: "network/LoadBalancer",
		},
		{
			name: "completely unknown type custom/Widget",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "custom/Widget",
				Sku:          "Standard",
				Region:       "eastus",
			},
			wantContains: "custom/Widget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := MapDescriptorToQuery(tt.desc)
			if !errors.Is(err, ErrUnsupportedResourceType) {
				t.Fatalf("expected ErrUnsupportedResourceType, got: %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("expected error to contain %q, got: %v", tt.wantContains, err)
			}
		})
	}
}

func TestMapDescriptorToQuery_NonAzureProvider(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "aws",
		ResourceType: "compute/VirtualMachine",
		Sku:          "Standard_B1s",
		Region:       "eastus",
	}
	_, err := MapDescriptorToQuery(desc)
	if !errors.Is(err, ErrUnsupportedResourceType) {
		t.Fatalf("expected ErrUnsupportedResourceType, got: %v", err)
	}
	if !strings.Contains(err.Error(), "aws") {
		t.Errorf("expected error to contain provider name 'aws', got: %v", err)
	}
}

func TestMapDescriptorToQuery_CaseInsensitiveResourceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
	}{
		{name: "mixed case", resourceType: "Compute/VirtualMachine"},
		{name: "all uppercase", resourceType: "COMPUTE/VIRTUALMACHINE"},
		{name: "all lowercase", resourceType: "compute/virtualmachine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			desc := &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: tt.resourceType,
				Sku:          "Standard_B1s",
				Region:       "eastus",
			}
			query, err := MapDescriptorToQuery(desc)
			if err != nil {
				t.Fatalf("unexpected error for resource type %q: %v", tt.resourceType, err)
			}
			if query.ServiceName != "Virtual Machines" {
				t.Errorf("ServiceName = %q, want %q", query.ServiceName, "Virtual Machines")
			}
		})
	}
}

// --- SupportedResourceTypes ---

func TestSupportedResourceTypes(t *testing.T) {
	t.Parallel()

	types := SupportedResourceTypes()
	expected := []string{
		"compute/VirtualMachine",
		"storage/BlobStorage",
		"storage/ManagedDisk",
	}
	if len(types) != len(expected) {
		t.Fatalf("expected %d types, got %d: %v", len(expected), len(types), types)
	}
	for i, want := range expected {
		if types[i] != want {
			t.Errorf("types[%d] = %q, want %q", i, types[i], want)
		}
	}
}
