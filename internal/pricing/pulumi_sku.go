package pricing

import (
	"fmt"
	"strconv"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pulumiUnknownValue is what Pulumi prints at preview for an input that
// depends on another resource. It is a placeholder, not a value.
const pulumiUnknownValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

const (
	skuCapacityTag    = "sku.capacity"
	aksSupportPlanTag = "supportPlan"
	aksSupportPlanLTS = "AKSLongTermSupport"
	aksSKUBase        = "Base"
	diskTierTag       = "tier"
	premiumDiskPrefix = "P"

	workerCountMissingNote = "worker count not provided, priced as 1 worker"
	workerCountUnknownNote = "worker count unknown at preview, priced as 1 worker"
)

// workerCountTags are classic workerCount, then native sku.capacity.
func workerCountTags() []string {
	return []string{"workerCount", skuCapacityTag}
}

// pulumiTag is firstNonEmptyTag that also skips the unknown placeholder and
// the formatted nil the core prints for an absent value.
func pulumiTag(tags map[string]string, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(tags[key])
		if value != "" && value != pulumiUnknownValue && value != formattedNil {
			return value
		}
	}
	return ""
}

// pulumiSKU is the descriptor Sku unless it is the unknown placeholder.
func pulumiSKU(resource *finfocusv1.ResourceDescriptor) string {
	sku := strings.TrimSpace(resource.GetSku())
	if sku == pulumiUnknownValue {
		return ""
	}
	return sku
}

// skuWithPulumiKeys returns the descriptor Sku, then the per-type Pulumi
// properties in keys, then the generic tags descriptorSKU reads. The per-type
// property wins over a generic tag, so an unrelated tier never becomes the SKU.
func skuWithPulumiKeys(resource *finfocusv1.ResourceDescriptor, keys ...string) string {
	if sku := pulumiSKU(resource); sku != "" {
		return sku
	}
	if sku := pulumiTag(resource.GetTags(), keys...); sku != "" {
		return sku
	}
	return descriptorSKU(resource)
}

// diskSKU reads classic storageAccountType or native sku.name, for example
// Premium_LRS. The disk's tier is its performance tier, read by diskBillingTier.
func diskSKU(resource *finfocusv1.ResourceDescriptor) string {
	return skuWithPulumiKeys(resource, "storageAccountType", "sku.name")
}

// diskBillingTier returns the tier Azure bills: the size tier, or the Pulumi
// performance tier when it is a higher Premium SSD tier. A lower performance
// tier is ignored. A performance tier on a disk that is not Premium SSD is
// ignored, because only Premium SSD has one. A value that is not a Premium SSD
// tier is InvalidArgument.
func diskBillingTier(info diskTypeInfo, sizeTier string, tags map[string]string) (string, error) {
	raw := pulumiTag(tags, diskTierTag)
	if raw == "" || info.TierPrefix != premiumDiskPrefix {
		return sizeTier, nil
	}
	performance, ok := premiumDiskTierCapacity(raw)
	if !ok {
		return "", status.Errorf(codes.InvalidArgument, "unsupported disk performance tier %q", raw)
	}
	size, _ := premiumDiskTierCapacity(sizeTier)
	if performance > size {
		return strings.ToUpper(raw), nil
	}
	return sizeTier, nil
}

func premiumDiskTierCapacity(tier string) (int, bool) {
	upper := strings.ToUpper(strings.TrimSpace(tier))
	number, err := strconv.Atoi(strings.TrimPrefix(upper, premiumDiskPrefix))
	if err != nil || !strings.HasPrefix(upper, premiumDiskPrefix) {
		return 0, false
	}
	for _, capacity := range diskTierCapacities {
		if capacity.Number == number {
			return capacity.Capacity, true
		}
	}
	return 0, false
}

// appServicePlanSKU reads classic skuName or native sku.name, for example P1v3.
func appServicePlanSKU(resource *finfocusv1.ResourceDescriptor) string {
	return skuWithPulumiKeys(resource, "skuName", "sku.name")
}

// appServicePlanWorkers reads classic workerCount, then native sku.capacity.
// A missing or unknown count prices one worker and returns the note that
// says so, because the plan may have more.
func appServicePlanWorkers(tags map[string]string) (int, string, error) {
	raw := pulumiTag(tags, workerCountTags()...)
	if raw == "" {
		for _, key := range workerCountTags() {
			if strings.TrimSpace(tags[key]) == pulumiUnknownValue {
				return 1, workerCountUnknownNote, nil
			}
		}
		return 1, workerCountMissingNote, nil
	}
	count, err := strconv.Atoi(raw)
	if err != nil || count < 1 {
		return 0, "", status.Errorf(codes.InvalidArgument, "unsupported worker count %q", raw)
	}
	return count, "", nil
}

// aksTier returns the cluster tier. The descriptor Sku (or tag sku) wins
// unless it is native sku.name Base, which is not a tier: then native
// sku.tier, classic skuTier, or tag tier is read. Automatic in Sku is
// returned as is, so it is refused rather than priced as its sku.tier.
func aksTier(resource *finfocusv1.ResourceDescriptor) string {
	tags := resource.GetTags()
	name := pulumiSKU(resource)
	if name == "" {
		name = pulumiTag(tags, "sku")
	}
	if name != "" && !strings.EqualFold(name, aksSKUBase) {
		return name
	}
	if tier := pulumiTag(tags, "sku.tier", "skuTier", aksTierTag); tier != "" {
		return tier
	}
	return name
}

// aksSupport returns the support value that asks for long term support
// and whether it came from Pulumi supportPlan=AKSLongTermSupport rather than
// tag support=lts. Empty means none.
func aksSupport(tags map[string]string) (string, bool) {
	if support := strings.TrimSpace(tags[aksSupportTag]); strings.EqualFold(support, aksSupportLTS) {
		return support, false
	}
	if plan := pulumiTag(tags, aksSupportPlanTag); strings.EqualFold(plan, aksSupportPlanLTS) {
		return plan, true
	}
	return "", false
}

// sqlDatabaseSKU reads the descriptor Sku, classic skuName (GP_Gen5_4), or
// native sku.name. For a vCore family (GP_, BC_, HS_) native sku.capacity is
// the vCore count and is appended when the name has none, so GP_Gen5 with
// capacity 4 is GP_Gen5_4. A vCore name with no count anywhere is a missing
// sku.capacity. Other names, such as DTU S0, are returned unchanged.
func sqlDatabaseSKU(resource *finfocusv1.ResourceDescriptor) (string, error) {
	tags := resource.GetTags()
	base := skuWithPulumiKeys(resource, "skuName", "sku.name")
	if base == "" || sqlSKUHasCount(base) || !sqlVCoreFamily(base) {
		return base, nil
	}
	raw := pulumiTag(tags, skuCapacityTag)
	if raw == "" {
		return "", missingFieldsError([]string{skuCapacityTag})
	}
	capacity, err := strconv.Atoi(raw)
	if err != nil || capacity < 1 {
		return "", status.Errorf(codes.InvalidArgument, "invalid %s %q", skuCapacityTag, raw)
	}
	return base + "_" + strconv.Itoa(capacity), nil
}

func sqlVCoreFamily(sku string) bool {
	upper := strings.ToUpper(sku)
	for _, prefix := range []string{"GP_", "BC_", "HS_"} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

// validateAppServicePlanInput applies the plan quote's OS and worker checks
// without a price lookup, so Supports and DryRun refuse what it refuses.
func validateAppServicePlanInput(resource *finfocusv1.ResourceDescriptor) error {
	if _, err := descriptorWindows(resource); err != nil {
		return err
	}
	if _, _, err := appServicePlanWorkers(resource.GetTags()); err != nil {
		return err
	}
	return nil
}

// storageAccountKindError refuses an account kind that is not priced from
// General Block Blob v2. Empty and StorageV2 are accepted.
func storageAccountKindError(tags map[string]string) error {
	kind := pulumiTag(tags, "kind", "accountKind")
	if kind == "" || strings.EqualFold(kind, storageKindV2) {
		return nil
	}
	return fmt.Errorf(
		"unsupported storage account kind %q: only %s is priced from %s",
		kind,
		storageKindV2,
		generalBlockBlobV2Product,
	)
}

func sqlSKUHasCount(sku string) bool {
	idx := strings.LastIndex(sku, "_")
	if idx < 0 {
		return false
	}
	_, err := strconv.Atoi(sku[idx+1:])
	return err == nil
}
