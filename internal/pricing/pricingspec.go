package pricing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/protobuf/proto"

	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

const (
	pricingSpecTask       = "AZ-2.11"
	billingModePerHour    = "per_hour"
	billingModePerGBMonth = "per_gb_month"
	billingModePerMonth   = "per_month"
	billingModePerSecond  = "per_second"
	billingModePerRU      = "per_ru"
	// The modes below are pluginsdk pricing.BillingMode values finfocus core
	// does not read (internal/engine/pricing_spec.go normalizeBilling). Their
	// units are chosen so core's unit fallback does not read them either, so
	// core skips the spec instead of pricing the rate as hourly or GB-month.
	billingModePerVCPUHour = "per_vcpu_hour"
	billingModePerDataGB   = "per_data_transfer_gb"
	billingModeNone        = "not_implemented"
	specUnitHourName       = "hour"
	specUnitGBMonthName    = "GB-month"
	specUnitMonthName      = "Month"
	specUnitGBSecond       = "GB-second"
	specUnitVCPUHour       = "vCPU-hour"
	specUnitDataGB         = "GB processed"
	specUnitMillionRU      = "1M RU"
	providerAzure          = "azure"
	providerAzureNative    = "azure-native"

	// Usage input tags and hint units named in PricingSpec metric_hints.
	tagSizeGB      = "size_gb"
	tagWorkerCount = "workerCount"
	hintUnitCount  = "count"
	hintUnitGB     = "GB"
)

// GetPricingSpec returns one PricingSpec for the requested resource.
// pluginsdk.Server.GetPricingSpec calls this method directly.
// The rate is the selected meter's retail price from quoteResource, not the
// monthly total.
func (c *Calculator) GetPricingSpec(
	ctx context.Context,
	req *finfocusv1.GetPricingSpecRequest,
) (*finfocusv1.GetPricingSpecResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetPricingSpec request")

	resource, err := withAttributeTags(req.GetResource())
	if err != nil {
		log.Warn().Str("result_status", "error").Err(err).Msg("GetPricingSpec validation failed")
		return nil, err
	}
	kind, _ := classifyResource(resource)
	quote, err := c.quoteResource(ctx, resource, pricingSpecTask)
	var unsupplied []string
	if missing, ok := usageOnlyMissing(kind, err); ok {
		missingErr := err
		quote, err = c.quoteResource(ctx, withUsagePlaceholders(resource, kind, missing), pricingSpecTask)
		if err == nil && coreComputesMode(pricingSpecFromQuote(quote).GetBillingMode()) {
			err = missingErr
		}
		unsupplied = missing
	}
	if err != nil {
		log.Warn().Str("result_status", "error").Err(err).Msg("GetPricingSpec pricing failed")
		return nil, err
	}

	spec := pricingSpecFromQuote(quote)
	spec.Assumptions = specAssumptions(kind, quote, spec, unsupplied)
	spec.MetricHints = append(spec.GetMetricHints(), usageHints(kind, quote, spec.GetMetricHints())...)
	log.Info().
		Str("region", quote.region).
		Str("sku", quote.sku).
		Str("resource_type", quote.resourceType).
		Float64("rate_per_unit", spec.GetRatePerUnit()).
		Str("billing_mode", spec.GetBillingMode()).
		Str("result_status", "success").
		Msg("GetPricingSpec completed")

	return &finfocusv1.GetPricingSpecResponse{Spec: spec}, nil
}

func pricingSpecFromQuote(quote monthlyQuote) *finfocusv1.PricingSpec {
	meters := metersForSpec(quote)
	mode, unit, rate := specRate(meters)
	return &finfocusv1.PricingSpec{
		Provider:     providerAzure,
		ResourceType: quote.resourceType,
		Sku:          quote.sku,
		Region:       quote.region,
		BillingMode:  mode,
		RatePerUnit:  rate,
		Currency:     quote.currency,
		Description:  quote.billingDetail,
		MetricHints:  metricHints(meters),
		Source:       actualSourceName,
		Unit:         unit,
	}
}

func metersForSpec(quote monthlyQuote) []quoteMeter {
	if len(quote.meters) > 0 {
		return quote.meters
	}
	if quote.breakdownKey == "" {
		return nil
	}
	return []quoteMeter{{
		key:   quote.breakdownKey,
		price: quote.unitPrice,
	}}
}

// specBillingModes lists every billing mode specRate can return for a meter
// it recognises, sorted. The manifest's supported_resources billing_modes is
// built from it. billingModeNone is the fallback for an unrecognised meter and
// is not a mode the plugin supports, so it is not listed.
func specBillingModes() []string {
	return []string{
		billingModePerDataGB,
		billingModePerGBMonth,
		billingModePerHour,
		billingModePerMonth,
		billingModePerRU,
		billingModePerSecond,
		billingModePerVCPUHour,
	}
}

// specRate picks the meter whose price is rate_per_unit and names its mode.
// finfocus core prices per_hour (rate * 730), per_gb_month (rate * GB), and
// per_month (flat) itself. A Cosmos request-unit block, a Functions
// GB-second, a Premium vCPU-hour, and a Load Balancer processed GB have no
// core mode with that meaning, so they use the precise pluginsdk mode and a
// unit core does not read; core then skips the spec rather than misprice it.
func specRate(meters []quoteMeter) (string, string, float64) {
	if ru, ok := meterByKey(meters, cosmosComponentRU); ok {
		return billingModePerRU, ru.specUnit, ru.price
	}
	if gb, ok := consumptionGBSecond(meters); ok {
		return billingModePerSecond, specUnitGBSecond, gb.price
	}
	if vcpu, ok := meterByKey(meters, breakdownVCPU); ok {
		return billingModePerVCPUHour, specUnitVCPUHour, vcpu.price
	}
	if hourly, ok := firstMatchingMeter(meters, isHourMeter); ok {
		return billingModePerHour, specUnitHourName, hourly.price
	}
	if stored, ok := firstMatchingMeter(meters, isStorageMeter); ok {
		return billingModePerGBMonth, specUnitGBMonthName, stored.price
	}
	if monthly, ok := firstMatchingMeter(meters, isMonthMeter); ok {
		return billingModePerMonth, specUnitMonthName, monthly.price
	}
	if processed, ok := firstMatchingMeter(meters, isProcessedGBMeter); ok {
		return billingModePerDataGB, specUnitDataGB, processed.price
	}
	if len(meters) > 0 {
		return billingModeNone, meters[0].unit, meters[0].price
	}
	return billingModePerHour, specUnitHourName, 0
}

// coreComputesMode reports whether finfocus core turns a spec with this mode
// into a monthly total (rate * 730, rate * GB, or the flat rate). Such a spec
// must not be built from a placeholder usage quantity.
func coreComputesMode(mode string) bool {
	switch mode {
	case billingModePerHour, billingModePerGBMonth, billingModePerMonth:
		return true
	default:
		return false
	}
}

func isProcessedGBMeter(meter quoteMeter) bool {
	return strings.EqualFold(strings.TrimSpace(meter.unit), "1 GB")
}

func consumptionGBSecond(meters []quoteMeter) (quoteMeter, bool) {
	if _, ok := meterByKey(meters, breakdownExecutions); !ok {
		return quoteMeter{}, false
	}
	return meterByKey(meters, breakdownGBSeconds)
}

func meterByKey(meters []quoteMeter, key string) (quoteMeter, bool) {
	for _, meter := range meters {
		if meter.key == key {
			return meter, true
		}
	}
	return quoteMeter{}, false
}

func firstMatchingMeter(meters []quoteMeter, match func(quoteMeter) bool) (quoteMeter, bool) {
	for _, meter := range meters {
		if match(meter) {
			return meter, true
		}
	}
	return quoteMeter{}, false
}

func isHourMeter(meter quoteMeter) bool {
	switch strings.ToLower(strings.TrimSpace(meter.unit)) {
	case strings.ToLower(appServiceUnitHour), strings.ToLower(cosmosUnitPerHour):
		return true
	default:
		return false
	}
}

func isStorageMeter(meter quoteMeter) bool {
	return strings.EqualFold(strings.TrimSpace(meter.unit), storageUnitGBMonth)
}

func isMonthMeter(meter quoteMeter) bool {
	switch strings.ToLower(strings.TrimSpace(meter.unit)) {
	case "1/month", "1 month":
		return true
	default:
		return false
	}
}

// usagePlaceholders are the usage inputs that change a quote's total but not
// its unit rate, per resource kind, with a neutral value to quote the rate.
// Only kinds whose spec mode finfocus core does not multiply are listed:
// Cosmos request units and Functions GB-seconds or vCPU-hours. Blob, storage
// account, and SQL rates are per GB-month or per hour, which core would turn
// into a monthly total, so a missing size stays InvalidArgument for them.
// A managed disk's size is not a usage input: it picks the disk tier.
func usagePlaceholders(kind string) map[string]string {
	switch kind {
	case kindCosmosDB:
		return map[string]string{cosmosTagRUPerSecond: "100", cosmosTagRequestUnits: "1000000"}
	case kindFunctionApp:
		return map[string]string{tagExecutions: "0", tagGBSeconds: "0", tagVCPUCount: "1", tagMemoryGiB: "1"}
	default:
		return nil
	}
}

// usageOnlyMissing reports the fields a missingFieldsError names when every
// one of them is a usage input of this kind. Region, SKU, tier, and other
// identity fields keep the InvalidArgument error.
func usageOnlyMissing(kind string, err error) ([]string, bool) {
	var missing *requiredFieldsError
	if !errors.As(err, &missing) {
		return nil, false
	}
	placeholders := usagePlaceholders(kind)
	fields := missing.fields
	for _, field := range fields {
		if _, ok := placeholders[field]; !ok {
			return nil, false
		}
	}
	return fields, len(fields) > 0
}

// withUsagePlaceholders returns a copy of resource with each missing usage
// input set to its neutral value, so the quote selects the same meters.
func withUsagePlaceholders(
	resource *finfocusv1.ResourceDescriptor,
	kind string,
	missing []string,
) *finfocusv1.ResourceDescriptor {
	filled, ok := proto.Clone(resource).(*finfocusv1.ResourceDescriptor)
	if !ok {
		return resource
	}
	if filled.Tags == nil {
		filled.Tags = map[string]string{}
	}
	placeholders := usagePlaceholders(kind)
	for _, field := range missing {
		filled.Tags[field] = placeholders[field]
	}
	return filled
}

// specAssumptions states what rate_per_unit means and what it leaves out.
// finfocus core shows these in the cost estimate view.
func specAssumptions(kind string, quote monthlyQuote, spec *finfocusv1.PricingSpec, unsupplied []string) []string {
	var out []string
	if len(unsupplied) > 0 {
		out = append(out, fmt.Sprintf(
			"Usage not supplied (%s): rate_per_unit is the unit rate and no monthly total is implied",
			strings.Join(unsupplied, ", "),
		))
	}
	switch spec.GetBillingMode() {
	case billingModePerHour, billingModePerVCPUHour, billingModePerRU:
		if spec.GetUnit() != specUnitMillionRU {
			out = append(out, "Monthly cost uses 730 hours per month")
		}
	}
	out = append(out, kindAssumptions(kind, quote)...)
	return out
}

func kindAssumptions(kind string, quote monthlyQuote) []string {
	switch kind {
	case kindVM:
		if quote.spot {
			return []string{
				"Rate is one Spot instance, which Azure can evict; a scale set multiplies it by the instance count",
			}
		}
		return []string{"Rate is one on-demand instance; a scale set multiplies it by the instance count"}
	case kindDisk:
		return []string{"Rate is the monthly price of the disk tier for the requested size"}
	case kindBlob, kindStorageAccount:
		return []string{
			"Rate is the first Data Stored volume band; larger volumes bill later bands at lower rates",
			"Transactions and data transfer are not included",
		}
	case kindAppServicePlan:
		return []string{"Rate is one worker; Linux unless os=Windows; multiply by the worker count"}
	case kindFunctionApp:
		return functionAssumptions(quote)
	case kindAKS:
		return []string{"Rate is the cluster control plane; node pools are priced as virtual machines"}
	case kindSQLDatabase:
		return []string{"Compute rate covers every vCore of the SKU; storage is billed per GB-month"}
	case kindCosmosDB:
		return cosmosAssumptions(quote)
	case kindLoadBalancer:
		return loadBalancerAssumptions(quote)
	default:
		return nil
	}
}

func loadBalancerAssumptions(quote monthlyQuote) []string {
	if _, ok := meterByKey(quote.meters, loadBalancerComponentRules); ok {
		return []string{"Rate is the included rules meter, covering up to 5 rules; data processed is billed per GB"}
	}
	if _, ok := meterByKey(quote.meters, loadBalancerComponentData); ok {
		return []string{"rule_count is 0, so there is no hourly rules charge; rate is per GB of data processed"}
	}
	return []string{"rule_count is 0 and no data processed was supplied, so there is no charge"}
}

func functionAssumptions(quote monthlyQuote) []string {
	if _, ok := meterByKey(quote.meters, breakdownExecutions); ok {
		return []string{fmt.Sprintf(
			"The first %d executions and %d GB-seconds each month are free; executions bill per 10",
			functionsFreeExecutions, functionsFreeGBSeconds,
		)}
	}
	if _, ok := meterByKey(quote.meters, breakdownVCPU); ok {
		return []string{"Rate is one vCPU-hour; memory bills separately per GiB-hour"}
	}
	return []string{"Rate is one App Service plan worker"}
}

func cosmosAssumptions(quote monthlyQuote) []string {
	ru, ok := meterByKey(quote.meters, cosmosComponentRU)
	if ok && ru.specUnit == specUnitMillionRU {
		return []string{"Serverless rate is per 1,000,000 request units consumed; there is no storage meter"}
	}
	return []string{"Rate is one block of provisioned throughput per hour; storage bills per GB-month"}
}

// usageHints names the usage inputs a quote of this kind reads, so a caller
// knows which tags change the monthly total. A name the meter hints already
// use (Functions executions and gb_seconds) is not repeated.
func usageHints(
	kind string,
	quote monthlyQuote,
	existing []*finfocusv1.UsageMetricHint,
) []*finfocusv1.UsageMetricHint {
	seen := make(map[string]bool, len(existing))
	for _, hint := range existing {
		seen[hint.GetMetric()] = true
	}
	var hints []*finfocusv1.UsageMetricHint
	for _, input := range usageInputs(kind, quote) {
		if seen[input[0]] {
			continue
		}
		hints = append(hints, &finfocusv1.UsageMetricHint{Metric: input[0], Unit: input[1]})
	}
	return hints
}

func usageInputs(kind string, quote monthlyQuote) [][2]string {
	switch kind {
	case kindVM:
		if isVirtualMachineScaleSetResourceType(strings.ToLower(quote.resourceType)) {
			return [][2]string{{"instances", hintUnitCount}}
		}
	case kindDisk, kindBlob, kindStorageAccount, kindSQLDatabase:
		return [][2]string{{tagSizeGB, hintUnitGB}}
	case kindAppServicePlan:
		return [][2]string{{tagWorkerCount, hintUnitCount}}
	case kindFunctionApp:
		return functionInputs(quote)
	case kindAKS:
		return [][2]string{{"node_pool_N_count", hintUnitCount}}
	case kindCosmosDB:
		return cosmosInputs(quote)
	case kindLoadBalancer:
		return [][2]string{{"rule_count", hintUnitCount}, {"data_processed_gb", hintUnitGB}}
	}
	return nil
}

func functionInputs(quote monthlyQuote) [][2]string {
	if _, ok := meterByKey(quote.meters, breakdownExecutions); ok {
		return [][2]string{{tagExecutions, hintUnitCount}, {tagGBSeconds, "GB-second"}}
	}
	if _, ok := meterByKey(quote.meters, breakdownVCPU); ok {
		return [][2]string{{tagVCPUCount, "vCPU"}, {tagMemoryGiB, "GiB"}}
	}
	return [][2]string{{tagWorkerCount, hintUnitCount}}
}

func cosmosInputs(quote monthlyQuote) [][2]string {
	if ru, ok := meterByKey(quote.meters, cosmosComponentRU); ok && ru.specUnit == specUnitMillionRU {
		return [][2]string{{cosmosTagRequestUnits, "RU"}}
	}
	return [][2]string{{cosmosTagRUPerSecond, "RU/s"}, {tagSizeGB, hintUnitGB}}
}

func metricHints(meters []quoteMeter) []*finfocusv1.UsageMetricHint {
	hints := make([]*finfocusv1.UsageMetricHint, 0, len(meters))
	for _, meter := range meters {
		if meter.key == "" {
			continue
		}
		hints = append(hints, &finfocusv1.UsageMetricHint{
			Metric: meter.key,
			Unit:   meter.unit,
		})
	}
	return hints
}
