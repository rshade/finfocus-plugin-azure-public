package pricing

import (
	"fmt"
	"sort"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// defaultCurrency is the currency code used when no preference is specified.
const defaultCurrency = "USD"

// managedDisksService is the Azure Retail Prices service name for managed disks.
const managedDisksService = "Managed Disks"

// resourceTypeToService maps normalized (lowercased) resource type identifiers
// to their corresponding Azure service names for the Retail Prices API.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var resourceTypeToService = map[string]string{
	"compute/virtualmachine":      "Virtual Machines",
	"storage/manageddisk":         managedDisksService,
	"storage/blobstorage":         storageServiceName,
	storageAccountResourceSegment: storageServiceName,
	appServicePlanSegment:         appServiceName,
	functionAppSegment:            functionsServiceName,
	aksResourceSegment:            aksServiceName,
	sqlDatabaseSegment:            sqlServiceName,
	cosmosAccountSegment:          cosmosServiceName,
	loadBalancerSegment:           loadBalancerServiceName,
}

// canonicalResourceTypes maps normalized keys back to their display form.
//
//nolint:gochecknoglobals // Static lookup table; immutable after init.
var canonicalResourceTypes = map[string]string{
	"compute/virtualmachine":      "compute/VirtualMachine",
	"storage/manageddisk":         "storage/ManagedDisk",
	"storage/blobstorage":         "storage/BlobStorage",
	storageAccountResourceSegment: "storage/StorageAccount",
	appServicePlanSegment:         canonicalAppServicePlan,
	functionAppSegment:            canonicalFunctionApp,
	aksResourceSegment:            canonicalKubernetesCluster,
	sqlDatabaseSegment:            canonicalSQLDatabase,
	cosmosAccountSegment:          canonicalCosmosAccount,
	loadBalancerSegment:           canonicalLoadBalancer,
}

// MapDescriptorToQuery translates a finfocus ResourceDescriptor into an
// azureclient PriceQuery suitable for the Azure Retail Prices API.
//
// Validation is performed before mapping:
//   - Provider must be "azure", or "azure-native" from finfocus releases
//     before rshade/finfocus#1645 (case-insensitive)
//   - ResourceType must match a supported type (case-insensitive)
//   - Region must be resolvable (primary field or Tags["region"])
//   - SKU must be resolvable (primary field or Tags["sku"]), except a function
//     app, which may omit it
//
// Storage accounts also match a type that contains the storage/storageAccount
// segment, including a Pulumi form. When Sku and Tags["sku"] are empty, the
// SKU is Tags["tier"] or Tags["access_tier"] plus Tags["redundancy"]. The
// query leaves ArmSkuName empty and sets ProductName to General Block Blob v2.
// There is no ARM SKU to invent.
//
// App Service plans match web/appserviceplan and the Pulumi appservice/plan
// segment. Function apps match web/functionapp and appservice/functionapp.
// Both leave ArmSkuName empty: the short plan SKU is not an armSkuName filter.
// A function app may omit SKU when it is classic Consumption.
//
// AKS clusters match containerservice/kubernetescluster, including the Pulumi
// kubernetesCluster segment. ArmSkuName and ProductName stay empty. The tier
// is not an ARM SKU. Sku, Tags["sku"], or Tags["tier"] satisfies that check.
//
// SQL databases match sql/database, including Pulumi azure:sql/database:Database.
// ArmSkuName and ProductName stay empty. GP_Gen5_2 is not an ARM SKU. Sku,
// Tags["sku"], or tags tier, hardware, and vcores satisfy that check.
//
// Cosmos DB accounts match cosmosdb/account, including Pulumi
// azure:cosmosdb/account:Account. ArmSkuName and ProductName stay empty.
// Throughput is not an ARM SKU. Region is required. Sku may be empty.
//
// Load balancers match network/loadbalancer and the classic lb/loadbalancer
// token. ArmSkuName stays empty. Region is required. Sku may be empty and
// then means Standard. The price query may fall back to Global.
//
// Returns ErrUnsupportedResourceType for unknown providers or resource types.
// Returns ErrMissingRequiredFields naming all missing fields in a single error.
// MapDescriptorToQuery returns a PriceQuery whose CurrencyCode defaults to USD.
func MapDescriptorToQuery(desc *finfocusv1.ResourceDescriptor) (*azureclient.PriceQuery, error) {
	if desc == nil {
		return nil, fmt.Errorf("%w: descriptor is nil", ErrMissingRequiredFields)
	}

	// Validate provider (case-insensitive). Core copies azure-native from the token prefix.
	if !acceptedAzureProvider(desc.GetProvider()) {
		return nil, fmt.Errorf("unsupported provider: %s: %w", desc.GetProvider(), ErrUnsupportedResourceType)
	}

	// Look up resource type (case-insensitive).
	normalizedType := strings.ToLower(strings.TrimSpace(desc.GetResourceType()))
	mapped, ok := resolveMappedResource(normalizedType, desc.GetTags())
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s: %w", desc.GetResourceType(), ErrUnsupportedResourceType)
	}
	serviceName := mapped.serviceName
	storageAccount := mapped.storageAccount

	// Resolve fields with tag fallback.
	region := resolveField(desc.GetRegion(), "region", desc.GetTags())
	sku, err := mappedSKU(mapped, desc)
	if err != nil {
		return nil, err
	}

	// Validate required fields — report all missing in one error.
	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	missing = append(missing, missingMappedSKU(mapped, sku, desc.GetTags())...)
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrMissingRequiredFields, strings.Join(missing, ", "))
	}

	query := &azureclient.PriceQuery{
		ArmRegionName: region,
		ArmSkuName:    sku,
		ServiceName:   serviceName,
		CurrencyCode:  defaultCurrency,
	}
	if storageAccount {
		query.ArmSkuName = ""
		query.ProductName = generalBlockBlobV2Product
	}
	if mapped.appServicePlan || mapped.functionApp || mapped.aks ||
		mapped.sqlDatabase || mapped.cosmos || mapped.loadBalancer {
		query.ArmSkuName = ""
	}
	return query, nil
}

type mappedResource struct {
	serviceName    string
	vm             bool
	disk           bool
	storageAccount bool
	appServicePlan bool
	functionApp    bool
	aks            bool
	sqlDatabase    bool
	cosmos         bool
	loadBalancer   bool
}

func resolveMappedResource(normalizedType string, tags map[string]string) (mappedResource, bool) {
	serviceName, ok := resourceTypeToService[normalizedType]
	mapped := mappedResource{serviceName: serviceName}
	mapped, ok = matchDirectSegments(mapped, ok, normalizedType)
	if isStorageAccountResourceType(normalizedType) {
		mapped.storageAccount = true
		if !ok {
			mapped.serviceName = storageServiceName
			ok = true
		}
	}
	if isFunctionAppResourceType(normalizedType) || isNativeFunctionWebApp(normalizedType, tags) {
		mapped.functionApp = true
		if !ok {
			mapped.serviceName = functionsServiceName
			ok = true
		}
	}
	if isAppServicePlanResourceType(normalizedType) {
		mapped.appServicePlan = true
		if !ok {
			mapped.serviceName = appServiceName
			ok = true
		}
	}
	if isAKSResourceType(normalizedType) {
		mapped.aks = true
		if !ok {
			mapped.serviceName = aksServiceName
			ok = true
		}
	}
	if isSQLDatabaseResourceType(normalizedType) {
		mapped.sqlDatabase = true
		if !ok {
			mapped.serviceName = sqlServiceName
			ok = true
		}
	}
	if isCosmosAccountResourceType(normalizedType) {
		mapped.cosmos = true
		if !ok {
			mapped.serviceName = cosmosServiceName
			ok = true
		}
	}
	return markLoadBalancer(mapped, ok, normalizedType)
}

func matchDirectSegments(mapped mappedResource, ok bool, normalizedType string) (mappedResource, bool) {
	if isPricedVMResourceType(normalizedType) {
		mapped.serviceName = defaultServiceName
		mapped.vm = true
		ok = true
	}
	if isManagedDiskResourceType(normalizedType) {
		mapped.serviceName = managedDisksService
		mapped.disk = true
		ok = true
	}
	if isBlobStorageResourceType(normalizedType) {
		mapped.serviceName = storageServiceName
		ok = true
	}
	return mapped, ok
}

func markLoadBalancer(mapped mappedResource, ok bool, normalizedType string) (mappedResource, bool) {
	if !isLoadBalancerResourceType(normalizedType) {
		return mapped, ok
	}
	mapped.loadBalancer = true
	if !ok {
		mapped.serviceName = loadBalancerServiceName
		ok = true
	}
	return mapped, ok
}

// mappedSKU reads the SKU from the real Pulumi property for the type, so
// Supports and DryRun accept what GetProjectedCost prices.
func mappedSKU(mapped mappedResource, desc *finfocusv1.ResourceDescriptor) (string, error) {
	switch {
	case mapped.storageAccount:
		return storageAccountSKU(desc)
	case mapped.sqlDatabase:
		return sqlDatabaseSKU(desc)
	case mapped.aks:
		return mappedAKSTier(desc)
	case mapped.appServicePlan:
		if err := validateAppServicePlanInput(desc); err != nil {
			return "", err
		}
		return appServicePlanSKU(desc), nil
	case mapped.disk:
		return diskSKU(desc), nil
	case mapped.vm:
		return vmSKU(desc), nil
	default:
		return resolveField(desc.GetSku(), "sku", desc.GetTags()), nil
	}
}

// mappedAKSTier returns the tier when aksControlPlane accepts it, so a native
// cluster whose Sku is Base or Automatic is not reported as configured.
func mappedAKSTier(desc *finfocusv1.ResourceDescriptor) (string, error) {
	tier := aksTier(desc)
	if tier == "" {
		return "", nil
	}
	if _, _, err := aksControlPlane(desc); err != nil {
		return "", err
	}
	return tier, nil
}

func missingMappedSKU(mapped mappedResource, sku string, tags map[string]string) []string {
	if sku != "" || mapped.functionApp || mapped.cosmos || mapped.loadBalancer {
		return nil
	}
	if mapped.sqlDatabase {
		return sqlIdentityMissing(tags)
	}
	name := "sku"
	if mapped.aks {
		name = aksTierTag
	}
	return []string{name}
}

// SupportedResourceTypes returns the list of resource type identifiers that
// have a defined mapping to Azure service names.
// SupportedResourceTypes returns those identifiers in canonical form, sorted alphabetically.
func SupportedResourceTypes() []string {
	types := make([]string, 0, len(canonicalResourceTypes))
	for _, canonical := range canonicalResourceTypes {
		types = append(types, canonical)
	}
	sort.Strings(types)
	return types
}

// resolveField returns the primary value if non-empty, otherwise falls back
// to the tag value identified by tagKey. Returns empty string if neither is
// available.
func resolveField(primary, tagKey string, tags map[string]string) string {
	if primary != "" {
		return primary
	}
	if tags != nil {
		return tags[tagKey]
	}
	return ""
}
