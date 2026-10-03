package pricing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	storageServiceName          = "Storage"
	generalBlockBlobV2Product   = "General Block Blob v2"
	storageDataStoredSuffix     = " Data Stored"
	storageUnitGBMonth          = "1 GB/Month"
	storagePriceTypeConsumption = "Consumption"

	storageSKUPartCount = 2

	storagePerformanceStandard = "Standard"
	storageDefaultAccessTier   = "Hot"
	storageKindV2              = "StorageV2"

	storageAccountResourceSegment = "storage/storageaccount"
)

// storageAccountTiers is the access tier in a Storage Account capacity SKU.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var storageAccountTiers = map[string]string{
	"hot":     "Hot",
	"cool":    "Cool",
	"cold":    "Cold",
	"archive": "Archive",
}

// storageAccountRedundancy is the redundancy in a Storage Account capacity SKU.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var storageAccountRedundancy = map[string]string{
	"lrs":     "LRS",
	"zrs":     "ZRS",
	"grs":     "GRS",
	"gzrs":    "GZRS",
	"ra-grs":  "RA-GRS",
	"ra-gzrs": "RA-GZRS",
	"ragrs":   "RA-GRS",
	"ragzrs":  "RA-GZRS",
}

func (c *Calculator) quoteStorageAccount(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	region := descriptorRegion(resource)
	sku, err := storageAccountSKU(resource)
	if err != nil {
		return monthlyQuote{}, storageAccountStatus(err)
	}
	sizeGB, sizeSet, err := descriptorSizeGB(resource)
	if err != nil {
		return monthlyQuote{}, status.Error(codes.InvalidArgument, err.Error())
	}

	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if sku == "" {
		missing = append(missing, "sku")
	}
	if !sizeSet {
		missing = append(missing, "size_gb")
	}
	if len(missing) > 0 {
		return monthlyQuote{}, missingFieldsError(missing)
	}

	result, err := c.fetchPrices(ctx, azureclient.PriceQuery{
		ArmRegionName: region,
		ServiceName:   storageServiceName,
		ProductName:   generalBlockBlobV2Product,
		CurrencyCode:  descriptorCurrency(resource),
	}, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}

	item, bands, err := storageAccountBands(result.Items, sku)
	if err != nil {
		return monthlyQuote{}, MapToGRPCStatus(err).Err()
	}

	monthly := marginalGBMonth(bands, sizeGB)
	currency := item.CurrencyCode
	if strings.TrimSpace(currency) == "" {
		currency = defaultCurrency
	}
	return monthlyQuote{
		unitPrice: item.RetailPrice,
		monthly:   monthly,
		currency:  currency,
		billingDetail: fmt.Sprintf(
			"Storage account %s %.0f GB-month in %s, transactions excluded",
			sku,
			sizeGB,
			region,
		),
		breakdownKey: breakdownStorage,
		expiresAt:    result.ExpiresAt,
		region:       region,
		sku:          sku,
		resourceType: resource.GetResourceType(),
		meters: []quoteMeter{{
			key:   breakdownStorage,
			price: item.RetailPrice,
			unit:  item.UnitOfMeasure,
		}},
	}, nil
}

// isStorageAccountResourceType reports whether lower contains the
// storage/storageaccount segment, including a Pulumi type such as
// azure:storage/storageAccount:StorageAccount.
func isStorageAccountResourceType(lower string) bool {
	return resourceSegment(lower, storageAccountResourceSegment) ||
		resourceSegment(lower, "storage/account") ||
		tokenSuffix(lower, "storage", "storageaccount")
}

// storageAccountSKU resolves "{Tier} {Redundancy}". Sku and Tags["sku"] win.
// An ARM SKU such as native sku.name Standard_GRS takes its access tier from
// accessTier, access_tier, or tier, default Hot. Next are the classic Pulumi
// accountTier, accountReplicationType, and accessTier. Otherwise the SKU is
// built from tier (or access_tier) and redundancy. Premium performance and
// an account kind other than StorageV2 are not General Block Blob v2 and are
// errors. An empty result means a missing sku.
func storageAccountSKU(resource *finfocusv1.ResourceDescriptor) (string, error) {
	tags := resource.GetTags()
	if err := storageAccountKindError(tags); err != nil {
		return "", err
	}
	if sku := pulumiSKU(resource); sku != "" {
		return storageSKUFromValue(sku, tags)
	}
	if sku := firstNonEmptyTag(tags, "sku"); sku != "" {
		return storageSKUFromValue(sku, tags)
	}
	if replication := pulumiTag(tags, "accountReplicationType"); replication != "" {
		return storageSKUFromARM(pulumiTag(tags, "accountTier"), replication, tags)
	}
	return storageAccountSKUFromTags(tags)
}

func storageSKUFromValue(raw string, tags map[string]string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	performance, replication, ok := strings.Cut(trimmed, "_")
	if ok && !strings.Contains(trimmed, " ") {
		return storageSKUFromARM(performance, replication, tags)
	}
	return canonicalStorageAccountSKU(trimmed)
}

func storageSKUFromARM(performance, replication string, tags map[string]string) (string, error) {
	if performance != "" && !strings.EqualFold(performance, storagePerformanceStandard) {
		return "", fmt.Errorf(
			"unsupported storage performance %q: only %s is priced from %s",
			performance,
			storagePerformanceStandard,
			generalBlockBlobV2Product,
		)
	}
	access := pulumiTag(tags, "accessTier", "access_tier", "tier")
	if access == "" {
		access = storageDefaultAccessTier
	}
	return canonicalStorageAccountSKU(access + " " + replication)
}

func storageAccountSKUFromTags(tags map[string]string) (string, error) {
	tier := firstNonEmptyTag(tags, "tier", "access_tier")
	redundancy := firstNonEmptyTag(tags, "redundancy")
	if tier == "" && redundancy == "" {
		return "", nil
	}
	if tier == "" {
		return "", fmt.Errorf("%w: tier", ErrMissingRequiredFields)
	}
	if redundancy == "" {
		return "", fmt.Errorf("%w: redundancy", ErrMissingRequiredFields)
	}
	return canonicalStorageAccountSKU(tier + " " + redundancy)
}

func canonicalStorageAccountSKU(raw string) (string, error) {
	parts := strings.Fields(raw)
	if len(parts) != storageSKUPartCount {
		return "", fmt.Errorf("unsupported storage sku %q", strings.TrimSpace(raw))
	}
	tier, ok := storageAccountTiers[strings.ToLower(parts[0])]
	if !ok {
		return "", fmt.Errorf("unsupported storage tier %q", parts[0])
	}
	redundancy, ok := storageAccountRedundancy[strings.ToLower(parts[1])]
	if !ok {
		return "", fmt.Errorf("unsupported storage redundancy %q", parts[1])
	}
	return tier + " " + redundancy, nil
}

// storageAccountBands returns the base row and every marginal band for sku,
// ordered by tierMinimumUnits. The base row is the lowest minimum. Duplicate
// minima keep the first row. Reserved-capacity rows contain TB or PB and are
// skipped. No match is ErrNotFound and names the tier and redundancy.
func storageAccountBands(
	items []azureclient.PriceItem,
	sku string,
) (azureclient.PriceItem, []azureclient.PriceItem, error) {
	wantMeter := sku + storageDataStoredSuffix
	var bands []azureclient.PriceItem
	for i := range items {
		item := items[i]
		if storageReservedCapacity(item) {
			continue
		}
		if item.ProductName != generalBlockBlobV2Product ||
			item.SkuName != sku ||
			item.MeterName != wantMeter ||
			item.UnitOfMeasure != storageUnitGBMonth ||
			item.Type != storagePriceTypeConsumption {
			continue
		}
		bands = append(bands, item)
	}
	if len(bands) == 0 {
		tier, redundancy := storageTierAndRedundancy(sku)
		return azureclient.PriceItem{}, nil, fmt.Errorf(
			"no storage account price for tier %s redundancy %s: %w",
			tier,
			redundancy,
			azureclient.ErrNotFound,
		)
	}
	sort.SliceStable(bands, func(i, j int) bool {
		return bands[i].TierMinimumUnits < bands[j].TierMinimumUnits
	})
	bands = uniqueTierBands(bands)
	return bands[0], bands, nil
}

func storageReservedCapacity(item azureclient.PriceItem) bool {
	name := item.SkuName + item.MeterName
	return strings.Contains(name, "TB") || strings.Contains(name, "PB")
}

func storageTierAndRedundancy(sku string) (string, string) {
	tier, redundancy, ok := strings.Cut(sku, " ")
	if !ok {
		return sku, ""
	}
	return tier, redundancy
}

func storageAccountStatus(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.Unknown {
		return err
	}
	if errors.Is(err, ErrMissingRequiredFields) || errors.Is(err, ErrUnsupportedResourceType) {
		return MapToGRPCStatus(err).Err()
	}
	return status.Error(codes.InvalidArgument, err.Error())
}
