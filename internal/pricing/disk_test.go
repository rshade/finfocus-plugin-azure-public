package pricing

import (
	"strings"
	"testing"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// Phase 2: Foundational Tests (T003-T005)

func TestNormalizeDiskType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantSKU    string
		wantPrefix string
		wantRedund string
		wantErr    bool
	}{
		{name: "Standard_LRS", input: "Standard_LRS", wantSKU: "Standard_LRS", wantPrefix: "S", wantRedund: "LRS"},
		{name: "StandardSSD_LRS", input: "StandardSSD_LRS", wantSKU: "StandardSSD_LRS", wantPrefix: "E", wantRedund: "LRS"},
		{name: "Premium_SSD_LRS", input: "Premium_SSD_LRS", wantSKU: "Premium_LRS", wantPrefix: "P", wantRedund: "LRS"},
		{name: "Standard_ZRS", input: "Standard_ZRS", wantSKU: "Standard_ZRS", wantPrefix: "S", wantRedund: "ZRS"},
		{name: "StandardSSD_ZRS", input: "StandardSSD_ZRS", wantSKU: "StandardSSD_ZRS", wantPrefix: "E", wantRedund: "ZRS"},
		{name: "Premium_ZRS", input: "Premium_ZRS", wantSKU: "Premium_ZRS", wantPrefix: "P", wantRedund: "ZRS"},
		{name: "case_insensitive_lower", input: "premium_ssd_lrs", wantSKU: "Premium_LRS", wantPrefix: "P", wantRedund: "LRS"},
		{name: "case_insensitive_upper", input: "STANDARD_LRS", wantSKU: "Standard_LRS", wantPrefix: "S", wantRedund: "LRS"},
		{name: "case_insensitive_mixed", input: "StandardSSD_lrs", wantSKU: "StandardSSD_LRS", wantPrefix: "E", wantRedund: "LRS"},
		{name: "unsupported_UltraSSD", input: "UltraSSD_LRS", wantErr: true},
		{name: "unsupported_PremiumV2", input: "PremiumV2_LRS", wantErr: true},
		{name: "unsupported_empty", input: "", wantErr: true},
		{name: "unsupported_garbage", input: "not_a_disk_type", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			info, err := normalizeDiskType(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if info.ArmSkuName != tc.wantSKU {
				t.Errorf("ArmSkuName: got %q, want %q", info.ArmSkuName, tc.wantSKU)
			}
			if info.TierPrefix != tc.wantPrefix {
				t.Errorf("TierPrefix: got %q, want %q", info.TierPrefix, tc.wantPrefix)
			}
			if info.Redundancy != tc.wantRedund {
				t.Errorf("Redundancy: got %q, want %q", info.Redundancy, tc.wantRedund)
			}
		})
	}
}

func TestTierForSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prefix   string
		sizeGB   float64
		wantTier string
		wantErr  bool
	}{
		{name: "exact_32GB", prefix: "P", sizeGB: 32, wantTier: "P4"},
		{name: "exact_128GB", prefix: "P", sizeGB: 128, wantTier: "P10"},
		{name: "exact_512GB", prefix: "S", sizeGB: 512, wantTier: "S20"},
		{name: "exact_1024GB", prefix: "E", sizeGB: 1024, wantTier: "E30"},
		{name: "ceiling_100_to_128", prefix: "P", sizeGB: 100, wantTier: "P10"},
		{name: "ceiling_200_to_512", prefix: "P", sizeGB: 200, wantTier: "P15"},
		{name: "smallest_tier_4GB", prefix: "P", sizeGB: 4, wantTier: "P1"},
		{name: "smallest_tier_1GB", prefix: "E", sizeGB: 1, wantTier: "E1"},
		{name: "largest_tier", prefix: "S", sizeGB: 32767, wantTier: "S80"},
		{name: "exceeds_max", prefix: "P", sizeGB: 99999, wantErr: true},
		{name: "fractional_rounds_up", prefix: "P", sizeGB: 0.5, wantTier: "P1"},
		{name: "fractional_128_5", prefix: "P", sizeGB: 128.5, wantTier: "P15"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tier, err := tierForSize(tc.prefix, tc.sizeGB)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for size %.1f, got nil", tc.sizeGB)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for size %.1f: %v", tc.sizeGB, err)
			}
			if tier != tc.wantTier {
				t.Errorf("got tier %q, want %q", tier, tc.wantTier)
			}
		})
	}
}

func TestIsManagedDiskResourceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "short_form", input: "storage/manageddisk", want: true},
		{name: "full_form", input: "azure:storage/manageddisk:manageddisk", want: true},
		{name: "mixed_case_lower", input: "storage/manageddisk", want: true},
		{name: "with_colon_suffix", input: "azure:storage/manageddisk:ManagedDisk", want: true},
		{name: "vm_type", input: "compute/virtualmachine", want: false},
		{name: "empty_string", input: "", want: false},
		{name: "partial_match", input: "storage/managed", want: false},
		{name: "prefix_continuation", input: "storage/manageddiskset", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isManagedDiskResourceType(strings.ToLower(tc.input))
			if got != tc.want {
				t.Errorf("isManagedDiskResourceType(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSelectDiskTierPrice(t *testing.T) {
	t.Parallel()

	items := []azureclient.PriceItem{
		{MeterName: "P4", RetailPrice: 5.28, CurrencyCode: "USD"},
		{MeterName: "P10", RetailPrice: 19.71, CurrencyCode: "USD"},
		{MeterName: "P20", RetailPrice: 38.02, CurrencyCode: "USD"},
		{MeterName: "P10 ZRS", RetailPrice: 24.64, CurrencyCode: "USD"},
	}

	tests := []struct {
		name       string
		tierName   string
		redundancy string
		wantPrice  float64
		wantErr    bool
	}{
		{name: "LRS_P10", tierName: "P10", redundancy: "LRS", wantPrice: 19.71},
		{name: "LRS_P4", tierName: "P4", redundancy: "LRS", wantPrice: 5.28},
		{name: "ZRS_P10", tierName: "P10", redundancy: "ZRS", wantPrice: 24.64},
		{name: "not_found", tierName: "P30", redundancy: "LRS", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			price, _, err := selectDiskTierPrice(items, tc.tierName, tc.redundancy)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if price != tc.wantPrice {
				t.Errorf("price = %.2f, want %.2f", price, tc.wantPrice)
			}
		})
	}
}
