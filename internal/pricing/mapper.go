package pricing

import (
	"fmt"
	"sort"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// defaultCurrency is the currency code used when no preference is specified.
const defaultCurrency = "USD"

// resourceTypeToService maps normalized (lowercased) resource type identifiers
// to their corresponding Azure service names for the Retail Prices API.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var resourceTypeToService = map[string]string{
	"compute/virtualmachine": "Virtual Machines",
	"storage/manageddisk":    "Managed Disks",
	"storage/blobstorage":    "Storage",
}

// canonicalResourceTypes maps normalized keys back to their display form.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var canonicalResourceTypes = map[string]string{
	"compute/virtualmachine": "compute/VirtualMachine",
	"storage/manageddisk":    "storage/ManagedDisk",
	"storage/blobstorage":    "storage/BlobStorage",
}

// MapDescriptorToQuery translates a finfocus ResourceDescriptor into an
// azureclient PriceQuery suitable for the Azure Retail Prices API.
//
// Validation is performed before mapping:
//   - Provider must be "azure" (case-insensitive)
//   - ResourceType must match a supported type (case-insensitive)
//   - Region must be resolvable (primary field or Tags["region"])
//   - SKU must be resolvable (primary field or Tags["sku"])
//
// Returns ErrUnsupportedResourceType for unknown providers or resource types.
// Returns ErrMissingRequiredFields naming all missing fields in a single error.
// Returns a valid *PriceQuery with CurrencyCode defaulted to "USD" on success.
func MapDescriptorToQuery(desc *finfocusv1.ResourceDescriptor) (*azureclient.PriceQuery, error) {
	if desc == nil {
		return nil, fmt.Errorf("%w: descriptor is nil", ErrMissingRequiredFields)
	}

	// Validate provider (case-insensitive).
	if !strings.EqualFold(desc.GetProvider(), "azure") {
		return nil, fmt.Errorf("unsupported provider: %s: %w", desc.GetProvider(), ErrUnsupportedResourceType)
	}

	// Look up resource type (case-insensitive).
	normalizedType := strings.ToLower(desc.GetResourceType())
	serviceName, ok := resourceTypeToService[normalizedType]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s: %w", desc.GetResourceType(), ErrUnsupportedResourceType)
	}

	// Resolve fields with tag fallback.
	region := resolveField(desc.GetRegion(), "region", desc.GetTags())
	sku := resolveField(desc.GetSku(), "sku", desc.GetTags())

	// Validate required fields — report all missing in one error.
	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if sku == "" {
		missing = append(missing, "sku")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrMissingRequiredFields, strings.Join(missing, ", "))
	}

	return &azureclient.PriceQuery{
		ArmRegionName: region,
		ArmSkuName:    sku,
		ServiceName:   serviceName,
		CurrencyCode:  defaultCurrency,
	}, nil
}

// SupportedResourceTypes returns the list of resource type identifiers that
// have a defined mapping to Azure service names. Resource types are returned
// in their canonical form (e.g., "compute/VirtualMachine") and sorted
// alphabetically.
func SupportedResourceTypes() []string {
	types := make([]string, 0, len(canonicalResourceTypes))
	for _, canonical := range canonicalResourceTypes {
		types = append(types, canonical)
	}
	sort.Strings(types)
	return types
}

// resolveField returns the primary value if non-empty, otherwise falls back
// to the tag value identified by tagKey. Returns empty string if neither is
// available.
func resolveField(primary, tagKey string, tags map[string]string) string {
	if primary != "" {
		return primary
	}
	if tags != nil {
		return tags[tagKey]
	}
	return ""
}
