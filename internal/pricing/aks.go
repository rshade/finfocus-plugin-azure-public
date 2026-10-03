package pricing

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	kindAKS = "aks"

	aksServiceName             = "Azure Kubernetes Service"
	aksResourceSegment         = "containerservice/kubernetescluster"
	canonicalKubernetesCluster = "containerservice/KubernetesCluster"

	aksMeterStandard = "Standard Uptime SLA"
	aksMeterLTS      = "Standard Long Term Support"
	aksMeterFree     = "FreeTierInfrastructureCost Uptime SLA"

	// aksFreeTierNote explains the zero control plane. The Retail Prices API
	// lists the Free meter at 0.05 USD per hour from 2026-10-01, while the
	// published AKS pricing page and the Pricing Calculator still show the Free
	// tier as having no control-plane charge.
	aksFreeTierNote = "Free control plane priced at 0: the published AKS pricing page lists no " +
		"Free-tier charge, so the retail meter " + aksMeterFree + " is not billed"

	aksTierFree     = "free"
	aksTierStandard = "standard"
	aksTierTag      = "tier"
	aksSupportTag   = "support"
	aksSupportLTS   = "lts"

	aksLabelFree     = "Free"
	aksLabelStandard = "Standard"
	aksLabelLTS      = "Standard Long Term Support"

	aksComponentControlPlane = "control_plane"
	aksNodePoolKeyPrefix     = "node_pool_"
	aksBreakdownKeyMaxLen    = 64
)

// quoteAKS prices the control plane from the AKS retail page and each node
// pool as one on-demand VM times the pool count. unitPrice stays the control
// plane hourly retailPrice.
func (c *Calculator) quoteAKS(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	region := descriptorRegion(resource)
	if region == "" {
		return monthlyQuote{}, missingFieldsError([]string{missingFieldRegion})
	}
	meter, tierLabel, err := aksControlPlane(resource)
	if err != nil {
		return monthlyQuote{}, err
	}
	pools, err := aksNodePools(resource.GetTags())
	if err != nil {
		return monthlyQuote{}, err
	}

	if tierLabel == aksLabelFree {
		return c.aksQuote(ctx, resource, taskID, aksQuoteInput{
			region:    region,
			tierLabel: tierLabel,
			item: azureclient.PriceItem{
				CurrencyCode:  descriptorCurrency(resource),
				UnitOfMeasure: appServiceUnitHour,
			},
			pools: pools,
		})
	}

	result, err := c.fetchServicePrices(ctx, resource, aksServiceName, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}
	item, err := selectAKSControlPlane(result.Items, meter)
	if err != nil {
		return monthlyQuote{}, aksStatus(err)
	}
	return c.aksQuote(ctx, resource, taskID, aksQuoteInput{
		region:    region,
		tierLabel: tierLabel,
		item:      item,
		expiresAt: result.ExpiresAt,
		pools:     pools,
	})
}

type aksQuoteInput struct {
	region    string
	tierLabel string
	item      azureclient.PriceItem
	expiresAt time.Time
	pools     []aksPool
}

func (c *Calculator) aksQuote(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
	in aksQuoteInput,
) (monthlyQuote, error) {
	controlMonthly := in.item.RetailPrice * pluginsdk.HoursPerMonth
	components := map[string]float64{aksComponentControlPlane: controlMonthly}
	meters := []quoteMeter{{
		key:   aksComponentControlPlane,
		price: in.item.RetailPrice,
		unit:  in.item.UnitOfMeasure,
	}}
	total := controlMonthly
	expires := in.expiresAt
	currency := itemCurrency(in.item)

	for _, pool := range in.pools {
		cost, poolExpires, meter, err := c.aksPoolCost(ctx, resource, taskID, in.region, currency, pool)
		if err != nil {
			return monthlyQuote{}, err
		}
		key, err := aksComponentKey(pool.name)
		if err != nil {
			return monthlyQuote{}, err
		}
		if _, exists := components[key]; exists {
			return monthlyQuote{}, status.Errorf(codes.InvalidArgument, "duplicate node pool %q", key)
		}
		components[key] = cost
		meter.key = key
		meters = append(meters, meter)
		total += cost
		expires = earlierTime(expires, poolExpires)
	}

	return monthlyQuote{
		unitPrice:     in.item.RetailPrice,
		monthly:       total,
		currency:      currency,
		billingDetail: aksBillingDetail(in.tierLabel, in.region, len(in.pools)),
		components:    components,
		expiresAt:     expires,
		region:        in.region,
		sku:           in.tierLabel,
		resourceType:  resource.GetResourceType(),
		meters:        meters,
	}, nil
}

func (c *Calculator) aksPoolCost(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID, region, currency string,
	pool aksPool,
) (float64, time.Time, quoteMeter, error) {
	quote, err := c.quoteVM(ctx, aksNodeResource(resource, region, pool.sku), taskID)
	if err != nil {
		return 0, time.Time{}, quoteMeter{}, err
	}
	if quote.currency != currency {
		return 0, time.Time{}, quoteMeter{}, status.Error(
			codes.InvalidArgument,
			"aks node currency does not match the control plane",
		)
	}
	meter := quoteMeter{price: quote.unitPrice, unit: appServiceUnitHour}
	if len(quote.meters) > 0 {
		meter = quote.meters[0]
		meter.key = ""
	}
	return quote.monthly * float64(pool.count), quote.expiresAt, meter, nil
}

// aksControlPlane reads the tier from SKU, Tags["sku"], or Tags["tier"].
// support=lts selects Standard Long Term Support. That support value with
// Free is InvalidArgument. Any other tier, including Automatic, is rejected.
// Free returns its meter name for the note only; quoteAKS prices it at 0.
func aksControlPlane(resource *finfocusv1.ResourceDescriptor) (string, string, error) {
	raw := strings.TrimSpace(resource.GetSku())
	if raw == "" {
		raw = firstNonEmptyTag(resource.GetTags(), "sku", aksTierTag)
	}
	if raw == "" {
		return "", "", missingFieldsError([]string{aksTierTag})
	}

	lts := strings.EqualFold(strings.TrimSpace(resource.GetTags()[aksSupportTag]), aksSupportLTS)
	switch strings.ToLower(raw) {
	case aksTierFree:
		if lts {
			return "", "", status.Errorf(
				codes.InvalidArgument,
				"support %q is invalid for tier %q",
				resource.GetTags()[aksSupportTag],
				raw,
			)
		}
		return aksMeterFree, aksLabelFree, nil
	case aksTierStandard:
		if lts {
			return aksMeterLTS, aksLabelLTS, nil
		}
		return aksMeterStandard, aksLabelStandard, nil
	default:
		return "", "", status.Errorf(codes.InvalidArgument, "unsupported tier %q", raw)
	}
}

// selectAKSControlPlane keeps the open Consumption row for meter.
// Product is Azure Kubernetes Service, unit 1 Hour. An empty EffectiveEndDate
// is open. Closed rows are dropped. Differing open prices are an error that
// names the meter. Prices are not averaged.
func selectAKSControlPlane(items []azureclient.PriceItem, meter string) (azureclient.PriceItem, error) {
	matched := aksMeterRows(items, meter)
	if len(matched) == 0 {
		return azureclient.PriceItem{}, fmt.Errorf(
			"no aks price for meter %s: %w",
			meter,
			azureclient.ErrNotFound,
		)
	}
	open := aksOpenRows(matched)
	if len(open) == 0 {
		return azureclient.PriceItem{}, status.Errorf(
			codes.InvalidArgument,
			"no open price for meter %s",
			meter,
		)
	}
	return aksSameOpenPrice(open, meter)
}

func aksMeterRows(items []azureclient.PriceItem, meter string) []azureclient.PriceItem {
	matched := make([]azureclient.PriceItem, 0, len(items))
	for i := range items {
		item := items[i]
		if item.ProductName != aksServiceName || item.MeterName != meter {
			continue
		}
		if item.UnitOfMeasure != appServiceUnitHour || item.Type != storagePriceTypeConsumption {
			continue
		}
		matched = append(matched, item)
	}
	return matched
}

func aksOpenRows(items []azureclient.PriceItem) []azureclient.PriceItem {
	open := make([]azureclient.PriceItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.EffectiveEndDate) == "" {
			open = append(open, item)
		}
	}
	return open
}

func aksSameOpenPrice(items []azureclient.PriceItem, meter string) (azureclient.PriceItem, error) {
	chosen := items[0]
	currency := itemCurrency(chosen)
	for _, item := range items[1:] {
		if item.RetailPrice != chosen.RetailPrice || itemCurrency(item) != currency {
			return azureclient.PriceItem{}, status.Errorf(
				codes.InvalidArgument,
				"ambiguous price for meter %s",
				meter,
			)
		}
	}
	return chosen, nil
}

type aksPool struct {
	name  string
	sku   string
	count int
}

func aksNodePools(tags map[string]string) ([]aksPool, error) {
	maxIndex := 0
	for key, value := range tags {
		if strings.TrimSpace(value) == "" {
			continue
		}
		index, ok := aksPoolIndex(key)
		if ok && index > maxIndex {
			maxIndex = index
		}
	}

	pools := make([]aksPool, 0, maxIndex)
	for index := 1; index <= maxIndex; index++ {
		pool, present, err := aksPoolAt(tags, index)
		if err != nil {
			return nil, err
		}
		if !present {
			return nil, status.Errorf(
				codes.InvalidArgument,
				"node pool gap: node_pool_%d without node_pool_%d",
				maxIndex,
				index,
			)
		}
		pools = append(pools, pool)
	}
	return pools, nil
}

func aksPoolAt(tags map[string]string, index int) (aksPool, bool, error) {
	skuKey := aksPoolTag(index, "sku")
	countKey := aksPoolTag(index, "count")
	nameKey := aksPoolTag(index, "name")
	sku := strings.TrimSpace(tags[skuKey])
	countRaw := strings.TrimSpace(tags[countKey])
	name := strings.TrimSpace(tags[nameKey])
	if sku == "" && countRaw == "" && name == "" {
		return aksPool{}, false, nil
	}
	if sku == "" {
		return aksPool{}, true, status.Errorf(codes.InvalidArgument, "missing %s", skuKey)
	}
	count, err := aksPoolCount(countKey, countRaw)
	if err != nil {
		return aksPool{}, true, err
	}
	if name == "" {
		name = fmt.Sprintf("pool_%d", index)
	}
	return aksPool{name: name, sku: sku, count: count}, true, nil
}

func aksPoolCount(countKey, countRaw string) (int, error) {
	if countRaw == "" {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s", countKey)
	}
	count, err := strconv.Atoi(countRaw)
	if err != nil || count < 1 {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s %q", countKey, countRaw)
	}
	return count, nil
}

func aksPoolTag(index int, suffix string) string {
	return fmt.Sprintf("node_pool_%d_%s", index, suffix)
}

func aksPoolIndex(key string) (int, bool) {
	rest, ok := strings.CutPrefix(key, aksNodePoolKeyPrefix)
	if !ok {
		return 0, false
	}
	number, suffix, ok := strings.Cut(rest, "_")
	if !ok || !aksPoolSuffix(suffix) {
		return 0, false
	}
	index, err := strconv.Atoi(number)
	if err != nil || index < 1 {
		return 0, false
	}
	return index, true
}

func aksPoolSuffix(suffix string) bool {
	switch suffix {
	case "sku", "count", "name":
		return true
	default:
		return false
	}
}

func aksNodeResource(resource *finfocusv1.ResourceDescriptor, region, sku string) *finfocusv1.ResourceDescriptor {
	node := &finfocusv1.ResourceDescriptor{
		Region: region,
		Sku:    sku,
	}
	if currency := firstNonEmptyTag(resource.GetTags(), "currency", "currencyCode"); currency != "" {
		node.Tags = map[string]string{"currency": currency}
	}
	return node
}

func aksComponentKey(name string) (string, error) {
	snake := aksSnake(name)
	key := aksNodePoolKeyPrefix + snake
	if snake == "" || len(key) > aksBreakdownKeyMaxLen {
		return "", status.Errorf(codes.InvalidArgument, "invalid node pool name %q", name)
	}
	return key, nil
}

func aksSnake(name string) string {
	var b strings.Builder
	underscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			underscore = false
			continue
		}
		if b.Len() == 0 || underscore {
			continue
		}
		b.WriteByte('_')
		underscore = true
	}
	return strings.TrimSuffix(b.String(), "_")
}

func aksBillingDetail(tier, region string, pools int) string {
	subject := "control plane"
	if pools > 0 {
		subject = "control plane and node pools"
	}
	detail := fmt.Sprintf("AKS %s %s in %s, 730 hrs/month", tier, subject, region)
	if tier == aksLabelFree {
		detail += "; " + aksFreeTierNote
	}
	return detail
}

func earlierTime(left, right time.Time) time.Time {
	if left.IsZero() || (!right.IsZero() && right.Before(left)) {
		return right
	}
	return left
}

func aksStatus(err error) error {
	if err == nil || status.Code(err) != codes.Unknown {
		return err
	}
	return MapToGRPCStatus(err).Err()
}

func isAKSResourceType(lower string) bool {
	return resourceSegment(lower, aksResourceSegment) ||
		tokenSuffix(lower, "containerservice", "managedcluster")
}
