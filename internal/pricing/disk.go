package pricing

import (
	"fmt"
	"math"
	"strings"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// diskTypeInfo holds the Azure API mapping for a supported disk type.
type diskTypeInfo struct {
	ArmSkuName string
	TierPrefix string
	Redundancy string
}

// supportedDiskTypes maps user-facing disk type names (lowercased) to Azure API values.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var supportedDiskTypes = map[string]diskTypeInfo{
	"standard_lrs":    {ArmSkuName: "Standard_LRS", TierPrefix: "S", Redundancy: "LRS"},
	"standardssd_lrs": {ArmSkuName: "StandardSSD_LRS", TierPrefix: "E", Redundancy: "LRS"},
	"premium_ssd_lrs": {ArmSkuName: "Premium_LRS", TierPrefix: "P", Redundancy: "LRS"},
	"standard_zrs":    {ArmSkuName: "Standard_ZRS", TierPrefix: "S", Redundancy: "ZRS"},
	"standardssd_zrs": {ArmSkuName: "StandardSSD_ZRS", TierPrefix: "E", Redundancy: "ZRS"},
	"premium_zrs":     {ArmSkuName: "Premium_ZRS", TierPrefix: "P", Redundancy: "ZRS"},
}

// diskTierCapacity maps tier numbers to their provisioned capacity in GiB.
// Tier numbers are consistent across all disk types (S, E, P).
type diskTierCapacity struct {
	Number   int
	Capacity int
}

// diskTierCapacities is the static table of tier numbers to GiB capacities,
// sorted by capacity ascending for ceiling-match lookup.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var diskTierCapacities = []diskTierCapacity{
	{Number: 1, Capacity: 4},
	{Number: 2, Capacity: 8},
	{Number: 3, Capacity: 16},
	{Number: 4, Capacity: 32},
	{Number: 6, Capacity: 64},
	{Number: 10, Capacity: 128},
	{Number: 15, Capacity: 256},
	{Number: 20, Capacity: 512},
	{Number: 30, Capacity: 1024},
	{Number: 40, Capacity: 2048},
	{Number: 50, Capacity: 4096},
	{Number: 60, Capacity: 8192},
	{Number: 70, Capacity: 16384},
	{Number: 80, Capacity: 32767},
}

// normalizeDiskType validates and normalizes a user-facing disk type name
// to its Azure API mapping. Input is case-insensitive.
// Returns the diskTypeInfo or an error for unsupported types.
func normalizeDiskType(diskType string) (diskTypeInfo, error) {
	info, ok := supportedDiskTypes[strings.ToLower(diskType)]
	if !ok {
		return diskTypeInfo{}, fmt.Errorf("unsupported disk type: %s", diskType)
	}
	return info, nil
}

// tierForSize returns the tier name (e.g., "P10", "S30") for a given prefix
// and requested size in GB using ceiling-match. The size is rounded up to the
// nearest integer before matching. Returns an error if the size exceeds the
// largest available tier.
func tierForSize(prefix string, sizeGB float64) (string, error) {
	rounded := int(math.Ceil(sizeGB))
	for _, tier := range diskTierCapacities {
		if rounded <= tier.Capacity {
			return fmt.Sprintf("%s%d", prefix, tier.Number), nil
		}
	}
	return "", fmt.Errorf("no disk tier found for %d GB (max supported: %d GB)",
		rounded, diskTierCapacities[len(diskTierCapacities)-1].Capacity)
}

// isManagedDiskResourceType checks whether the resource type string refers to
// storage/manageddisk as a segment (case-insensitive), consistent with the
// isVirtualMachineResourceType pattern.
func isManagedDiskResourceType(lower string) bool {
	const segment = "storage/manageddisk"
	idx := strings.Index(lower, segment)
	if idx < 0 {
		return false
	}
	end := idx + len(segment)
	if end == len(lower) {
		return true
	}
	next := lower[end]
	return next == ':' || next == '/' || next == ' '
}

// selectDiskTierPrice filters Azure price items by the target tier's meter name
// and returns the retail price and currency. For ZRS disk types, the meter name
// includes a " ZRS" suffix (e.g., "P10 ZRS").
func selectDiskTierPrice(items []azureclient.PriceItem, tierName string, redundancy string) (float64, string, error) {
	meterName := tierName
	if redundancy == "ZRS" {
		meterName = tierName + " ZRS"
	}

	for _, item := range items {
		if item.MeterName == meterName {
			price := item.RetailPrice
			if price == 0 {
				price = item.UnitPrice
			}
			currency := item.CurrencyCode
			if strings.TrimSpace(currency) == "" {
				currency = "USD"
			}
			return price, currency, nil
		}
	}

	return 0, "", fmt.Errorf("no pricing found for disk tier %s: %w", meterName, azureclient.ErrNotFound)
}
