package pricing

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	kindLoadBalancer = "loadbalancer"

	loadBalancerServiceName   = "Load Balancer"
	loadBalancerSegment       = "network/loadbalancer"
	loadBalancerPulumiLB      = "lb/loadbalancer"
	canonicalLoadBalancer     = "network/LoadBalancer"
	loadBalancerPriceRegion   = "Global"
	loadBalancerSKUStandard   = "Standard"
	loadBalancerIncludedRules = 5

	loadBalancerMeterIncluded = "Standard Included LB Rules and Outbound Rules"
	loadBalancerMeterOverage  = "Standard Overage LB Rules and Outbound Rules"
	loadBalancerMeterData     = "Standard Data Processed"
	loadBalancerUnitData      = "1 GB"

	loadBalancerComponentRules   = "rules"
	loadBalancerComponentOverage = "rule_overage"
	loadBalancerComponentData    = "data_processed"
)

// quoteLoadBalancer prices Standard Load Balancer rules.
// A regional page with no included-rules meter is read again at Global.
// An omitted rule_count bills the included meter once, which covers up to
// five load-balancing and outbound rules. rule_count 0 has no hourly charge.
// Rules above that included count use the overage meter. data_processed_gb
// is charged only when the caller sets it. Gateway and the cross-region sku
// are rejected.
func (c *Calculator) quoteLoadBalancer(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	region := descriptorRegion(resource)
	if region == "" {
		return monthlyQuote{}, missingFieldsError([]string{missingFieldRegion})
	}
	sku, err := loadBalancerSKU(resource)
	if err != nil {
		return monthlyQuote{}, err
	}
	usage, err := parseLoadBalancerUsage(resource.GetTags())
	if err != nil {
		return monthlyQuote{}, err
	}
	page, priceRegion, err := c.loadBalancerPage(ctx, region, descriptorCurrency(resource), taskID)
	if err != nil {
		return monthlyQuote{}, err
	}
	return loadBalancerQuote(resource, sku, region, priceRegion, page, usage)
}

type loadBalancerUsage struct {
	rules    int
	rulesSet bool
	dataGB   float64
	dataSet  bool
}

func loadBalancerSKU(resource *finfocusv1.ResourceDescriptor) (string, error) {
	sku := descriptorSKU(resource)
	if sku == "" || strings.EqualFold(sku, loadBalancerSKUStandard) {
		return loadBalancerSKUStandard, nil
	}
	return "", status.Errorf(
		codes.InvalidArgument,
		"load balancer sku %q is not quoted; Standard rules are the priced sku",
		sku,
	)
}

func parseLoadBalancerUsage(tags map[string]string) (loadBalancerUsage, error) {
	var usage loadBalancerUsage
	raw := firstNonEmptyTag(tags, "rule_count", "rules")
	if raw != "" {
		rules, err := strconv.Atoi(raw)
		if err != nil || rules < 0 {
			return loadBalancerUsage{}, status.Errorf(codes.InvalidArgument, "invalid rule_count %q", raw)
		}
		usage.rules = rules
		usage.rulesSet = true
	}
	dataRaw := firstNonEmptyTag(tags, "data_processed_gb", "dataProcessedGb")
	if dataRaw == "" {
		return usage, nil
	}
	dataGB, err := parseNonNegative("data_processed_gb", dataRaw)
	if err != nil {
		return loadBalancerUsage{}, err
	}
	usage.dataGB = dataGB
	usage.dataSet = true
	return usage, nil
}

func (c *Calculator) loadBalancerPage(
	ctx context.Context,
	region, currency, taskID string,
) (azureclient.CachedResult, string, error) {
	result, err := c.fetchPrices(ctx, loadBalancerQuery(region, currency), taskID)
	if err == nil && (loadBalancerHasIncluded(result.Items) || strings.EqualFold(region, loadBalancerPriceRegion)) {
		return result, region, nil
	}
	if err != nil && status.Code(err) != codes.NotFound {
		return azureclient.CachedResult{}, "", err
	}
	if strings.EqualFold(region, loadBalancerPriceRegion) {
		if err != nil {
			return azureclient.CachedResult{}, "", err
		}
		return result, region, nil
	}
	global, globalErr := c.fetchPrices(ctx, loadBalancerQuery(loadBalancerPriceRegion, currency), taskID)
	if globalErr != nil {
		return azureclient.CachedResult{}, "", globalErr
	}
	return global, loadBalancerPriceRegion, nil
}

func loadBalancerQuery(region, currency string) azureclient.PriceQuery {
	return azureclient.PriceQuery{
		ArmRegionName: region,
		ServiceName:   loadBalancerServiceName,
		CurrencyCode:  currency,
	}
}

func loadBalancerHasIncluded(items []azureclient.PriceItem) bool {
	_, err := selectLoadBalancerMeter(items, loadBalancerMeterIncluded, appServiceUnitHour)
	return err == nil
}

func loadBalancerQuote(
	resource *finfocusv1.ResourceDescriptor,
	sku, region, priceRegion string,
	page azureclient.CachedResult,
	usage loadBalancerUsage,
) (monthlyQuote, error) {
	priced, err := priceLoadBalancer(page.Items, usage)
	if err != nil {
		return monthlyQuote{}, loadBalancerStatus(err)
	}
	quote := meterQuote(
		resource,
		sku,
		priced.currency,
		loadBalancerDetail(region, priceRegion, sku, usage, priced.overageRules),
		priced.monthly,
		page.ExpiresAt,
		priced.components,
	)
	quote.unitPrice = priced.unitPrice
	quote.meters = priced.meters
	return quote, nil
}

type loadBalancerPriced struct {
	unitPrice    float64
	monthly      float64
	currency     string
	overageRules int
	components   map[string]float64
	meters       []quoteMeter
}

func priceLoadBalancer(items []azureclient.PriceItem, usage loadBalancerUsage) (loadBalancerPriced, error) {
	included, err := selectLoadBalancerMeter(items, loadBalancerMeterIncluded, appServiceUnitHour)
	if err != nil {
		return loadBalancerPriced{}, err
	}
	priced := loadBalancerPriced{
		unitPrice:  included.RetailPrice,
		currency:   itemCurrency(included),
		components: map[string]float64{},
		meters: []quoteMeter{{
			key:   loadBalancerComponentRules,
			price: included.RetailPrice,
			unit:  included.UnitOfMeasure,
		}},
	}
	if rulesErr := addLoadBalancerRules(&priced, items, included, usage); rulesErr != nil {
		return loadBalancerPriced{}, rulesErr
	}
	if dataErr := addLoadBalancerData(&priced, items, usage); dataErr != nil {
		return loadBalancerPriced{}, dataErr
	}
	if len(priced.components) == 0 {
		priced.components[loadBalancerComponentRules] = 0
	}
	for _, value := range priced.components {
		priced.monthly += value
	}
	return priced, nil
}

func addLoadBalancerRules(
	priced *loadBalancerPriced,
	items []azureclient.PriceItem,
	included azureclient.PriceItem,
	usage loadBalancerUsage,
) error {
	if usage.rulesSet && usage.rules == 0 {
		return nil
	}
	priced.components[loadBalancerComponentRules] = included.RetailPrice * pluginsdk.HoursPerMonth
	rules := loadBalancerIncludedRules
	if usage.rulesSet {
		rules = usage.rules
	}
	if rules <= loadBalancerIncludedRules {
		return nil
	}
	overage, err := selectLoadBalancerMeter(items, loadBalancerMeterOverage, cosmosUnitPerHour)
	if err != nil {
		return err
	}
	priced.overageRules = rules - loadBalancerIncludedRules
	priced.components[loadBalancerComponentOverage] =
		float64(priced.overageRules) * overage.RetailPrice * pluginsdk.HoursPerMonth
	priced.meters = append(priced.meters, quoteMeter{
		key:   loadBalancerComponentOverage,
		price: overage.RetailPrice,
		unit:  overage.UnitOfMeasure,
	})
	return nil
}

func addLoadBalancerData(priced *loadBalancerPriced, items []azureclient.PriceItem, usage loadBalancerUsage) error {
	if !usage.dataSet || usage.dataGB == 0 {
		return nil
	}
	data, err := selectLoadBalancerMeter(items, loadBalancerMeterData, loadBalancerUnitData)
	if err != nil {
		return err
	}
	priced.components[loadBalancerComponentData] = usage.dataGB * data.RetailPrice
	priced.meters = append(priced.meters, quoteMeter{
		key:   loadBalancerComponentData,
		price: data.RetailPrice,
		unit:  data.UnitOfMeasure,
	})
	return nil
}

func selectLoadBalancerMeter(items []azureclient.PriceItem, meter, unit string) (azureclient.PriceItem, error) {
	var chosen *azureclient.PriceItem
	for i := range items {
		item := items[i]
		if !loadBalancerRowMatches(item, meter, unit) || strings.TrimSpace(item.EffectiveEndDate) != "" {
			continue
		}
		if chosen != nil && (item.RetailPrice != chosen.RetailPrice || itemCurrency(item) != itemCurrency(*chosen)) {
			return azureclient.PriceItem{}, status.Errorf(codes.InvalidArgument, "ambiguous price for meter %s", meter)
		}
		chosen = &item
	}
	if chosen == nil {
		return azureclient.PriceItem{}, fmt.Errorf(
			"no load balancer price for meter %s: %w",
			meter,
			azureclient.ErrNotFound,
		)
	}
	return *chosen, nil
}

func loadBalancerRowMatches(item azureclient.PriceItem, meter, unit string) bool {
	return item.ServiceName == loadBalancerServiceName &&
		strings.EqualFold(item.SkuName, loadBalancerSKUStandard) &&
		item.MeterName == meter &&
		item.UnitOfMeasure == unit &&
		item.Type == storagePriceTypeConsumption
}

func loadBalancerDetail(region, priceRegion, sku string, usage loadBalancerUsage, overageRules int) string {
	where := region
	if !strings.EqualFold(region, priceRegion) {
		where = region + " using " + priceRegion + " prices"
	}
	return fmt.Sprintf(
		"Load Balancer %s in %s. %s. %s. Inbound NAT rules are not counted.",
		sku,
		where,
		loadBalancerRuleNote(usage, overageRules),
		loadBalancerDataNote(usage),
	)
}

func loadBalancerRuleNote(usage loadBalancerUsage, overageRules int) string {
	switch {
	case !usage.rulesSet:
		return "rule_count omitted, so the included rules meter is used once and covers up to 5 load-balancing and outbound rules"
	case usage.rules == 0:
		return "no rules configured, so there is no hourly charge"
	case overageRules == 0:
		return fmt.Sprintf("%d rules stay inside the included 5", usage.rules)
	default:
		return fmt.Sprintf("%d rules, with %d above the included 5", usage.rules, overageRules)
	}
}

func loadBalancerDataNote(usage loadBalancerUsage) string {
	if !usage.dataSet {
		return "data processed omitted"
	}
	return fmt.Sprintf("%g GB data processed", usage.dataGB)
}

func loadBalancerStatus(err error) error {
	if err == nil || status.Code(err) != codes.Unknown {
		return err
	}
	return MapToGRPCStatus(err).Err()
}

func isLoadBalancerResourceType(lower string) bool {
	return resourceSegment(lower, loadBalancerSegment) ||
		resourceSegment(lower, loadBalancerPulumiLB) ||
		tokenSuffix(lower, "network", "loadbalancer")
}
