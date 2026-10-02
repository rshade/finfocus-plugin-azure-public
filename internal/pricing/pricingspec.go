package pricing

import (
	"context"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

const (
	pricingSpecTask       = "AZ-2.11"
	billingModePerHour    = "per_hour"
	billingModePerGBMonth = "per_gb_month"
	billingModePerMonth   = "per_month"
	billingModePerSecond  = "per_second"
	billingModePerRU      = "per_ru"
	specUnitHourName      = "hour"
	specUnitGBMonthName   = "GB-month"
	specUnitMonthName     = "Month"
	providerAzure         = "azure"
	providerAzureNative   = "azure-native"
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

	quote, err := c.quoteResource(ctx, req.GetResource(), pricingSpecTask)
	if err != nil {
		log.Warn().Str("result_status", "error").Err(err).Msg("GetPricingSpec pricing failed")
		return nil, err
	}

	spec := pricingSpecFromQuote(quote)
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

func specRate(meters []quoteMeter) (string, string, float64) {
	if gb, ok := consumptionGBSecond(meters); ok {
		return billingModePerSecond, gb.unit, gb.price
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
	if len(meters) > 0 {
		return billingModePerRU, meters[0].unit, meters[0].price
	}
	return billingModePerHour, specUnitHourName, 0
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
