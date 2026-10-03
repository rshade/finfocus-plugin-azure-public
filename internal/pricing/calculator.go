package pricing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

const defaultServiceName = "Virtual Machines"

// Calculator implements finfocus.v1.CostSourceServiceServer.
type Calculator struct {
	finfocusv1.UnimplementedCostSourceServiceServer

	logger           zerolog.Logger
	cachedClient     *azureclient.CachedClient
	billingAccountID string
}

// NewCalculator creates a new instance of Calculator with the provided logger.
func NewCalculator(logger zerolog.Logger, cachedClient ...*azureclient.CachedClient) *Calculator {
	var cc *azureclient.CachedClient
	if len(cachedClient) > 0 {
		cc = cachedClient[0]
	}

	return &Calculator{
		logger:       logger,
		cachedClient: cc,
	}
}

// SetBillingAccountID stores the process FOCUS billing account id.
// GetActualCost uses GetActualCostRequest.billing_account_id when that
// field is set. This value is the fallback. An empty id leaves FocusRecord
// nil. The value is not read from Azure and is not invented.
func (c *Calculator) SetBillingAccountID(id string) {
	if c == nil {
		return
	}
	c.billingAccountID = strings.TrimSpace(id)
}

// Name returns the name of the plugin for the SDK.
// The SDK wraps this and provides the gRPC Name RPC implementation.
func (c *Calculator) Name() string {
	return PluginInfo().Name
}

// pluginVersion is the version GetPluginInfo reports and the manifest files
// carry. Release Please does not bump it (no extra-files entry).
const pluginVersion = "0.1.0"

const (
	pluginMetadataType      = "type"
	pluginTypePublicPricing = "public-pricing-fallback"
)

// PluginCapabilities lists the RPCs this plugin really serves. It is explicit
// because Calculator embeds UnimplementedCostSourceServiceServer, which makes
// the SDK's interface inference report every optional capability, including
// BATCH_COST and RESOLVE_RESOURCE_TYPES that finfocus core then calls.
func PluginCapabilities() []finfocusv1.PluginCapability {
	return []finfocusv1.PluginCapability{
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_ESTIMATE_COST,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN,
	}
}

// PluginInfo is the plugin's identity and explicit capabilities. GetPluginInfo
// serves it, and the binary passes it as pluginsdk.ServeConfig.PluginInfo,
// whose Capabilities feed the SDK's Supports capabilities_enum.
func PluginInfo() *pluginsdk.PluginInfo {
	return &pluginsdk.PluginInfo{
		Name:         "azure-public",
		Version:      pluginVersion,
		SpecVersion:  pluginsdk.SpecVersion,
		Providers:    []string{providerAzure, providerAzureNative},
		Metadata:     map[string]string{pluginMetadataType: pluginTypePublicPricing},
		Capabilities: PluginCapabilities(),
	}
}

// GetPluginInfo returns metadata about the plugin including name, version,
// spec version, supported cloud providers, and its explicit capabilities.
// The SDK server adds the legacy supports_* metadata keys from Capabilities.
func (c *Calculator) GetPluginInfo(
	ctx context.Context,
	_ *finfocusv1.GetPluginInfoRequest,
) (*finfocusv1.GetPluginInfoResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetPluginInfo request")

	info := PluginInfo()
	return &finfocusv1.GetPluginInfoResponse{
		Name:         info.Name,
		Version:      info.Version,
		SpecVersion:  info.SpecVersion,
		Providers:    info.Providers,
		Metadata:     info.Metadata,
		Capabilities: info.Capabilities,
	}, nil
}

// Supports checks if this plugin supports a given resource type by attempting
// to map the resource descriptor to an Azure pricing query. Returns
// Supported:true if the mapping succeeds, or Supported:false with a reason
// describing why the resource cannot be priced.
// A Virtual Machine with tag priority=Spot is supported. Tag
// pricing_model=spot is the same choice when priority is empty. The quote
// uses the Linux Spot meter and pricing category Dynamic. Any other
// priority or pricing_model value is not.
func (c *Calculator) Supports(
	ctx context.Context,
	req *finfocusv1.SupportsRequest,
) (*finfocusv1.SupportsResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling Supports request")

	_, err := MapDescriptorToQuery(req.GetResource())
	if err != nil {
		log.Debug().Err(err).Msg("resource not supported")
		return &finfocusv1.SupportsResponse{
			Supported: false,
			Reason:    err.Error(),
		}, nil
	}

	return &finfocusv1.SupportsResponse{
		Supported: true,
	}, nil
}

// EstimateCost estimates monthly cost from Azure Retail Prices data.
// Virtual machines and managed disks keep their attribute parsers.
// Every other mapped type uses the same quote as GetProjectedCost.
// The request must contain the appropriate attributes for the resource type.
// Returns InvalidArgument for missing required fields, Unimplemented for
// unsupported resource types, and mapped gRPC status codes for Azure API
// failures.
func (c *Calculator) EstimateCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)

	resourceType := strings.TrimSpace(req.GetResourceType())
	lowerType := strings.ToLower(resourceType)

	log.Info().
		Str("resource_type", resourceType).
		Msg("handling EstimateCost request")

	// Route by resource type: disk, VM (empty type included), then the shared quote.
	// The shared quote classifies the descriptor after attributes are copied,
	// so a native WebApp can see kind=FunctionApp.
	switch {
	case isManagedDiskResourceType(lowerType):
		return c.estimateDiskCost(ctx, req, resourceType)
	case resourceType == "" || isVirtualMachineResourceType(lowerType):
		return c.estimateVMCost(ctx, req, resourceType)
	default:
		if _, classErr := classifyResource(descriptorFromEstimate(req, resourceType)); classErr == nil {
			return c.estimateQuotedCost(ctx, req, resourceType)
		}
		err := status.Errorf(codes.Unimplemented, "unsupported resource type: %s", resourceType)
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost validation failed")
		return nil, err
	}
}

// estimateQuotedCost prices a mapped type through quoteResource.
func (c *Calculator) estimateQuotedCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
	resourceType string,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	resource := descriptorFromEstimate(req, resourceType)
	quote, err := c.quoteResource(ctx, resource, taskEstimate)
	if err != nil {
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost pricing failed")
		return nil, err
	}

	category := projectedPricingCategory(resource)
	if category == finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC {
		log.Warn().
			Str("resource_type", resourceType).
			Msg("spot interruption risk is unknown; score left at 0")
	}

	log.Info().
		Str("region", quote.region).
		Str("sku", quote.sku).
		Str("resource_type", resourceType).
		Float64("cost_monthly", quote.monthly).
		Str("currency", quote.currency).
		Str("result_status", "success").
		Msg("EstimateCost completed")

	return newEstimateResponse(quote.currency, quote.monthly, category, quote.advisories, quote.regions)
}

func descriptorFromEstimate(
	req *finfocusv1.EstimateCostRequest,
	resourceType string,
) *finfocusv1.ResourceDescriptor {
	attributes := map[string]any{}
	if req != nil && req.GetAttributes() != nil {
		attributes = req.GetAttributes().AsMap()
	}
	tags := make(map[string]string, len(attributes))
	for key, value := range attributes {
		tags[key] = fmt.Sprint(value)
	}
	return &finfocusv1.ResourceDescriptor{
		Provider:     providerAzure,
		ResourceType: resourceType,
		Region:       firstNonEmptyMapValue(attributes, "location", "region"),
		Sku:          firstNonEmptyMapValue(attributes, "sku", "vmSize", "armSkuName", "disk_type", "diskType"),
		Tags:         tags,
	}
}

// estimateVMCost handles VM cost estimation (existing path).
func (c *Calculator) estimateVMCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
	resourceType string,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)

	query, err := estimateQueryFromRequest(req)
	if err != nil {
		err = status.Error(codes.InvalidArgument, err.Error())
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost validation failed")
		return nil, err
	}

	spot, err := estimateSpot(req)
	if err != nil {
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost validation failed")
		return nil, err
	}

	if c.cachedClient == nil {
		unimplementedErr := status.Error(codes.Unimplemented, "not yet implemented")
		log.Warn().
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(unimplementedErr).
			Msg("EstimateCost unavailable")
		return nil, unimplementedErr
	}

	result, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost pricing lookup failed")
		return nil, err
	}

	item, err := selectVMItem(result.Items, spot)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost response mapping failed")
		return nil, err
	}

	unitPrice, currency, err := unitPriceAndCurrency([]azureclient.PriceItem{item})
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost response mapping failed")
		return nil, err
	}

	costMonthly := unitPrice * pluginsdk.HoursPerMonth
	category := estimateVMCategory(log, query.ArmRegionName, query.ArmSkuName, spot)
	advisories, regions := c.vmAdvisories(ctx, query, result.Items, spot, taskEstimate)

	log.Info().
		Str("region", query.ArmRegionName).
		Str("sku", query.ArmSkuName).
		Str("resource_type", resourceType).
		Float64("cost_monthly", costMonthly).
		Str("currency", currency).
		Str("result_status", "success").
		Msg("EstimateCost completed")

	return newEstimateResponse(currency, costMonthly, category, advisories, regions)
}

func estimateVMCategory(log zerolog.Logger, region, sku string, spot bool) finfocusv1.FocusPricingCategory {
	if !spot {
		return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
	}
	log.Warn().
		Str("region", region).
		Str("sku", sku).
		Msg("spot interruption risk is unknown; score left at 0")
	return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
}

// estimateDiskCost handles Managed Disk cost estimation.
// Disk pricing is monthly (not hourly like VMs), so retailPrice is used directly.
func (c *Calculator) estimateDiskCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
	resourceType string,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)

	query, diskInfo, sizeGB, err := estimateDiskQueryFromRequest(req)
	if err != nil {
		err = status.Error(codes.InvalidArgument, err.Error())
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost disk validation failed")
		return nil, err
	}

	tierName, err := tierForSize(diskInfo.TierPrefix, sizeGB)
	if err != nil {
		notFoundErr := status.Errorf(codes.NotFound,
			"no disk tier found for %.0f GB with type %s", sizeGB, diskInfo.ArmSkuName)
		log.Warn().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(notFoundErr).
			Msg("EstimateCost disk tier lookup failed")
		return nil, notFoundErr
	}
	query = diskRetailQuery(query.ArmRegionName, query.CurrencyCode, diskInfo, tierName)

	if c.cachedClient == nil {
		unimplementedErr := status.Error(codes.Unimplemented, "not yet implemented")
		log.Warn().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(unimplementedErr).
			Msg("EstimateCost unavailable")
		return nil, unimplementedErr
	}

	result, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost disk pricing lookup failed")
		return nil, err
	}

	costMonthly, currency, err := selectDiskTierPrice(result.Items, tierName, diskInfo.Redundancy)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("tier", tierName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost disk tier price lookup failed")
		return nil, err
	}

	log.Info().
		Str("region", query.ArmRegionName).
		Str("disk_type", diskInfo.ArmSkuName).
		Float64("size_gb", sizeGB).
		Str("tier", tierName).
		Str("resource_type", resourceType).
		Float64("cost_monthly", costMonthly).
		Str("currency", currency).
		Str("result_status", "success").
		Msg("EstimateCost disk completed")

	return pluginsdk.NewEstimateCostResponse(
		pluginsdk.WithEstimateCost(currency, costMonthly),
		pluginsdk.WithPricingCategory(
			finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD,
		),
	), nil
}

// estimateDiskQueryFromRequest extracts and validates disk-specific attributes
// from an EstimateCostRequest. Returns the PriceQuery, diskTypeInfo, sizeGB,
// or an error listing all missing/invalid fields.
func estimateDiskQueryFromRequest(
	req *finfocusv1.EstimateCostRequest,
) (azureclient.PriceQuery, diskTypeInfo, float64, error) {
	attributes := map[string]any{}
	if req != nil && req.GetAttributes() != nil {
		attributes = req.GetAttributes().AsMap()
	}

	region := firstNonEmptyMapValue(attributes, "location", "region")
	diskTypeStr := firstNonEmptyMapValue(attributes, "diskType", "disk_type", "sku")
	sizeGBStr := firstNonEmptyMapValue(attributes, "sizeGb", "size_gb", "diskSizeGb")
	currency := firstNonEmptyMapValue(attributes, "currencyCode", "currency")
	if currency == "" {
		currency = defaultCurrency
	}

	// Validate required fields — report all missing in one error.
	var missingFields []string
	if region == "" {
		missingFields = append(missingFields, "region")
	}
	if diskTypeStr == "" {
		missingFields = append(missingFields, "disk_type")
	}
	if sizeGBStr == "" {
		missingFields = append(missingFields, "size_gb")
	}
	if len(missingFields) > 0 {
		return azureclient.PriceQuery{}, diskTypeInfo{}, 0,
			fmt.Errorf("missing required field(s): %s", strings.Join(missingFields, ", "))
	}

	// Parse and validate size_gb.
	sizeGB, err := parseSizeGB(sizeGBStr)
	if err != nil {
		return azureclient.PriceQuery{}, diskTypeInfo{}, 0, err
	}

	// Validate and normalize disk type.
	diskInfo, err := normalizeDiskType(diskTypeStr)
	if err != nil {
		return azureclient.PriceQuery{}, diskTypeInfo{}, 0, err
	}

	query := azureclient.PriceQuery{
		ArmRegionName: region,
		ArmSkuName:    diskInfo.ArmSkuName,
		ServiceName:   managedDisksService,
		CurrencyCode:  currency,
	}

	return query, diskInfo, sizeGB, nil
}

// parseSizeGB parses and validates the size_gb attribute value.
func parseSizeGB(value string) (float64, error) {
	var sizeGB float64
	_, err := fmt.Sscanf(value, "%f", &sizeGB)
	if err != nil {
		return 0, fmt.Errorf("size_gb must be a valid number: %s", value)
	}
	if sizeGB <= 0 {
		return 0, errors.New("size_gb must be greater than 0")
	}
	return sizeGB, nil
}

// DryRun delegates to HandleDryRun so direct callers and the gRPC server agree.
func (c *Calculator) DryRun(
	ctx context.Context,
	req *finfocusv1.DryRunRequest,
) (*finfocusv1.DryRunResponse, error) {
	return c.HandleDryRun(ctx, req)
}

// HandleDryRun validates a resource descriptor without calling Azure.
// pluginsdk.Server.DryRun calls this method.
func (c *Calculator) HandleDryRun(
	ctx context.Context,
	req *finfocusv1.DryRunRequest,
) (*finfocusv1.DryRunResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling DryRun request")

	if req.GetResource() == nil {
		return nil, status.Error(codes.InvalidArgument, "resource descriptor is required")
	}

	query, err := MapDescriptorToQuery(dryRunDescriptor(req.GetResource()))
	if errors.Is(err, ErrUnsupportedResourceType) {
		return unsupportedDryRunResponse(), nil
	}
	if err != nil {
		return invalidDryRunResponse(err.Error()), nil
	}

	filter := retailPriceFilter(query)
	log.Debug().Str("odata_filter", filter).Msg("odata filter omitted from DryRunResponse")

	return supportedDryRunResponse(), nil
}

// dryRunDescriptor fills an empty provider from the resource type token.
// `finfocus plugin inspect <plugin> <type>` sends DryRun with the resource type
// only. An azure-native token means azure-native; an azure token or a bare
// canonical type such as compute/VirtualMachine means azure. Another cloud's
// token keeps the empty provider and stays unsupported. Supports and the cost
// RPCs keep requiring the provider, which core always sends there.
func dryRunDescriptor(resource *finfocusv1.ResourceDescriptor) *finfocusv1.ResourceDescriptor {
	if strings.TrimSpace(resource.GetProvider()) != "" {
		return resource
	}
	resourceType := strings.ToLower(strings.TrimSpace(resource.GetResourceType()))
	var provider string
	switch {
	case strings.HasPrefix(resourceType, providerAzureNative+":"):
		provider = providerAzureNative
	case strings.HasPrefix(resourceType, providerAzure+":"), !strings.Contains(resourceType, ":"):
		provider = providerAzure
	default:
		return resource
	}
	inferred, ok := proto.Clone(resource).(*finfocusv1.ResourceDescriptor)
	if !ok {
		return resource
	}
	inferred.Provider = provider
	return inferred
}

func supportedDryRunResponse() *finfocusv1.DryRunResponse {
	return pluginsdk.NewDryRunResponse(
		pluginsdk.WithFieldMappings(projectedFieldMappings()),
		pluginsdk.WithResourceTypeSupported(true),
		pluginsdk.WithConfigurationValid(true),
	)
}

func invalidDryRunResponse(message string) *finfocusv1.DryRunResponse {
	return pluginsdk.NewDryRunResponse(
		pluginsdk.WithFieldMappings(projectedFieldMappings()),
		pluginsdk.WithResourceTypeSupported(true),
		pluginsdk.WithConfigurationValid(false),
		pluginsdk.WithConfigurationErrors([]string{message}),
	)
}

func unsupportedDryRunResponse() *finfocusv1.DryRunResponse {
	return pluginsdk.NewDryRunResponse(
		pluginsdk.WithResourceTypeSupported(false),
		pluginsdk.WithConfigurationValid(true),
	)
}

// projectedFocusFields lists FOCUS names filled by GetProjectedCostResponse:
// cost_per_month, currency, billing_detail, unit_price, and pricing_category.
// provider_name is not set on that response.
func projectedFocusFields() []string {
	return []string{
		"billed_cost",
		"billing_currency",
		"charge_description",
		"list_unit_price",
		"pricing_category",
	}
}

func projectedFieldMappings() []*finfocusv1.FieldMapping {
	mappings := pluginsdk.AllFieldsWithStatus(
		finfocusv1.FieldSupportStatus_FIELD_SUPPORT_STATUS_UNSUPPORTED,
	)
	supported := finfocusv1.FieldSupportStatus_FIELD_SUPPORT_STATUS_SUPPORTED
	for _, name := range projectedFocusFields() {
		mappings = pluginsdk.SetFieldStatus(mappings, name, supported)
	}
	return mappings
}

// retailPriceFilter is the OData filter GetPrices would send.
// DryRunResponse has no field for it.
func retailPriceFilter(query *azureclient.PriceQuery) string {
	return azureclient.NewFilterBuilder().
		Region(query.ArmRegionName).
		SKU(query.ArmSkuName).
		Service(query.ServiceName).
		ProductName(query.ProductName).
		CurrencyCode(query.CurrencyCode).
		Build()
}

// estimateQueryFromRequest extracts an Azure pricing query from EstimateCost
// request attributes. Supported keys include location/region and
// vmSize/sku/armSkuName with defaults for serviceName and currencyCode.
// Returns an error in the format "missing required field(s): ..." when
// required fields are missing.
func estimateQueryFromRequest(req *finfocusv1.EstimateCostRequest) (azureclient.PriceQuery, error) {
	attributes := map[string]any{}
	if req != nil && req.GetAttributes() != nil {
		attributes = req.GetAttributes().AsMap()
	}

	query := azureclient.PriceQuery{
		ArmRegionName: firstNonEmptyMapValue(attributes, "location", "region"),
		ArmSkuName:    firstNonEmptyMapValue(attributes, "vmSize", "sku", "armSkuName"),
		ServiceName:   firstNonEmptyMapValue(attributes, "serviceName"),
		ProductName:   firstNonEmptyMapValue(attributes, "productName"),
		CurrencyCode:  firstNonEmptyMapValue(attributes, "currencyCode", "currency"),
	}
	if query.CurrencyCode == "" {
		query.CurrencyCode = defaultCurrency
	}
	if query.ServiceName == "" {
		query.ServiceName = defaultServiceName
	}
	var missingFields []string
	if query.ArmRegionName == "" {
		missingFields = append(missingFields, "region")
	}
	if query.ArmSkuName == "" {
		missingFields = append(missingFields, "sku")
	}
	if len(missingFields) > 0 {
		return azureclient.PriceQuery{}, fmt.Errorf(
			"missing required field(s): %s",
			strings.Join(missingFields, ", "),
		)
	}

	return query, nil
}

// isVirtualMachineResourceType checks whether the lowercased resource type
// refers to compute/virtualmachine as a full segment (not a prefix of e.g.
// "compute/virtualmachinescaleset").
func isWindowsVirtualMachineResourceType(lower string) bool {
	return resourceSegment(lower, "compute/windowsvirtualmachine")
}

func isVirtualMachineScaleSetResourceType(lower string) bool {
	return resourceSegment(lower, "compute/linuxvirtualmachinescaleset") ||
		resourceSegment(lower, "compute/windowsvirtualmachinescaleset") ||
		resourceSegment(lower, "compute/orchestratedvirtualmachinescaleset") ||
		tokenSuffix(lower, "compute", "virtualmachinescaleset")
}

func isPricedVMResourceType(lower string) bool {
	return isVirtualMachineResourceType(lower) ||
		isWindowsVirtualMachineResourceType(lower) ||
		isVirtualMachineScaleSetResourceType(lower)
}

func isVirtualMachineResourceType(lower string) bool {
	return resourceSegment(lower, "compute/virtualmachine") ||
		resourceSegment(lower, "compute/linuxvirtualmachine") ||
		tokenSuffix(lower, "compute", "virtualmachine")
}

func firstNonEmptyTag(tags map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(tags[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyMapValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprintf("%v", raw))
			if text != "" && text != formattedNil {
				return text
			}
		}
	}
	return ""
}

func unitPriceAndCurrency(items []azureclient.PriceItem) (float64, string, error) {
	if len(items) == 0 {
		return 0, "", azureclient.ErrNotFound
	}

	item := items[0]
	price := item.RetailPrice
	if price == 0 {
		price = item.UnitPrice
	}
	currency := item.CurrencyCode
	if strings.TrimSpace(currency) == "" {
		currency = defaultCurrency
	}

	return price, currency, nil
}
