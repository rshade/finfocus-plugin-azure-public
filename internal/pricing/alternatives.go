package pricing

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
	"github.com/rshade/finfocus-plugin-azure-public/internal/estimation"
)

const (
	// previewAPIVersion is the Retail Prices preview that returns savingsPlan.
	previewAPIVersion = "2023-01-01-preview"

	priceModelConsumption = "Consumption"
	priceModelReservation = "Reservation"
	priceModelSavingsPlan = "SavingsPlan"
)

const (
	rankConsumption = iota
	rankSpot
	rankSavingsPlan
	rankReservation
	rankOther
)

// advisoryPrice is one alternative retail price. monthly is not part of the
// selected quote. savings_fraction is applied when the response is built,
// because projected cost compares unit prices and estimate compares months.
type advisoryPrice struct {
	category finfocusv1.FocusPricingCategory
	model    string
	term     string
	unit     float64
	monthly  float64
	upfront  float64
}

// advisoryRegion is one other region's on-demand or spot price.
type advisoryRegion struct {
	region   string
	unit     float64
	monthly  float64
	currency string
}

// vmAdvisories reads Consumption, Savings Plan, and Reservation prices for the
// VM, plus the same SKU in other regions. A failed extra query is logged and
// dropped. It does not change the selected quote.
func (c *Calculator) vmAdvisories(
	ctx context.Context,
	query azureclient.PriceQuery,
	primary []azureclient.PriceItem,
	spot bool,
	taskID string,
) ([]advisoryPrice, []advisoryRegion) {
	pages := [][]azureclient.PriceItem{primary}
	preview := query
	preview.APIVersion = previewAPIVersion
	if items := c.fetchAdvisory(ctx, preview, taskID); len(items) > 0 {
		pages = append(pages, items)
	}

	rows := consumptionAdvisories(pages)
	reservation := query
	reservation.PriceType = priceModelReservation
	rows = append(rows, reservationAdvisories(c.fetchAdvisory(ctx, reservation, taskID))...)
	rows = dedupeAdvisories(rows)

	regionQuery := query
	regionQuery.ArmRegionName = ""
	regions := regionAdvisories(
		c.fetchAdvisory(ctx, regionQuery, taskID),
		query.ArmRegionName,
		spot,
		query.CurrencyCode,
	)
	return rows, regions
}

func (c *Calculator) fetchAdvisory(
	ctx context.Context,
	query azureclient.PriceQuery,
	taskID string,
) []azureclient.PriceItem {
	result, err := c.fetchPrices(ctx, query, taskID)
	if err != nil {
		c.logger.Debug().
			Err(err).
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("price_type", query.PriceType).
			Str("api_version", query.APIVersion).
			Msg("vm advisory price query skipped")
		return nil
	}
	return result.Items
}

func consumptionAdvisories(pages [][]azureclient.PriceItem) []advisoryPrice {
	var items []azureclient.PriceItem
	for _, page := range pages {
		items = append(items, consumptionItems(page)...)
	}

	var rows []advisoryPrice
	if item, err := selectVMItem(items, false); err == nil {
		if row, ok := hourlyOption(
			item,
			finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD,
			priceModelConsumption,
			"",
		); ok {
			rows = append(rows, row)
		}
		rows = append(rows, savingsAdvisories(items)...)
	}
	if item, err := selectVMItem(items, true); err == nil {
		if row, ok := hourlyOption(
			item,
			finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC,
			vmPrioritySpot,
			"",
		); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func consumptionItems(items []azureclient.PriceItem) []azureclient.PriceItem {
	out := make([]azureclient.PriceItem, 0, len(items))
	for _, item := range items {
		if consumptionType(item.Type) {
			out = append(out, item)
		}
	}
	return out
}

func consumptionType(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.EqualFold(value, priceModelConsumption)
}

func savingsAdvisories(items []azureclient.PriceItem) []advisoryPrice {
	var rows []advisoryPrice
	for _, item := range items {
		if !vmProductAllowed(item.ProductName) || isLowPriorityVM(item) || isSpotVM(item) {
			continue
		}
		for _, plan := range item.SavingsPlan {
			hourly, ok := savingsHourly(plan)
			if !ok {
				continue
			}
			rows = append(rows, advisoryPrice{
				category: finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_COMMITTED,
				model:    priceModelSavingsPlan,
				term:     strings.TrimSpace(plan.Term),
				unit:     hourly,
				monthly:  hourly * pluginsdk.HoursPerMonth,
			})
		}
	}
	return rows
}

func savingsHourly(plan azureclient.SavingsPlanPrice) (float64, bool) {
	if strings.TrimSpace(plan.Term) == "" {
		return 0, false
	}
	hourly := plan.UnitPrice
	if hourly == 0 {
		hourly = plan.RetailPrice
	}
	if !finiteNonNegative(hourly) {
		return 0, false
	}
	return hourly, true
}

func reservationAdvisories(items []azureclient.PriceItem) []advisoryPrice {
	seen := map[string]struct{}{}
	var rows []advisoryPrice
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Type), priceModelReservation) {
			continue
		}
		if !vmProductAllowed(item.ProductName) || isLowPriorityVM(item) || isSpotVM(item) {
			continue
		}
		term := strings.TrimSpace(item.ReservationTerm)
		if _, ok := seen[term]; ok {
			continue
		}
		hourly, err := estimation.ReservationHourly(item.RetailPrice, term)
		if err != nil {
			continue
		}
		seen[term] = struct{}{}
		rows = append(rows, advisoryPrice{
			category: finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_COMMITTED,
			model:    priceModelReservation,
			term:     term,
			unit:     hourly,
			monthly:  hourly * pluginsdk.HoursPerMonth,
			upfront:  item.RetailPrice,
		})
	}
	return rows
}

func hourlyOption(
	item azureclient.PriceItem,
	category finfocusv1.FocusPricingCategory,
	model, term string,
) (advisoryPrice, bool) {
	hourly, ok := itemHourly(item)
	if !ok {
		return advisoryPrice{}, false
	}
	return advisoryPrice{
		category: category,
		model:    model,
		term:     term,
		unit:     hourly,
		monthly:  hourly * pluginsdk.HoursPerMonth,
	}, true
}

func itemHourly(item azureclient.PriceItem) (float64, bool) {
	price := item.RetailPrice
	if price == 0 {
		price = item.UnitPrice
	}
	if !finiteNonNegative(price) {
		return 0, false
	}
	return price, true
}

func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func dedupeAdvisories(rows []advisoryPrice) []advisoryPrice {
	seen := make(map[string]struct{}, len(rows))
	out := make([]advisoryPrice, 0, len(rows))
	for _, row := range rows {
		key := row.model + "|" + row.term + "|" + strconv.FormatFloat(row.unit, 'g', -1, 64)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if advisoryRank(out[i].model) != advisoryRank(out[j].model) {
			return advisoryRank(out[i].model) < advisoryRank(out[j].model)
		}
		if out[i].term != out[j].term {
			return out[i].term < out[j].term
		}
		return out[i].unit < out[j].unit
	})
	return out
}

func advisoryRank(model string) int {
	switch model {
	case priceModelConsumption:
		return rankConsumption
	case vmPrioritySpot:
		return rankSpot
	case priceModelSavingsPlan:
		return rankSavingsPlan
	case priceModelReservation:
		return rankReservation
	default:
		return rankOther
	}
}

func regionAdvisories(
	items []azureclient.PriceItem,
	requested string,
	spot bool,
	fallbackCurrency string,
) []advisoryRegion {
	pages := map[string][]azureclient.PriceItem{}
	for _, item := range items {
		region := strings.TrimSpace(item.ArmRegionName)
		if region == "" || strings.EqualFold(region, requested) {
			continue
		}
		pages[region] = append(pages[region], item)
	}
	if len(pages) == 0 {
		return nil
	}
	if spot {
		return spotRegionAdvisories(pages, fallbackCurrency)
	}
	return foundRegionAdvisories(SortRegionPrices(pages), pages, false, fallbackCurrency)
}

func spotRegionAdvisories(
	pages map[string][]azureclient.PriceItem,
	fallbackCurrency string,
) []advisoryRegion {
	sorted := make([]RegionPrice, 0, len(pages))
	for region, items := range pages {
		item, err := selectVMItem(items, true)
		if err != nil {
			sorted = append(sorted, RegionPrice{Region: region})
			continue
		}
		sorted = append(sorted, RegionPrice{Region: region, Price: item.RetailPrice, Found: true})
	}
	sortRegionPrices(sorted)
	return foundRegionAdvisories(sorted, pages, true, fallbackCurrency)
}

func foundRegionAdvisories(
	sorted []RegionPrice,
	pages map[string][]azureclient.PriceItem,
	spot bool,
	fallbackCurrency string,
) []advisoryRegion {
	out := make([]advisoryRegion, 0, len(sorted))
	for _, row := range sorted {
		if !row.Found {
			continue
		}
		item, err := selectVMItem(pages[row.Region], spot)
		if err != nil {
			continue
		}
		unit, ok := itemHourly(item)
		if !ok {
			continue
		}
		currency := strings.TrimSpace(item.CurrencyCode)
		if currency == "" {
			currency = strings.TrimSpace(fallbackCurrency)
		}
		if currency == "" {
			continue
		}
		out = append(out, advisoryRegion{
			region:   row.Region,
			unit:     unit,
			monthly:  unit * pluginsdk.HoursPerMonth,
			currency: currency,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// priceOptionProtos sets savings_fraction from unit prices, or from monthly
// costs when monthlyBasis is set. primary 0 yields fraction 0. A row that
// cannot be compared is omitted.
func priceOptionProtos(
	rows []advisoryPrice,
	primaryUnit, primaryMonthly float64,
	monthlyBasis bool,
) []*finfocusv1.PriceOption {
	if len(rows) == 0 {
		return nil
	}
	out := make([]*finfocusv1.PriceOption, 0, len(rows))
	for _, row := range rows {
		primary := primaryUnit
		other := row.unit
		if monthlyBasis {
			primary = primaryMonthly
			other = row.monthly
		}
		fraction, ok := savingsOrZero(primary, other)
		if !ok {
			continue
		}
		out = append(out, &finfocusv1.PriceOption{
			Category:        row.category,
			Model:           row.model,
			Term:            row.term,
			UnitPrice:       row.unit,
			MonthlyCost:     row.monthly,
			UpfrontCost:     row.upfront,
			SavingsFraction: fraction,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func savingsOrZero(primary, other float64) (float64, bool) {
	if !finiteNonNegative(other) {
		return 0, false
	}
	if primary == 0 {
		return 0, finiteNonNegative(primary)
	}
	if !finiteNonNegative(primary) {
		return 0, false
	}
	fraction, err := estimation.SavingsFraction(primary, other)
	if err != nil {
		return 0, false
	}
	return fraction, true
}

func regionPriceProtos(rows []advisoryRegion) []*finfocusv1.RegionPrice {
	if len(rows) == 0 {
		return nil
	}
	out := make([]*finfocusv1.RegionPrice, 0, len(rows))
	for _, row := range rows {
		out = append(out, &finfocusv1.RegionPrice{
			Region:      row.region,
			UnitPrice:   row.unit,
			MonthlyCost: row.monthly,
			Currency:    row.currency,
		})
	}
	return out
}

func newEstimateResponse(
	currency string,
	monthly float64,
	category finfocusv1.FocusPricingCategory,
	rows []advisoryPrice,
	regions []advisoryRegion,
) (*finfocusv1.EstimateCostResponse, error) {
	opts := []pluginsdk.EstimateCostResponseOption{
		pluginsdk.WithEstimateCost(currency, monthly),
		pluginsdk.WithPricingCategory(category),
	}
	if priceOpts := priceOptionProtos(rows, 0, monthly, true); len(priceOpts) > 0 {
		opts = append(opts, pluginsdk.WithEstimatePriceOptions(priceOpts...))
	}
	if regionOpts := regionPriceProtos(regions); len(regionOpts) > 0 {
		opts = append(opts, pluginsdk.WithEstimateCostRegionPrices(regionOpts...))
	}
	resp := pluginsdk.NewEstimateCostResponse(opts...)
	if err := pluginsdk.ValidateEstimateCostResponse(resp); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid estimate cost response: %v", err)
	}
	return resp, nil
}
