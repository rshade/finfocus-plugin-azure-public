package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	realPlanRelativeTolerance = 0.005
	realPlanAbsoluteTolerance = 0.01
	realPlanDir               = "testdata/pulumi-real"
)

// realPlanMustPass is the ratchet. AZ-7.2 and later add an id here only
// after that input set matches plan-expected.json.
func realPlanMustPass() map[string]bool {
	ids := []string{
		"azure-native/disk",
		"azure-native/linuxVm",
		"azure-native/vmNoLocation",
		"azure-native/windowsVm",
		"azure/disk",
		"azure/legacyVm",
		"azure/linuxVm",
		"azure/linuxVmRegular",
		"azure/plan",
		"azure/vmss",
		"azure/windowsVm",
		"azure/winPlan",
	}
	// Today's core view drops sku.capacity, so only the dotted and attributes
	// inputs have the worker count.
	structuredOnly := []string{"azure-native/plan"}
	out := make(map[string]bool, len(ids)*3+len(structuredOnly)*2)
	for _, id := range ids {
		out[id+"/core"] = true
		out[id+"/dotted"] = true
		out[id+"/attributes"] = true
	}
	for _, id := range structuredOnly {
		out[id+"/dotted"] = true
		out[id+"/attributes"] = true
	}
	return out
}

type realPlanCase struct {
	ID           string         `json:"id"`
	Provider     string         `json:"provider"`
	ResourceName string         `json:"resource_name"`
	TypeToken    string         `json:"type_token"`
	Expect       realPlanExpect `json:"expect"`
}

type realPlanExpect struct {
	Status          string                  `json:"status"`
	ExpectedMonthly *float64                `json:"expected_monthly"`
	Must            string                  `json:"must"`
	Notes           []string                `json:"notes"`
	Rows            []azureclient.PriceItem `json:"rows"`
}

type realPlanNotPriced struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	TypeToken string `json:"type_token"`
}

type realPlanFile struct {
	Cases     []realPlanCase      `json:"cases"`
	NotPriced []realPlanNotPriced `json:"not_priced"`
}

type coreViewEntry struct {
	Provider string            `json:"provider"`
	Region   string            `json:"region"`
	Sku      string            `json:"sku"`
	Tags     map[string]string `json:"tags"`
	Type     string            `json:"type"`
}

type realPlanVerdict struct {
	ID     string
	Input  string
	Status string
	Result string
}

func TestRealPulumiPlan(t *testing.T) {
	plan := loadRealPlan(t)
	views := loadCoreViews(t)
	previews := loadPreviewInputs(t)

	var verdicts []realPlanVerdict
	for _, tc := range plan.Cases {
		view, ok := views[tc.ID]
		if !ok {
			t.Errorf("case %s has no core-view entry", tc.ID)
			continue
		}
		verdicts = append(verdicts, judgeAndRecord(t, tc.ID, "core", tc.Expect, requestFromView(view)))
		dotted, err := dottedRequest(view, previews, tc)
		if err != nil {
			t.Errorf("case %s dotted inputs: %v", tc.ID, err)
			continue
		}
		verdicts = append(verdicts, judgeAndRecord(t, tc.ID, "dotted", tc.Expect, dotted))
		structured, err := attributesRequest(view, previews, tc)
		if err != nil {
			t.Errorf("case %s attributes: %v", tc.ID, err)
			continue
		}
		verdicts = append(verdicts, judgeAndRecord(t, tc.ID, "attributes", tc.Expect, structured))
	}
	for _, item := range plan.NotPriced {
		view, ok := views[item.ID]
		if !ok {
			t.Errorf("not_priced %s has no core-view entry", item.ID)
			continue
		}
		expect := realPlanExpect{Status: "unsupported_must_error", Must: item.Reason}
		verdicts = append(verdicts, judgeAndRecord(t, item.ID, "core", expect, requestFromView(view)))
	}

	writeRealPlanResults(t, verdicts)
	for _, verdict := range verdicts {
		key := verdict.ID + "/" + verdict.Input
		if realPlanMustPass()[key] && verdict.Result != "pass" {
			t.Errorf("%s regressed: %s", key, verdict.Result)
		}
	}
}

// With the v0.7.4 descriptor on GetActualCostRequest, actual cost over 730
// hours is the projected monthly cost for every case and input form, and both
// RPCs fail with the same code. Cloud tags that name pricing dimensions are
// sent beside the descriptor and must not change either result.
func TestRealPulumiPlan_ActualWithResource_MatchesProjected(t *testing.T) {
	plan := loadRealPlan(t)
	views := loadCoreViews(t)
	previews := loadPreviewInputs(t)
	cloudTags := map[string]string{"region": "prod-eu", "sku": "gold", "instances": "9"}

	for _, tc := range plan.Cases {
		view, ok := views[tc.ID]
		if !ok {
			t.Errorf("case %s has no core-view entry", tc.ID)
			continue
		}
		dotted, dottedErr := dottedRequest(view, previews, tc)
		structured, structuredErr := attributesRequest(view, previews, tc)
		if dottedErr != nil || structuredErr != nil {
			t.Errorf("case %s inputs: %v %v", tc.ID, dottedErr, structuredErr)
			continue
		}
		inputs := map[string]*finfocusv1.GetProjectedCostRequest{
			"core": requestFromView(view), "dotted": dotted, "attributes": structured,
		}
		for name, req := range inputs {
			calc := newPricingCalc(t, tc.Expect.Rows)
			projected, projErr := calc.GetProjectedCost(context.Background(), req)
			actual, actErr := calc.GetActualCost(context.Background(),
				actualWithResource(req.GetResource(), cloudTags, pluginsdk.HoursPerMonth))
			if status.Code(projErr) != status.Code(actErr) {
				t.Errorf("%s/%s: projected error %v, actual error %v", tc.ID, name, projErr, actErr)
				continue
			}
			if projErr != nil {
				if realPlanMustPass()[tc.ID+"/"+name] {
					t.Errorf("%s/%s must pass but both calls failed: %v", tc.ID, name, projErr)
				}
				continue
			}
			got := actual.GetResults()[0].GetCost()
			if math.Abs(got-projected.GetCostPerMonth()) > 1e-9 {
				t.Errorf("%s/%s: actual %v, projected %v", tc.ID, name, got, projected.GetCostPerMonth())
			}
		}
	}
}

func TestRealPlanDottedVMSize(t *testing.T) {
	previews := loadPreviewInputs(t)
	key := previewKey("azure-native:compute:VirtualMachine", "linuxVm")
	inputs := previews["azure-native"][key]
	tags := dottedTags(inputs)
	if tags["hardwareProfile.vmSize"] == "" {
		t.Fatal("hardwareProfile.vmSize missing from the native linuxVm preview")
	}
}

func judgeAndRecord(
	t *testing.T,
	id, input string,
	expect realPlanExpect,
	req *finfocusv1.GetProjectedCostRequest,
) realPlanVerdict {
	t.Helper()
	result := quoteRealPlan(t, expect.Rows, req)
	verdict := judgeRealPlan(expect, result.monthly, result.detail, result.err)
	t.Logf("%s %s %s: %s", id, input, expect.Status, verdict)
	return realPlanVerdict{ID: id, Input: input, Status: expect.Status, Result: verdict}
}

type realPlanQuote struct {
	monthly float64
	detail  string
	err     error
}

func quoteRealPlan(t *testing.T, rows []azureclient.PriceItem, req *finfocusv1.GetProjectedCostRequest) realPlanQuote {
	t.Helper()
	calc := newPricingCalc(t, rows)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	finfocusv1.RegisterCostSourceServiceServer(server, pluginsdk.NewServer(calc))
	go func() {
		_ = server.Serve(lis)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///"+lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	resp, err := finfocusv1.NewCostSourceServiceClient(conn).GetProjectedCost(context.Background(), req)
	if err != nil {
		return realPlanQuote{err: err}
	}
	return realPlanQuote{monthly: resp.GetCostPerMonth(), detail: resp.GetBillingDetail()}
}

func judgeRealPlan(expect realPlanExpect, monthly float64, detail string, err error) string {
	if err != nil {
		return judgeRealPlanError(expect.Status, err)
	}
	return judgeRealPlanPrice(expect, monthly, detail)
}

func judgeRealPlanError(expectStatus string, err error) string {
	message := strings.TrimSpace(status.Convert(err).Message())
	if message == "" {
		return "empty error"
	}
	if expectStatus == "ok" {
		return "error: " + message
	}
	if status.Code(err) == codes.OK {
		return "error has no status"
	}
	return "pass"
}

func judgeRealPlanPrice(expect realPlanExpect, monthly float64, detail string) string {
	switch expect.Status {
	case "ok":
		return judgeRealPlanOK(expect.ExpectedMonthly, monthly, detail, expect.Notes)
	case "unsupported_must_error":
		return fmt.Sprintf("priced %.6f", monthly)
	case "usage_required", "needs_parent_resource", "ambiguous":
		if monthly == 0 {
			return "zero cost"
		}
		if strings.TrimSpace(detail) == "" {
			return "price without a note"
		}
		return "price without the required note: " + expect.Must
	default:
		return "unknown status " + expect.Status
	}
}

func judgeRealPlanOK(want *float64, monthly float64, detail string, notes []string) string {
	if want == nil {
		return "missing expected_monthly"
	}
	tol := realPlanRelativeTolerance * math.Abs(*want)
	if tol < realPlanAbsoluteTolerance {
		tol = realPlanAbsoluteTolerance
	}
	if math.Abs(monthly-*want) > tol {
		return fmt.Sprintf("cost %.6f want %.6f", monthly, *want)
	}
	for _, note := range notes {
		if strings.TrimSpace(note) == "" {
			continue
		}
		if !strings.Contains(detail, note) {
			return "missing note"
		}
	}
	return "pass"
}

func requestFromView(view coreViewEntry) *finfocusv1.GetProjectedCostRequest {
	return &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     view.Provider,
			ResourceType: view.Type,
			Region:       view.Region,
			Sku:          view.Sku,
			Tags:         cloneTags(view.Tags),
		},
	}
}

func dottedRequest(
	view coreViewEntry,
	previews map[string]map[string]map[string]any,
	tc realPlanCase,
) (*finfocusv1.GetProjectedCostRequest, error) {
	inputs := previews[tc.Provider][previewKey(tc.TypeToken, tc.ResourceName)]
	if inputs == nil {
		return nil, errors.New("no preview inputs")
	}
	req := requestFromView(view)
	tags := req.GetResource().GetTags()
	for key, value := range dottedTags(inputs) {
		if _, exists := tags[key]; exists {
			continue
		}
		tags[key] = value
	}
	return req, nil
}

// attributesRequest is today's core view plus ResourceDescriptor.attributes
// built from the preview inputs after redactPreviewInputs.
func attributesRequest(
	view coreViewEntry,
	previews map[string]map[string]map[string]any,
	tc realPlanCase,
) (*finfocusv1.GetProjectedCostRequest, error) {
	inputs := previews[tc.Provider][previewKey(tc.TypeToken, tc.ResourceName)]
	if inputs == nil {
		return nil, errors.New("no preview inputs")
	}
	attrs, err := structpb.NewStruct(redactPreviewInputs(inputs))
	if err != nil {
		return nil, err
	}
	req := requestFromView(view)
	req.Resource.Attributes = attrs
	return req, nil
}

func dottedTags(inputs any) map[string]string {
	tags := map[string]string{}
	flattenRealInputs(inputs, "", tags)
	return tags
}

func flattenRealInputs(value any, prefix string, tags map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "__defaults" {
				continue
			}
			flattenRealInputs(child, joinRealKey(prefix, key), tags)
		}
	case []any:
		for i, child := range typed {
			flattenRealInputs(child, joinRealKey(prefix, strconv.Itoa(i)), tags)
		}
	default:
		if prefix == "" {
			return
		}
		tags[prefix] = realScalar(typed)
	}
}

func joinRealKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func realScalar(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func cloneTags(tags map[string]string) map[string]string {
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		out[key] = value
	}
	return out
}

func previewKey(typeToken, name string) string {
	return typeToken + "\n" + name
}

func loadRealPlan(t *testing.T) realPlanFile {
	t.Helper()
	var plan realPlanFile
	readRealJSON(t, filepath.Join(realPlanDir, "plan-expected.json"), &plan)
	if len(plan.Cases) == 0 {
		t.Fatal("plan-expected.json has no cases")
	}
	return plan
}

func loadCoreViews(t *testing.T) map[string]coreViewEntry {
	t.Helper()
	views := map[string]coreViewEntry{}
	readRealJSON(t, filepath.Join(realPlanDir, "core-view.json"), &views)
	return views
}

func loadPreviewInputs(t *testing.T) map[string]map[string]map[string]any {
	t.Helper()
	return map[string]map[string]map[string]any{
		"azure":        previewInputs(t, "preview-classic.json"),
		"azure-native": previewInputs(t, "preview-native.json"),
	}
}

func previewInputs(t *testing.T, name string) map[string]map[string]any {
	t.Helper()
	var preview struct {
		Steps []struct {
			NewState struct {
				Type   string         `json:"type"`
				URN    string         `json:"urn"`
				Inputs map[string]any `json:"inputs"`
			} `json:"newState"`
		} `json:"steps"`
	}
	readRealJSON(t, filepath.Join(realPlanDir, name), &preview)
	out := map[string]map[string]any{}
	for _, step := range preview.Steps {
		if step.NewState.Inputs == nil {
			continue
		}
		resourceName := step.NewState.URN
		if i := strings.LastIndex(resourceName, "::"); i >= 0 {
			resourceName = resourceName[i+2:]
		}
		out[previewKey(step.NewState.Type, resourceName)] = step.NewState.Inputs
	}
	return out
}

func readRealJSON(t *testing.T, path string, dest any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

func writeRealPlanResults(t *testing.T, verdicts []realPlanVerdict) {
	t.Helper()
	sort.Slice(verdicts, func(i, j int) bool {
		if verdicts[i].ID == verdicts[j].ID {
			return verdicts[i].Input < verdicts[j].Input
		}
		return verdicts[i].ID < verdicts[j].ID
	})
	var b strings.Builder
	b.WriteString("# Real plan results\n\n")
	b.WriteString("| Resource | Input | Status | Verdict |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, verdict := range verdicts {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n",
			verdict.ID, verdict.Input, verdict.Status, strings.ReplaceAll(verdict.Result, "|", "/"))
	}
	dir := filepath.Join("..", "..", ".superpowers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir results: %v", err)
	}
	path := filepath.Join(dir, "real-plan-results.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write results: %v", err)
	}
}

func TestRedactPreviewInputs_SpecRules_DropSecretsAndReferences(t *testing.T) {
	got := redactPreviewInputs(map[string]any{
		"__createBeforeDelete": true,
		"ref":                  "x",
		"ref.vnet.urn":         "urn",
		"location":             "westeurope",
		"osProfile": map[string]any{
			"adminPassword": "pw",
			"adminUsername": "azureuser",
			"nested":        []any{map[string]any{"__meta": 1, "apiKey": "k", "keep": "v"}},
		},
		"wrapped": map[string]any{
			pulumiSecretSignature: "1b47061264138c4ac30d75fd1eb44270",
			"value":               "hidden",
		},
		"tags": map[string]any{"env": "dev"},
	})
	want := map[string]any{
		"location": "westeurope",
		"osProfile": map[string]any{
			"adminUsername": "azureuser",
			"nested":        []any{map[string]any{"keep": "v"}},
		},
		"tags": map[string]any{"env": "dev"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("redactPreviewInputs() = %#v\nwant %#v", got, want)
	}
}

func TestAttributesRequest_NativeWindowsVM_OmitsAdminPassword(t *testing.T) {
	plan := loadRealPlan(t)
	views := loadCoreViews(t)
	previews := loadPreviewInputs(t)
	for _, tc := range plan.Cases {
		if tc.ID != "azure-native/windowsVm" {
			continue
		}
		req, err := attributesRequest(views[tc.ID], previews, tc)
		if err != nil {
			t.Fatalf("attributesRequest() error = %v", err)
		}
		if _, ok := pluginsdk.AttributeValue(req.GetResource().GetAttributes(), "osProfile.adminUsername"); !ok {
			t.Fatal("osProfile.adminUsername missing; the fixture changed")
		}
		if _, ok := pluginsdk.AttributeValue(req.GetResource().GetAttributes(), "osProfile.adminPassword"); ok {
			t.Fatal("osProfile.adminPassword is in the attributes")
		}
		return
	}
	t.Fatal("no azure-native/windowsVm case")
}

// pulumiSecretSignature marks a Pulumi secret-wrapped value.
const pulumiSecretSignature = "4dabf18193072939515e22adb298388d"

// redactPreviewInputs applies the host redaction rshade/finfocus spec 621
// requires before inputs become attributes (FR-004, FR-005): "__" and
// credential-like keys at any depth, the top-level ref key and ref.* keys,
// and any object wrapped as a Pulumi secret. The tags, tagsAll, labels, and
// annotations containers are kept. It is independent of the plugin's own
// skipAttributeSegment so the ratchet does not test the plugin against itself.
func redactPreviewInputs(inputs map[string]any) map[string]any {
	out := make(map[string]any, len(inputs))
	for key, value := range inputs {
		if key == "ref" || strings.HasPrefix(key, "ref.") {
			continue
		}
		if redacted, keep := redactPreviewValue(key, value); keep {
			out[key] = redacted
		}
	}
	return out
}

func redactPreviewValue(key string, value any) (any, bool) {
	if strings.HasPrefix(key, "__") || credentialLikeKey(key) {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		if _, secret := typed[pulumiSecretSignature]; secret {
			return nil, false
		}
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			if redacted, keep := redactPreviewValue(k, v); keep {
				out[k] = redacted
			}
		}
		return out, true
	case []any:
		out := make([]any, 0, len(typed))
		for _, v := range typed {
			if redacted, keep := redactPreviewValue("", v); keep {
				out = append(out, redacted)
			}
		}
		return out, true
	default:
		return value, true
	}
}

// credentialLikeKey is the segment list of rshade/finfocus spec 619
// (internal/engine/flatten.go skipDottedSegment).
func credentialLikeKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range []string{
		"password", "secret", "token", "credential", "ciphertext", "privatekey",
		"apikey", "api_key", "accesskey", "access_key", "connectionstring", "connection_string",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}
