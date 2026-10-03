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
	kindSQLDatabase = "sqldatabase"

	sqlServiceName         = "SQL Database"
	sqlDatabaseSegment     = "sql/database"
	canonicalSQLDatabase   = "sql/Database"
	sqlComputeProduct      = "SQL Database Single/Elastic Pool General Purpose - Compute Gen5"
	sqlStorageProduct      = "SQL Database Single/Elastic Pool General Purpose - Storage"
	sqlHardwareGen5        = "Gen5"
	sqlMeterVCore          = "vCore"
	sqlMeterZoneVCore      = "Zone Redundancy vCore"
	sqlMeterDataStored     = "General Purpose Data Stored"
	sqlMeterZoneDataStored = "General Purpose Zone Redundancy Data Stored"
	sqlStorageSKUName      = "General Purpose"
	sqlZoneStorageSKUName  = "General Purpose Zone Redundancy"
	sqlUnitHour            = "1 Hour"
	sqlUnitGBMonth         = "1 GB/Month"
	sqlTagTier             = "tier"
	sqlTagHardware         = "hardware"
	sqlTagVCores           = "vcores"
	sqlTagZone             = "zone_redundant"
	sqlTagZonePulumi       = "zoneRedundant"
	sqlZoneTrue            = "true"
	sqlTierGP              = "gp"
	sqlTierBC              = "bc"
	sqlTierHS              = "hs"
	sqlServerlessMark      = "s"
	sqlModelDTU            = "DTU"
	sqlModelServerless     = "serverless"
	sqlModelBusinessCrit   = "Business Critical"
	sqlModelHyperscale     = "Hyperscale"
	sqlComponentZoneCPU    = "zone_redundancy_compute"
	sqlTaskAZ27            = "AZ-2.7"
	sqlSKUMinParts         = 3
)

// quoteSQLDatabase prices General Purpose Gen5 provisioned vCore compute plus
// General Purpose storage. The {n} vCore row is already the price for n
// vCores. Storage is GB-month. When zone_redundant is true, the zone vCore
// surcharge is added to compute and storage is billed at the zone rate instead
// of the local rate. Other purchasing models are Unimplemented and name AZ-2.7.
func (c *Calculator) quoteSQLDatabase(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	spec, err := sqlRequestFrom(resource)
	if err != nil {
		return monthlyQuote{}, err
	}

	computeResult, err := c.fetchPrices(ctx, sqlPriceQuery(resource, spec.region, sqlComputeProduct), taskID)
	if err != nil {
		return monthlyQuote{}, err
	}
	storageResult, err := c.fetchPrices(ctx, sqlPriceQuery(resource, spec.region, sqlStorageProduct), taskID)
	if err != nil {
		return monthlyQuote{}, err
	}
	return sqlMonthlyQuote(resource, spec, computeResult, storageResult)
}

type sqlRequest struct {
	region string
	sizeGB float64
	vcores int
	sku    string
	zone   bool
}

func sqlRequestFrom(resource *finfocusv1.ResourceDescriptor) (sqlRequest, error) {
	region := descriptorRegion(resource)
	sizeGB, sizeSet, err := descriptorSizeGB(resource)
	if err != nil {
		return sqlRequest{}, status.Error(codes.InvalidArgument, err.Error())
	}
	sku, err := sqlDatabaseSKU(resource)
	if err != nil {
		return sqlRequest{}, err
	}
	if missing := missingSQLQuoteFields(region, sizeSet, sku, resource.GetTags()); len(missing) > 0 {
		return sqlRequest{}, missingFieldsError(missing)
	}

	spec, err := parseSQLModel(sku, resource.GetTags())
	if err != nil {
		return sqlRequest{}, err
	}
	spec.region = region
	spec.sizeGB = sizeGB
	spec.zone = strings.EqualFold(pulumiTag(resource.GetTags(), sqlTagZone, sqlTagZonePulumi), sqlZoneTrue)
	return spec, nil
}

func missingSQLQuoteFields(region string, sizeSet bool, sku string, tags map[string]string) []string {
	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if !sizeSet {
		missing = append(missing, "size_gb")
	}
	if sku == "" {
		missing = append(missing, sqlIdentityMissing(tags)...)
	}
	return missing
}

func sqlIdentityMissing(tags map[string]string) []string {
	var missing []string
	if firstNonEmptyTag(tags, sqlTagTier) == "" {
		missing = append(missing, sqlTagTier)
	}
	if firstNonEmptyTag(tags, sqlTagHardware) == "" {
		missing = append(missing, sqlTagHardware)
	}
	if firstNonEmptyTag(tags, sqlTagVCores) == "" {
		missing = append(missing, sqlTagVCores)
	}
	return missing
}

func parseSQLModel(sku string, tags map[string]string) (sqlRequest, error) {
	if sku != "" {
		return parseSQLSKU(sku)
	}
	return parseSQLTags(tags)
}

func parseSQLSKU(raw string) (sqlRequest, error) {
	parts := strings.Split(strings.TrimSpace(raw), "_")
	if len(parts) < sqlSKUMinParts {
		return sqlRequest{}, sqlRefused(sqlModelDTU, raw)
	}
	vcores, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return sqlRequest{}, sqlRefused(sqlModelDTU, raw)
	}
	if vcores < 1 {
		return sqlRequest{}, status.Errorf(codes.InvalidArgument, "invalid %s %q", sqlTagVCores, parts[len(parts)-1])
	}
	tier, hardware, serverless := splitSQLSKU(parts)
	if familyErr := refuseSQLFamily(tier, hardware, serverless, raw); familyErr != nil {
		return sqlRequest{}, familyErr
	}
	return sqlRequest{vcores: vcores, sku: sqlCanonicalSKU(vcores)}, nil
}

func parseSQLTags(tags map[string]string) (sqlRequest, error) {
	tier := firstNonEmptyTag(tags, sqlTagTier)
	hardware := firstNonEmptyTag(tags, sqlTagHardware)
	rawVCores := firstNonEmptyTag(tags, sqlTagVCores)
	vcores, err := strconv.Atoi(strings.TrimSpace(rawVCores))
	if err != nil || vcores < 1 {
		return sqlRequest{}, status.Errorf(codes.InvalidArgument, "invalid %s %q", sqlTagVCores, rawVCores)
	}
	label := tier + " " + hardware
	if familyErr := refuseSQLFamily(tier, hardware, false, label); familyErr != nil {
		return sqlRequest{}, familyErr
	}
	return sqlRequest{vcores: vcores, sku: sqlCanonicalSKU(vcores)}, nil
}

func splitSQLSKU(parts []string) (string, string, bool) {
	middle := parts[1 : len(parts)-1]
	serverless := false
	if strings.EqualFold(middle[0], sqlServerlessMark) {
		serverless = true
		middle = middle[1:]
	}
	hardware := ""
	if len(middle) == 1 {
		hardware = middle[0]
	}
	return parts[0], hardware, serverless
}

func refuseSQLFamily(tier, hardware string, serverless bool, raw string) error {
	switch canonicalTierToken(tier) {
	case sqlTierBC:
		return sqlRefused(sqlModelBusinessCrit, raw)
	case sqlTierHS:
		return sqlRefused(sqlModelHyperscale, raw)
	case sqlTierGP:
		if serverless {
			return sqlRefused(sqlModelServerless, raw)
		}
		if strings.EqualFold(hardware, sqlHardwareGen5) {
			return nil
		}
		if hardware == "" {
			return sqlRefused(sqlModelDTU, raw)
		}
		return sqlRefused(hardware, raw)
	default:
		return sqlRefused(sqlModelDTU, raw)
	}
}

func sqlCanonicalSKU(vcores int) string {
	return fmt.Sprintf("GP_Gen5_%d", vcores)
}

func sqlVCoreSKU(vcores int) string {
	return strconv.Itoa(vcores) + " " + sqlMeterVCore
}

func sqlZoneVCoreSKU(vcores int) string {
	return strconv.Itoa(vcores) + " " + sqlMeterVCore + " Zone Redundancy"
}

func canonicalTierToken(tier string) string {
	switch compactSQLToken(tier) {
	case sqlTierGP, "generalpurpose":
		return sqlTierGP
	case sqlTierBC, "businesscritical":
		return sqlTierBC
	case sqlTierHS, "hyperscale":
		return sqlTierHS
	default:
		return compactSQLToken(tier)
	}
}

func compactSQLToken(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if r == ' ' || r == '_' || r == '-' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func sqlRefused(model, sku string) error {
	return status.Errorf(
		codes.Unimplemented,
		"unsupported SQL Database model %s (%s): %s",
		model,
		sku,
		sqlTaskAZ27,
	)
}

func sqlPriceQuery(resource *finfocusv1.ResourceDescriptor, region, product string) azureclient.PriceQuery {
	return azureclient.PriceQuery{
		ArmRegionName: region,
		ServiceName:   sqlServiceName,
		ProductName:   product,
		CurrencyCode:  descriptorCurrency(resource),
	}
}

func sqlMonthlyQuote(
	resource *finfocusv1.ResourceDescriptor,
	spec sqlRequest,
	compute, storage azureclient.CachedResult,
) (monthlyQuote, error) {
	computeItem, err := selectSQLPrice(
		compute.Items, sqlComputeProduct, sqlVCoreSKU(spec.vcores), sqlMeterVCore, sqlUnitHour,
	)
	if err != nil {
		return monthlyQuote{}, sqlStatus(err)
	}
	storageSKU, storageMeter := sqlStorageRow(spec.zone)
	storageItem, storageErr := selectSQLPrice(
		storage.Items, sqlStorageProduct, storageSKU, storageMeter, sqlUnitGBMonth,
	)
	if storageErr != nil {
		return monthlyQuote{}, sqlStatus(storageErr)
	}
	currency := itemCurrency(computeItem)
	if currencyErr := sqlSameCurrency(currency, storageItem); currencyErr != nil {
		return monthlyQuote{}, currencyErr
	}

	components := map[string]float64{
		breakdownCompute: computeItem.RetailPrice * pluginsdk.HoursPerMonth,
		breakdownStorage: storageItem.RetailPrice * spec.sizeGB,
	}
	meters := []quoteMeter{
		{key: breakdownCompute, price: computeItem.RetailPrice, unit: computeItem.UnitOfMeasure},
		{key: breakdownStorage, price: storageItem.RetailPrice, unit: storageItem.UnitOfMeasure},
	}
	if spec.zone {
		zoneCompute, zoneErr := addSQLZoneCompute(components, compute.Items, spec, currency)
		if zoneErr != nil {
			return monthlyQuote{}, zoneErr
		}
		meters = append(meters,
			quoteMeter{key: sqlComponentZoneCPU, price: zoneCompute.RetailPrice, unit: zoneCompute.UnitOfMeasure},
		)
	}

	return monthlyQuote{
		unitPrice:     computeItem.RetailPrice,
		monthly:       sqlComponentSum(components),
		currency:      currency,
		billingDetail: sqlBillingDetail(spec.sku, spec.sizeGB, spec.region, spec.zone),
		components:    components,
		expiresAt:     earlierTime(compute.ExpiresAt, storage.ExpiresAt),
		region:        spec.region,
		sku:           spec.sku,
		resourceType:  resource.GetResourceType(),
		meters:        meters,
	}, nil
}

// sqlStorageRow returns the storage sku and meter. Zone-redundant storage is
// a replacement rate, not a surcharge: Azure bills the zone row instead of the
// local row (issue #77).
func sqlStorageRow(zone bool) (string, string) {
	if zone {
		return sqlZoneStorageSKUName, sqlMeterZoneDataStored
	}
	return sqlStorageSKUName, sqlMeterDataStored
}

func addSQLZoneCompute(
	components map[string]float64,
	compute []azureclient.PriceItem,
	spec sqlRequest,
	currency string,
) (azureclient.PriceItem, error) {
	computeItem, err := selectSQLPrice(
		compute, sqlComputeProduct, sqlZoneVCoreSKU(spec.vcores), sqlMeterZoneVCore, sqlUnitHour,
	)
	if err != nil {
		return azureclient.PriceItem{}, sqlStatus(err)
	}
	if currencyErr := sqlSameCurrency(currency, computeItem); currencyErr != nil {
		return azureclient.PriceItem{}, currencyErr
	}
	components[sqlComponentZoneCPU] = computeItem.RetailPrice * pluginsdk.HoursPerMonth
	return computeItem, nil
}

func sqlComponentSum(components map[string]float64) float64 {
	return components[breakdownCompute] +
		components[breakdownStorage] +
		components[sqlComponentZoneCPU]
}

func selectSQLPrice(items []azureclient.PriceItem, product, sku, meter, unit string) (azureclient.PriceItem, error) {
	var matched []azureclient.PriceItem
	for i := range items {
		item := items[i]
		if item.ProductName != product || item.SkuName != sku || item.MeterName != meter {
			continue
		}
		if item.Type != storagePriceTypeConsumption || item.UnitOfMeasure != unit {
			continue
		}
		matched = append(matched, item)
	}
	if len(matched) == 0 {
		return azureclient.PriceItem{}, fmt.Errorf(
			"no sql price for sku %s meter %s: %w",
			sku,
			meter,
			azureclient.ErrNotFound,
		)
	}
	return sameSQLPrice(matched, sku, meter)
}

func sameSQLPrice(items []azureclient.PriceItem, sku, meter string) (azureclient.PriceItem, error) {
	chosen := items[0]
	currency := itemCurrency(chosen)
	for _, item := range items[1:] {
		if item.RetailPrice != chosen.RetailPrice || itemCurrency(item) != currency {
			return azureclient.PriceItem{}, status.Errorf(
				codes.InvalidArgument,
				"ambiguous sql price for sku %s meter %s",
				sku,
				meter,
			)
		}
	}
	return chosen, nil
}

func sqlSameCurrency(currency string, items ...azureclient.PriceItem) error {
	for _, item := range items {
		if itemCurrency(item) != currency {
			return status.Error(codes.InvalidArgument, "sql meters use different currencies")
		}
	}
	return nil
}

func sqlBillingDetail(sku string, sizeGB float64, region string, zone bool) string {
	detail := fmt.Sprintf("SQL Database %s %g GB in %s", sku, sizeGB, region)
	if zone {
		return detail + ", zone redundant"
	}
	return detail
}

func sqlStatus(err error) error {
	if err == nil || status.Code(err) != codes.Unknown {
		return err
	}
	return MapToGRPCStatus(err).Err()
}

// isSQLDatabaseResourceType reports whether lower contains the sql/database
// segment, including Pulumi azure:sql/database:Database. A longer prefix such
// as sql/databaseextra does not match, and neither does mysql/database.
func isSQLDatabaseResourceType(lower string) bool {
	return boundedSegment(lower, sqlDatabaseSegment) ||
		boundedSegment(lower, "mssql/database") ||
		tokenSuffix(lower, "sql", "database")
}

func boundedSegment(lower, segment string) bool {
	idx := strings.Index(lower, segment)
	if idx < 0 {
		return false
	}
	if idx > 0 && !sqlSegmentSep(lower[idx-1]) {
		return false
	}
	return resourceSegment(lower, segment)
}

func sqlSegmentSep(b byte) bool {
	return b == ':' || b == '/' || b == ' '
}
