package pricing

import (
	"fmt"
	"strings"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// FOCUS Unit Format values and the FOCUS 1.3 provider name for Azure. The name
// matches PublisherName in microsoft/finops-toolkit Services.csv.
const (
	focusUnitHours         = "Hours"
	focusUnitMonths        = "Months"
	focusProviderMicrosoft = "Microsoft"
)

// buildFocusRecord maps one actual-cost quote onto a FOCUS record.
// billingAccountID comes from the request when that id is set, and otherwise
// from the process setting. An empty id fails ValidateFocusRecord. This
// function does not invent one.
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
	category, subcategory, err := focusServiceClass(resourceType, tags)
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
		WithIdentity(focusProviderMicrosoft, billingAccountID, billingAccountID).
		WithServiceProvider(focusProviderMicrosoft).
		WithHostProvider(focusProviderMicrosoft).
		WithInvoice("", focusProviderMicrosoft).
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
		WithContractedCost(cost).
		WithService(category, query.ServiceName).
		WithServiceSubcategory(subcategory).
		WithSKU(quote.sku, "").
		WithLocation(quote.region, quote.region, "").
		WithResource(resourceID, "", resourceType)
	// FOCUS 1.3 deprecates PublisherName but still lists it as mandatory.
	//nolint:staticcheck // SA1019: required until FOCUS 1.4 removes the column.
	builder = builder.WithPublisher(focusProviderMicrosoft)
	if window.hours > 0 {
		quantity, unit, unitPrice := focusPricingBasis(quote.meters, cost, window.hours)
		builder = builder.
			WithPricing(quantity, unit, unitPrice).
			WithContractedUnitPrice(unitPrice).
			WithUsage(quantity, unit)
	}

	return builder.Build()
}

// focusServiceClass returns the FOCUS 1.3 ServiceCategory and
// ServiceSubcategory for a resource type. The pairs follow
// microsoft/finops-toolkit Services.csv, so rows line up with Azure's own
// FOCUS data. App Service plans map to Web / Application Platforms there,
// but FocusServiceCategory has no Web value until rshade/finfocus-spec#612,
// so they use Compute / Other (Compute).
func focusServiceClass(
	resourceType string,
	tags map[string]string,
) (finfocusv1.FocusServiceCategory, string, error) {
	lower := strings.ToLower(strings.TrimSpace(resourceType))
	compute := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE
	database := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE
	switch {
	case isPricedVMResourceType(lower) || isManagedDiskResourceType(lower):
		return compute, "Virtual Machines", nil
	case isFunctionAppResourceType(lower) || isNativeFunctionWebApp(lower, tags):
		return compute, "Serverless Compute", nil
	case isAppServicePlanResourceType(lower):
		return compute, "Other (Compute)", nil
	case isAKSResourceType(lower):
		return compute, "Containers", nil
	case isBlobStorageResourceType(lower) || isStorageAccountResourceType(lower):
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE, "Storage Platforms", nil
	case isSQLDatabaseResourceType(lower):
		return database, "Relational Databases", nil
	case isCosmosAccountResourceType(lower):
		return database, "NoSQL Databases", nil
	case isLoadBalancerResourceType(lower):
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_NETWORK, "Application Networking", nil
	default:
		return finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_UNSPECIFIED, "",
			fmt.Errorf("unsupported resource type: %s: %w", resourceType, ErrUnsupportedResourceType)
	}
}

// focusPricingBasis returns the FOCUS pricing quantity, unit, and unit price
// for one cost window. A quote priced by exactly one positive hourly or
// monthly meter is expressed in that meter's unit, so quantity is cost divided
// by the meter price (instance-hours for a scale set, months for a disk).
// Any other quote, including several meters, GB-month bands, and zero prices,
// is expressed in window hours. In both cases unit price times quantity is the
// cost.
func focusPricingBasis(meters []quoteMeter, cost, hours float64) (float64, string, float64) {
	if len(meters) == 1 && meters[0].price > 0 && cost > 0 {
		meter := meters[0]
		switch unit := strings.ToLower(strings.TrimSpace(meter.unit)); unit {
		case "1 hour":
			return cost / meter.price, focusUnitHours, meter.price
		case "1/month", "1 month":
			return cost / meter.price, focusUnitMonths, meter.price
		}
	}
	return hours, focusUnitHours, cost / hours
}

func focusPricingCategory(resource *finfocusv1.ResourceDescriptor) finfocusv1.FocusPricingCategory {
	spot, err := descriptorSpot(resource)
	if err == nil && spot {
		return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
	}
	return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
}
