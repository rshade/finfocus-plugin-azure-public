package pricing

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// FOCUS Unit Format values and the FOCUS 1.3 provider name for Azure. The
// units follow microsoft/finops-toolkit PricingUnits.csv (DistinctUnits), and
// the name matches PublisherName in its Services.csv.
const (
	focusUnitHours         = "Hours"
	focusUnitMonths        = "Months"
	focusUnitUnitsPerMonth = "Units/Month"
	focusProviderMicrosoft = "Microsoft"
	// focusVirtualMachines is both a Services.csv ServiceName and a FOCUS 1.3
	// ServiceSubcategory.
	focusVirtualMachines = "Virtual Machines"
)

// focusService is the FOCUS 1.3 ServiceName, ServiceCategory, and
// ServiceSubcategory of one priced resource.
type focusService struct {
	name        string
	category    finfocusv1.FocusServiceCategory
	subcategory string
}

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
	service, err := focusServiceClass(resourceType, tags, descriptorSKU(resource))
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
		WithService(service.category, service.name).
		WithServiceSubcategory(service.subcategory).
		WithSKU(quote.sku, "").
		WithLocation(quote.region, quote.region, "").
		WithResource(resourceID, "", resourceType)
	// FOCUS 1.3 deprecates PublisherName but still lists it as mandatory.
	//nolint:staticcheck // SA1019: required until FOCUS 1.4 removes the column.
	builder = builder.WithPublisher(focusProviderMicrosoft)
	if window.hours > 0 {
		quantity, unit, unitPrice := focusPricingBasis(quote.meters, quote.monthly, window.hours)
		builder = builder.
			WithPricing(quantity, unit, unitPrice).
			WithContractedUnitPrice(unitPrice).
			WithUsage(quantity, unit)
	}

	return builder.Build()
}

// focusServiceClass returns the FOCUS 1.3 service columns for a resource
// type. Name, category, and subcategory follow microsoft/finops-toolkit
// Services.csv, so rows line up with Azure's own FOCUS data. Azure App Service
// (server farms, and a Function App billed on a plan) is Web / Application
// Platforms there, but FocusServiceCategory has no Web value until
// rshade/finfocus-spec#612, so it uses Compute / Other (Compute). sku is only
// read to tell a Function App on a plan from a serverless one.
func focusServiceClass(resourceType string, tags map[string]string, sku string) (focusService, error) {
	lower := strings.ToLower(strings.TrimSpace(resourceType))
	compute := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE
	database := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE
	appService := focusService{name: "Azure App Service", category: compute, subcategory: "Other (Compute)"}
	switch {
	case isVirtualMachineScaleSetResourceType(lower):
		return focusService{
			name:        "Virtual Machine Scale Sets",
			category:    compute,
			subcategory: focusVirtualMachines,
		}, nil
	case isPricedVMResourceType(lower) || isManagedDiskResourceType(lower):
		return focusService{name: focusVirtualMachines, category: compute, subcategory: focusVirtualMachines}, nil
	case isFunctionAppResourceType(lower) || isNativeFunctionWebApp(lower, tags):
		if kind, err := functionQuoteKind(tags[tagPricingModel], sku); err == nil && kind == kindFunctionDedicated {
			return appService, nil
		}
		return focusService{name: "Functions", category: compute, subcategory: "Serverless Compute"}, nil
	case isAppServicePlanResourceType(lower):
		return appService, nil
	case isAKSResourceType(lower):
		return focusService{name: "Azure Kubernetes Service", category: compute, subcategory: "Containers"}, nil
	case isBlobStorageResourceType(lower) || isStorageAccountResourceType(lower):
		return focusService{
			name:        "Storage Accounts",
			category:    finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE,
			subcategory: "Storage Platforms",
		}, nil
	case isSQLDatabaseResourceType(lower):
		return focusService{name: "Azure SQL Database", category: database, subcategory: "Relational Databases"}, nil
	case isCosmosAccountResourceType(lower):
		return focusService{name: "Cosmos DB", category: database, subcategory: "NoSQL Databases"}, nil
	case isLoadBalancerResourceType(lower):
		return focusService{
			name:        "Load Balancer",
			category:    finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_NETWORK,
			subcategory: "Application Networking",
		}, nil
	default:
		return focusService{}, fmt.Errorf("unsupported resource type: %s: %w", resourceType, ErrUnsupportedResourceType)
	}
}

// focusPricingBasis returns the FOCUS pricing quantity, unit, and unit price
// for one cost window. A quote billed by exactly one positive hourly or
// monthly meter, whose monthly total is that meter times its count, is
// expressed in the meter's unit. The quantity is then counted directly
// (window hours times instances, or window months) rather than divided back
// out of the cost, so 24 hours stays exactly 24. Any other quote (several
// meters, GB-month bands, request units, zero prices) is expressed in window
// hours with both unit prices unset, because a blended rate is not a
// published price. ValidateFocusRecord only checks ContractedCost against
// unit price times quantity when both are non-zero.
func focusPricingBasis(meters []quoteMeter, monthly, hours float64) (float64, string, float64) {
	if len(meters) == 1 && meters[0].price > 0 && monthly > 0 {
		meter := meters[0]
		count := meter.count
		if count == 0 {
			count = 1
		}
		months := hours / pluginsdk.HoursPerMonth
		switch strings.ToLower(strings.TrimSpace(meter.unit)) {
		case "1 hour", "1 hours":
			if sameAmount(monthly, meter.price*pluginsdk.HoursPerMonth*count) {
				return hours * count, focusUnitHours, meter.price
			}
		case "1/month":
			if sameAmount(monthly, meter.price*count) {
				return months * count, focusUnitUnitsPerMonth, meter.price
			}
		case "1 month":
			if sameAmount(monthly, meter.price*count) {
				return months * count, focusUnitMonths, meter.price
			}
		}
	}
	return hours, focusUnitHours, 0
}

// sameAmount reports whether two money amounts agree to within float noise.
func sameAmount(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b))
}

func focusPricingCategory(resource *finfocusv1.ResourceDescriptor) finfocusv1.FocusPricingCategory {
	spot, err := descriptorSpot(resource)
	if err == nil && spot {
		return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
	}
	return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
}
