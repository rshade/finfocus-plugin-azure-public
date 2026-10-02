package pricing

import (
	"fmt"
	"strings"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// buildFocusRecord maps one actual-cost quote onto a FOCUS record.
// billingAccountID is the operator-supplied process setting. An empty id
// fails ValidateFocusRecord. The request has no billing account field, and
// this function does not invent one.
func buildFocusRecord(
	resource *finfocusv1.ResourceDescriptor,
	quote monthlyQuote,
	window actualWindow,
	billingAccountID string,
	resourceID string,
) (*finfocusv1.FocusCostRecord, error) {
	resourceType := ""
	if resource != nil {
		resourceType = resource.GetResourceType()
	}

	var tags map[string]string
	if resource != nil {
		tags = resource.GetTags()
	}
	category, err := focusServiceCategory(resourceType, tags)
	if err != nil {
		return nil, err
	}

	query, err := MapDescriptorToQuery(resource)
	if err != nil {
		return nil, err
	}

	description := quote.billingDetail
	if description == "" {
		description = resourceType
	}

	cost := quote.monthly * (window.hours / pluginsdk.HoursPerMonth)
	end := window.start.Add(time.Duration(window.hours * float64(time.Hour)))
	builder := pluginsdk.NewFocusRecordBuilder().
		WithIdentity(providerAzure, billingAccountID, "").
		WithBillingPeriod(window.start, end, quote.currency).
		WithChargePeriod(window.start, end).
		WithChargeDetails(
			finfocusv1.FocusChargeCategory_FOCUS_CHARGE_CATEGORY_USAGE,
			focusPricingCategory(resource),
		).
		WithChargeClassification(
			finfocusv1.FocusChargeClass_FOCUS_CHARGE_CLASS_REGULAR,
			description,
			finfocusv1.FocusChargeFrequency_FOCUS_CHARGE_FREQUENCY_USAGE_BASED,
		).
		WithFinancials(cost, cost, cost, quote.currency, "").
		WithServiceCategory(category).
		WithService(category, query.ServiceName).
		WithSKU(quote.sku, "").
		WithLocation(quote.region, quote.region, "").
		WithResource(resourceID, "", resourceType)
	if window.hours > 0 {
		builder = builder.
			WithPricing(window.hours, specUnitHourName, cost/window.hours).
			WithUsage(window.hours, specUnitHourName)
	}

	return builder.Build()
}

func focusServiceCategory(resourceType string, tags map[string]string) (finfocusv1.FocusServiceCategory, error) {
	lower := strings.ToLower(strings.TrimSpace(resourceType))
	switch {
	case isVirtualMachineResourceType(lower) ||
		isAppServicePlanResourceType(lower) ||
		isFunctionAppResourceType(lower) ||
		isNativeFunctionWebApp(lower, tags) ||
		isAKSResourceType(lower):
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE, nil
	case isManagedDiskResourceType(lower) ||
		isBlobStorageResourceType(lower) ||
		isStorageAccountResourceType(lower):
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE, nil
	case isSQLDatabaseResourceType(lower) ||
		isCosmosAccountResourceType(lower):
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE, nil
	case isLoadBalancerResourceType(lower):
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_NETWORK, nil
	default:
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_UNSPECIFIED,
			fmt.Errorf("unsupported resource type: %s: %w", resourceType, ErrUnsupportedResourceType)
	}
}

func focusPricingCategory(resource *finfocusv1.ResourceDescriptor) finfocusv1.FocusPricingCategory {
	if resource == nil {
		return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
	}
	priority := strings.TrimSpace(resource.GetTags()["priority"])
	if strings.EqualFold(priority, vmPrioritySpot) {
		return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
	}
	return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
}
