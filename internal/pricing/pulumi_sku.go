package pricing

import (
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
)

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

// skuWithPulumiKeys returns the descriptor Sku, then the per-type Pulumi
// properties in keys, then the generic tags descriptorSKU reads. The per-type
// property wins over a generic tag, so an unrelated tier never becomes the SKU.
func skuWithPulumiKeys(resource *finfocusv1.ResourceDescriptor, keys ...string) string {
	if sku := strings.TrimSpace(resource.GetSku()); sku != "" {
		return sku
	}
	if sku := pulumiTag(resource.GetTags(), keys...); sku != "" {
		return sku
	}
	return descriptorSKU(resource)
}

// diskSKU reads classic storageAccountType or native sku.name, for example
// Premium_LRS. A classic disk's tier is a performance tier and is not read.
func diskSKU(resource *finfocusv1.ResourceDescriptor) string {
	return skuWithPulumiKeys(resource, "storageAccountType", "sku.name")
}

// appServicePlanSKU reads classic skuName or native sku.name, for example P1v3.
func appServicePlanSKU(resource *finfocusv1.ResourceDescriptor) string {
	return skuWithPulumiKeys(resource, "skuName", "sku.name")
}

// appServicePlanWorkers reads classic workerCount, then native sku.capacity.
// A missing count is one worker.
func appServicePlanWorkers(tags map[string]string) (int, error) {
	raw := pulumiTag(tags, "workerCount", skuCapacityTag)
	if raw == "" {
		return 1, nil
	}
	count, err := strconv.Atoi(raw)
	if err != nil || count < 1 {
		return 0, status.Errorf(codes.InvalidArgument, "unsupported worker count %q", raw)
	}
	return count, nil
}

// aksTier reads native sku.tier or classic skuTier before the descriptor Sku.
// The core puts native sku.name (Base or Automatic) in Sku, which is not the tier.
func aksTier(resource *finfocusv1.ResourceDescriptor) string {
	tags := resource.GetTags()
	if tier := pulumiTag(tags, "sku.tier", "skuTier"); tier != "" {
		return tier
	}
	if sku := strings.TrimSpace(resource.GetSku()); sku != "" {
		return sku
	}
	return firstNonEmptyTag(tags, "sku", aksTierTag)
}

// aksSupport returns the support value that asks for long term support:
// tag support=lts or Pulumi supportPlan=AKSLongTermSupport. Empty means none.
func aksSupport(tags map[string]string) string {
	if support := strings.TrimSpace(tags[aksSupportTag]); strings.EqualFold(support, aksSupportLTS) {
		return support
	}
	if plan := pulumiTag(tags, aksSupportPlanTag); strings.EqualFold(plan, aksSupportPlanLTS) {
		return plan
	}
	return ""
}

// sqlDatabaseSKU reads the descriptor Sku, classic skuName (GP_Gen5_4), or
// native sku.name. Native sku.capacity is the vCore count and is appended
// when the name has none, so GP_Gen5 with capacity 4 is GP_Gen5_4.
func sqlDatabaseSKU(resource *finfocusv1.ResourceDescriptor) (string, error) {
	tags := resource.GetTags()
	base := skuWithPulumiKeys(resource, "skuName", "sku.name")
	if base == "" {
		return "", nil
	}
	raw := pulumiTag(tags, skuCapacityTag)
	if raw == "" || sqlSKUHasCount(base) {
		return base, nil
	}
	capacity, err := strconv.Atoi(raw)
	if err != nil || capacity < 1 {
		return "", status.Errorf(codes.InvalidArgument, "invalid %s %q", skuCapacityTag, raw)
	}
	return base + "_" + strconv.Itoa(capacity), nil
}

func sqlSKUHasCount(sku string) bool {
	idx := strings.LastIndex(sku, "_")
	if idx < 0 {
		return false
	}
	_, err := strconv.Atoi(sku[idx+1:])
	return err == nil
}
