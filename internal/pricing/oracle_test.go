package pricing

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const oracleToleranceFraction = 0.005

type oracleFile struct {
	Cases []oracleCase `json:"cases"`
}

type oracleCase struct {
	ID       string                  `json:"id"`
	Kind     string                  `json:"kind"`
	Status   string                  `json:"status"`
	Expected *float64                `json:"expected_monthly"`
	Notes    []string                `json:"notes"`
	Params   map[string]any          `json:"params"`
	Region   string                  `json:"region"`
	Hint     *oracleHint             `json:"request_hint"`
	Rows     []azureclient.PriceItem `json:"rows"`
}

type oracleHint struct {
	Provider     string            `json:"provider"`
	Region       string            `json:"region"`
	ResourceType string            `json:"resource_type"`
	SKU          string            `json:"sku"`
	Tags         map[string]string `json:"tags"`
}

type oracleResult struct {
	id       string
	status   string
	plugin   string
	expected string
	diff     string
	estimate string
	verdict  string
	fail     string
}

func TestOracleComparison(t *testing.T) {
	cases := loadOracleCases(t)
	if len(cases) < 50 {
		t.Fatalf("oracle cases = %d, want at least 50", len(cases))
	}

	live := os.Getenv("ORACLE_LIVE") == "1"
	var shared *Calculator
	if live {
		shared = newLivePricingCalc(t)
	}

	results := make([]oracleResult, 0, len(cases))
	for _, item := range cases {
		results = append(results, runOracleCase(t, item, shared))
	}
	writeOracleResults(t, results)

	failed := 0
	for _, result := range results {
		if result.fail == "" {
			continue
		}
		failed++
		t.Errorf("%s: %s", result.id, result.fail)
	}
	if failed > 0 {
		t.Logf("%d oracle cases failed; table is in .superpowers/oracle-results.md", failed)
	}
}

func runOracleCase(t *testing.T, item oracleCase, shared *Calculator) oracleResult {
	t.Helper()

	result := oracleResult{
		id:       item.ID,
		status:   item.Status,
		expected: formatOracleNumber(item.Expected),
		estimate: "n/a",
	}

	req, err := oracleProjectedRequest(item)
	if err != nil {
		result.plugin = "unexpressed"
		result.verdict = "finding"
		result.fail = err.Error()
		return result
	}

	calc := shared
	if calc == nil {
		calc = newPricingCalc(t, item.Rows)
	}

	pluginCost, callErr := callOracleProjected(t, calc, req)
	estimateCost, estimateErr := callOracleEstimate(t, calc, item)
	if estimateErr == nil && !math.IsNaN(estimateCost) {
		result.estimate = strconv.FormatFloat(estimateCost, 'f', -1, 64)
	} else if estimateErr != nil && status.Code(estimateErr) != codes.Unimplemented {
		result.estimate = status.Code(estimateErr).String()
	}

	switch item.Status {
	case "ok":
		judgeOracleOK(&result, item, pluginCost, callErr, estimateCost, estimateErr)
	case "not_available":
		judgeOracleMissing(&result, pluginCost, callErr)
	case "ambiguous":
		judgeOracleAmbiguous(&result, pluginCost, callErr)
	default:
		result.fail = "unknown oracle status " + item.Status
	}
	return result
}

func judgeOracleOK(
	result *oracleResult,
	item oracleCase,
	pluginCost float64,
	callErr error,
	estimateCost float64,
	estimateErr error,
) {
	if callErr != nil {
		result.plugin = status.Code(callErr).String()
		result.verdict = "fail"
		result.fail = "GetProjectedCost error: " + status.Convert(callErr).Message()
		return
	}
	result.plugin = strconv.FormatFloat(pluginCost, 'f', -1, 64)
	if item.Expected == nil {
		result.verdict = "fail"
		result.fail = "ok case has no expected_monthly"
		return
	}
	delta := pluginCost - *item.Expected
	result.diff = strconv.FormatFloat(delta, 'f', -1, 64)
	if math.Abs(delta) > oracleTolerance(*item.Expected) {
		result.verdict = "fail"
		result.fail = fmt.Sprintf(
			"cost %v outside tolerance of %v",
			pluginCost,
			*item.Expected,
		)
		return
	}
	result.verdict = "pass"
	if estimateErr == nil && item.Kind != "vm_spot_linux" {
		if math.Abs(estimateCost-pluginCost) > oracleTolerance(pluginCost) {
			result.verdict = "fail"
			result.fail = fmt.Sprintf(
				"EstimateCost %v disagrees with GetProjectedCost %v",
				estimateCost,
				pluginCost,
			)
		}
	}
	if item.Kind == "vm_spot_linux" && estimateErr == nil {
		if math.Abs(estimateCost-pluginCost) > oracleTolerance(pluginCost) {
			result.estimate += " (on-demand path; AZ-6.2)"
		}
	}
}

func judgeOracleMissing(result *oracleResult, pluginCost float64, callErr error) {
	if callErr == nil {
		result.plugin = strconv.FormatFloat(pluginCost, 'f', -1, 64)
		result.verdict = "fail"
		result.fail = "not_available returned a cost"
		return
	}
	result.plugin = status.Code(callErr).String()
	result.verdict = "pass"
	if pluginCost == 0 && status.Code(callErr) == codes.OK {
		result.verdict = "fail"
		result.fail = "not_available returned a zero cost"
	}
}

func judgeOracleAmbiguous(result *oracleResult, pluginCost float64, callErr error) {
	if callErr != nil {
		result.plugin = status.Code(callErr).String() + ": " + status.Convert(callErr).Message()
		result.verdict = "choice"
		return
	}
	result.plugin = strconv.FormatFloat(pluginCost, 'f', -1, 64)
	if pluginCost == 0 {
		result.verdict = "fail"
		result.fail = "ambiguous case returned a zero cost"
		return
	}
	result.verdict = "choice"
}

func oracleTolerance(expected float64) float64 {
	fraction := math.Abs(expected) * oracleToleranceFraction
	if fraction > 0.01 {
		return fraction
	}
	return 0.01
}

func callOracleProjected(
	t *testing.T,
	calc *Calculator,
	req *finfocusv1.GetProjectedCostRequest,
) (float64, error) {
	t.Helper()

	client := oracleGRPCClient(t, calc)
	resp, err := client.GetProjectedCost(context.Background(), req)
	if err != nil {
		return 0, err
	}
	if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
		return resp.GetCostPerMonth(), status.Errorf(codes.Internal, "invalid response: %v", validateErr)
	}
	return resp.GetCostPerMonth(), nil
}

func callOracleEstimate(t *testing.T, calc *Calculator, item oracleCase) (float64, error) {
	t.Helper()

	req, ok := oracleEstimateRequest(item)
	if !ok {
		return math.NaN(), status.Error(codes.Unimplemented, "estimate not in scope for kind")
	}
	client := oracleGRPCClient(t, calc)
	resp, err := client.EstimateCost(context.Background(), req)
	if err != nil {
		return 0, err
	}
	return resp.GetCostMonthly(), nil
}

func oracleGRPCClient(t *testing.T, calc *Calculator) finfocusv1.CostSourceServiceClient {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	finfocusv1.RegisterCostSourceServiceServer(server, pluginsdk.NewServer(calc))
	go func() {
		_ = server.Serve(lis)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///"+lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return finfocusv1.NewCostSourceServiceClient(conn)
}

func oracleProjectedRequest(item oracleCase) (*finfocusv1.GetProjectedCostRequest, error) {
	if item.Hint != nil {
		return &finfocusv1.GetProjectedCostRequest{
			Resource: &finfocusv1.ResourceDescriptor{
				Provider:     item.Hint.Provider,
				ResourceType: item.Hint.ResourceType,
				Region:       item.Hint.Region,
				Sku:          item.Hint.SKU,
				Tags:         item.Hint.Tags,
			},
		}, nil
	}

	resource, err := oracleResourceFromKind(item)
	if err != nil {
		return nil, err
	}
	return &finfocusv1.GetProjectedCostRequest{Resource: resource}, nil
}

func oracleResourceFromKind(item oracleCase) (*finfocusv1.ResourceDescriptor, error) {
	switch item.Kind {
	case "managed_disk":
		return oracleDiskResource(item)
	case "app_service_plan":
		return oracleAppServiceResource(item)
	case "aks_control_plane":
		return oracleAKSResource(item), nil
	case "sql_database":
		return oracleSQLResource(item)
	case "cosmos_manual", "cosmos_autoscale":
		return oracleCosmosResource(item)
	case "function_app":
		return oracleFunctionResource(item)
	default:
		return nil, fmt.Errorf("no request for kind %s", item.Kind)
	}
}

func oracleDiskResource(item oracleCase) (*finfocusv1.ResourceDescriptor, error) {
	tier, _ := paramString(item.Params, "tier")
	redundancy, _ := paramString(item.Params, "redundancy")
	diskType, sizeGB, err := oracleDiskSelection(tier, redundancy)
	if err != nil {
		return nil, err
	}
	return &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "storage/ManagedDisk",
		Region:       item.Region,
		Sku:          diskType,
		Tags:         map[string]string{"size_gb": sizeGB},
	}, nil
}

func oracleDiskSelection(tier, redundancy string) (string, string, error) {
	if tier == "" || redundancy == "" {
		return "", "", errors.New("disk case missing tier or redundancy")
	}
	number, err := strconv.Atoi(strings.TrimLeft(tier, "PES"))
	if err != nil {
		return "", "", fmt.Errorf("disk tier %q: %w", tier, err)
	}
	// Capacities match diskTierCapacities. The size is the tier's own GiB,
	// so ceiling-match selects that tier and not the next one.
	capacities := map[int]int{
		1: 4, 2: 8, 3: 16, 4: 32, 6: 64, 10: 128, 15: 256,
		20: 512, 30: 1024, 40: 2048, 50: 4096, 60: 8192, 70: 16384, 80: 32767,
	}
	size, ok := capacities[number]
	if !ok {
		return "", "", fmt.Errorf("no capacity for disk tier %s", tier)
	}
	prefix := tier[:1]
	types := map[string]string{
		"P/LRS": "Premium_SSD_LRS",
		"P/ZRS": "Premium_ZRS",
		"E/LRS": "StandardSSD_LRS",
		"E/ZRS": "StandardSSD_ZRS",
		"S/LRS": "Standard_LRS",
		"S/ZRS": "Standard_ZRS",
	}
	diskType, ok := types[prefix+"/"+redundancy]
	if !ok {
		return "", "", fmt.Errorf("no disk type for %s %s", tier, redundancy)
	}
	return diskType, strconv.Itoa(size), nil
}

func oracleAppServiceResource(item oracleCase) (*finfocusv1.ResourceDescriptor, error) {
	sku, ok := paramString(item.Params, "sku")
	if !ok {
		return nil, errors.New("app service case missing sku")
	}
	osName, _ := paramString(item.Params, "os")
	resource := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "web/AppServicePlan",
		Region:       item.Region,
		Sku:          sku,
		Tags:         map[string]string{},
	}
	if strings.EqualFold(osName, "windows") {
		resource.Tags["os"] = "Windows"
	}
	return resource, nil
}

func oracleAKSResource(item oracleCase) *finfocusv1.ResourceDescriptor {
	tier, _ := paramString(item.Params, "tier")
	return &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "containerservice/KubernetesCluster",
		Region:       item.Region,
		Sku:          tier,
	}
}

func oracleSQLResource(item oracleCase) (*finfocusv1.ResourceDescriptor, error) {
	vcores, ok := paramFloat(item.Params, "vcores")
	if !ok {
		return nil, errors.New("sql case missing vcores")
	}
	storage, ok := paramFloat(item.Params, "storage_gb")
	if !ok {
		return nil, errors.New("sql case missing storage_gb")
	}
	tags := map[string]string{
		"size_gb": strconv.FormatFloat(storage, 'f', -1, 64),
	}
	if zone, _ := paramBool(item.Params, "zone_redundant"); zone {
		tags["zone_redundant"] = "true"
	}
	return &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "sql/Database",
		Region:       item.Region,
		Sku:          fmt.Sprintf("GP_Gen5_%g", vcores),
		Tags:         tags,
	}, nil
}

func oracleCosmosResource(item oracleCase) (*finfocusv1.ResourceDescriptor, error) {
	ru, ok := paramFloat(item.Params, "ru_per_second")
	if !ok {
		return nil, errors.New("cosmos case missing ru_per_second")
	}
	tags := map[string]string{
		"ru_per_second": strconv.FormatFloat(ru, 'f', -1, 64),
	}
	if item.Kind == "cosmos_autoscale" {
		tags["pricing_model"] = "autoscale"
	}
	return &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "cosmosdb/Account",
		Region:       item.Region,
		Tags:         tags,
	}, nil
}

func oracleFunctionResource(item oracleCase) (*finfocusv1.ResourceDescriptor, error) {
	executions, ok := paramFloat(item.Params, "executions_per_month")
	if !ok {
		return nil, errors.New("function case missing executions_per_month")
	}
	gbSeconds, ok := paramFloat(item.Params, "gb_seconds")
	if !ok {
		return nil, errors.New("function case missing gb_seconds")
	}
	return &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "web/FunctionApp",
		Region:       item.Region,
		Tags: map[string]string{
			"executions": strconv.FormatFloat(executions, 'f', -1, 64),
			"gb_seconds": strconv.FormatFloat(gbSeconds, 'f', -1, 64),
		},
	}, nil
}

func oracleEstimateRequest(item oracleCase) (*finfocusv1.EstimateCostRequest, bool) {
	switch item.Kind {
	case "vm_ondemand_linux", "vm_spot_linux":
		sku, _ := paramString(item.Params, "sku")
		if item.Hint != nil && item.Hint.SKU != "" {
			sku = item.Hint.SKU
		}
		attrs, err := structpb.NewStruct(map[string]any{
			"location": item.Region,
			"vmSize":   sku,
		})
		if err != nil {
			return nil, false
		}
		return &finfocusv1.EstimateCostRequest{
			ResourceType: "compute/VirtualMachine",
			Attributes:   attrs,
		}, true
	case "managed_disk":
		tier, _ := paramString(item.Params, "tier")
		redundancy, _ := paramString(item.Params, "redundancy")
		diskType, sizeGB, err := oracleDiskSelection(tier, redundancy)
		if err != nil {
			return nil, false
		}
		size, _ := strconv.ParseFloat(sizeGB, 64)
		attrs, err := structpb.NewStruct(map[string]any{
			"location":  item.Region,
			"disk_type": diskType,
			"size_gb":   size,
		})
		if err != nil {
			return nil, false
		}
		return &finfocusv1.EstimateCostRequest{
			ResourceType: "storage/ManagedDisk",
			Attributes:   attrs,
		}, true
	default:
		return nil, false
	}
}

func loadOracleCases(t *testing.T) []oracleCase {
	t.Helper()

	path := filepath.Join("testdata", "oracle", "expected.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read oracle: %v", err)
	}
	var file oracleFile
	if err = json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse oracle: %v", err)
	}
	return file.Cases
}

func newLivePricingCalc(t *testing.T) *Calculator {
	t.Helper()

	cfg := azureclient.DefaultConfig()
	cfg.Logger = zerolog.Nop()
	client, err := azureclient.NewClient(cfg)
	if err != nil {
		t.Fatalf("live client: %v", err)
	}
	cacheCfg := azureclient.DefaultCacheConfig()
	cacheCfg.Logger = zerolog.Nop()
	cached, err := azureclient.NewCachedClient(client, cacheCfg)
	if err != nil {
		t.Fatalf("live cache: %v", err)
	}
	t.Cleanup(func() { cached.Close() })
	return NewCalculator(zerolog.Nop(), cached)
}

func writeOracleResults(t *testing.T, results []oracleResult) {
	t.Helper()

	root := findRepoRoot(t)
	dir := filepath.Join(root, ".superpowers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir results: %v", err)
	}
	var b strings.Builder
	b.WriteString("# Oracle results\n\n")
	b.WriteString("| Case | Status | Plugin | Expected | Difference | EstimateCost | Verdict |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, result := range results {
		fmt.Fprintf(
			&b,
			"| %s | %s | %s | %s | %s | %s | %s |\n",
			result.id,
			result.status,
			escapeCell(result.plugin),
			escapeCell(result.expected),
			escapeCell(result.diff),
			escapeCell(result.estimate),
			escapeCell(result.verdict),
		)
	}
	b.WriteString("\n## Calculator values\n\n")
	b.WriteString(oracleCalculatorNotes(t))
	path := filepath.Join(dir, "oracle-results.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write results: %v", err)
	}
}

func oracleCalculatorNotes(t *testing.T) string {
	t.Helper()

	path := filepath.Join("testdata", "oracle", "calculator-values.csv")
	file, err := os.Open(path)
	if err != nil {
		return "could not read calculator-values.csv: " + err.Error() + "\n"
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return "could not parse calculator-values.csv: " + err.Error() + "\n"
	}
	var b strings.Builder
	for i, record := range records {
		if i == 0 || len(record) < 3 {
			continue
		}
		if strings.TrimSpace(record[2]) == "" {
			fmt.Fprintf(&b, "- %s skipped: owner value not supplied\n", record[0])
			continue
		}
		fmt.Fprintf(&b, "- %s owner_monthly_usd=%s\n", record[0], record[2])
	}
	return b.String()
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func formatOracleNumber(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func escapeCell(value string) string {
	return strings.ReplaceAll(value, "|", "/")
}

func paramString(params map[string]any, key string) (string, bool) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	return text, text != "" && text != "<nil>"
}

func paramFloat(params map[string]any, key string) (float64, bool) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch value := raw.(type) {
	case float64:
		return value, true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	default:
		parsed, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		return parsed, err == nil
	}
}

func paramBool(params map[string]any, key string) (bool, bool) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return false, false
	}
	switch value := raw.(type) {
	case bool:
		return value, true
	default:
		parsed, err := strconv.ParseBool(fmt.Sprint(value))
		return parsed, err == nil
	}
}
