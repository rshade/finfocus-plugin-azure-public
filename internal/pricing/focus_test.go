package pricing

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestGetActualCostFocusRecordOverGRPC(t *testing.T) {
	t.Parallel()

	const accountID = "ba-focus-test"
	calc := focusActualCalc(t)
	calc.SetBillingAccountID(accountID)
	client := dialPricingClient(t, calc)

	resp, err := client.GetActualCost(context.Background(), focusActualRequest())
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	record := resp.GetResults()[0].GetFocusRecord()
	if record == nil {
		t.Fatal("FocusRecord = nil, want a record when the plugin has a billing account id")
	}
	if err := pluginsdk.ValidateFocusRecord(record); err != nil {
		t.Fatalf("ValidateFocusRecord() = %v", err)
	}
	if record.GetBillingAccountId() != accountID {
		t.Fatalf("billing account id = %q, want %q", record.GetBillingAccountId(), accountID)
	}
}

func TestGetActualCostRequestBillingAccountOverGRPC(t *testing.T) {
	t.Parallel()

	const requestID = "ba-request"
	calc := focusActualCalc(t)
	calc.SetBillingAccountID("ba-process")
	client := dialPricingClient(t, calc)

	req := focusActualRequest()
	req.BillingAccountId = requestID
	resp, err := client.GetActualCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	record := resp.GetResults()[0].GetFocusRecord()
	if record == nil {
		t.Fatal("FocusRecord = nil, want the request billing account id")
	}
	if err := pluginsdk.ValidateFocusRecord(record); err != nil {
		t.Fatalf("ValidateFocusRecord() = %v", err)
	}
	if record.GetBillingAccountId() != requestID {
		t.Fatalf("billing account id = %q, want %q", record.GetBillingAccountId(), requestID)
	}
}

func TestGetActualCostRequestBillingAccountWithoutProcessOverGRPC(t *testing.T) {
	t.Parallel()

	client := dialPricingClient(t, focusActualCalc(t))
	req := focusActualRequest()
	req.BillingAccountId = "ba-request-only"
	resp, err := client.GetActualCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	record := resp.GetResults()[0].GetFocusRecord()
	if record == nil {
		t.Fatal("FocusRecord = nil, want a record from the request id")
	}
	if record.GetBillingAccountId() != "ba-request-only" {
		t.Fatalf("billing account id = %q", record.GetBillingAccountId())
	}
}

func TestGetActualCostDryRunIgnoresRequestBillingAccount(t *testing.T) {
	t.Parallel()

	calc := focusActualCalc(t)
	calc.SetBillingAccountID("ba-process")
	req := focusActualRequest()
	req.DryRun = true
	req.BillingAccountId = "ba-request"
	resp, err := calc.GetActualCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	record := resp.GetResults()[0].GetFocusRecord()
	if record == nil {
		t.Fatal("FocusRecord = nil, want the process id when dry run ignores the request id")
	}
	if record.GetBillingAccountId() != "ba-process" {
		t.Fatalf("billing account id = %q, want the process id", record.GetBillingAccountId())
	}
}

func TestGetActualCostOmitsFocusRecordWithoutAccountOverGRPC(t *testing.T) {
	t.Parallel()

	client := dialPricingClient(t, focusActualCalc(t))
	resp, err := client.GetActualCost(context.Background(), focusActualRequest())
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	result := resp.GetResults()[0]
	if result.GetFocusRecord() != nil {
		t.Fatal("FocusRecord = non-nil, want nil when no billing account id is configured")
	}
	if !strings.Contains(result.GetSource(), "confidence:") {
		t.Fatalf("source = %q, want the confidence fallback", result.GetSource())
	}
}

func focusActualCalc(t *testing.T) *Calculator {
	t.Helper()
	return newPricingCalc(t, []azureclient.PriceItem{{
		RetailPrice:   0.0104,
		UnitPrice:     0.0104,
		CurrencyCode:  "USD",
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
		MeterName:     "B1s",
		ProductName:   "Virtual Machines BS Series",
		UnitOfMeasure: "1 Hour",
		Type:          "Consumption",
	}})
}

func focusActualRequest() *finfocusv1.GetActualCostRequest {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return &finfocusv1.GetActualCostRequest{
		ResourceId: "vm-1",
		Tags: map[string]string{
			"region": "eastus",
			"sku":    "Standard_B1s",
		},
		Start: timestamppb.New(start),
		End:   timestamppb.New(start.Add(24 * time.Hour)),
	}
}

func TestFocusEmptyBillingAccountID(t *testing.T) {
	t.Parallel()

	_, err := buildFocusRecord(
		focusVMDescriptor(nil),
		focusQuote(0.0104*pluginsdk.HoursPerMonth, "USD", "vm detail"),
		focusWindow(24),
		"",
		"vm-1",
	)
	if err == nil || !strings.Contains(err.Error(), "billing_account_id") {
		t.Fatalf("empty billing account error = %v, want billing_account_id", err)
	}

	calc := newPricingCalc(t, []azureclient.PriceItem{{
		RetailPrice:  0.0104,
		CurrencyCode: "USD",
	}})
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	resp, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		ResourceId: "vm-1",
		Tags: map[string]string{
			"region": "eastus",
			"sku":    "Standard_B1s",
		},
		Start: timestamppb.New(start),
		End:   timestamppb.New(start.Add(24 * time.Hour)),
	})
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	result := resp.GetResults()[0]
	want := 0.0104 * 24
	if math.Abs(result.GetCost()-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", result.GetCost(), want)
	}
	if result.GetFocusRecord() != nil {
		t.Fatal("FocusRecord = non-nil, want nil when the request has no billing account id")
	}
	if result.GetSource() != "azure-retail-prices[confidence:HIGH]" {
		t.Fatalf("source = %q", result.GetSource())
	}
}

func TestFocusRecordMatchesActualCost(t *testing.T) {
	t.Parallel()

	const hours = 24.0
	monthly := 0.0104 * pluginsdk.HoursPerMonth
	quote := focusQuote(monthly, "USD", "vm detail")
	quote.meters = []quoteMeter{{key: breakdownCompute, price: 0.0104, unit: "1 Hour"}}
	window := focusWindow(hours)
	// The RPC resolves this from the request or the process setting.
	billingAccountID := "ba-focus-test"

	record, err := buildFocusRecord(focusVMDescriptor(nil), quote, window, billingAccountID, "vm-1")
	if err != nil {
		t.Fatalf("buildFocusRecord() failed: %v", err)
	}
	if err := pluginsdk.ValidateFocusRecord(record); err != nil {
		t.Fatalf("ValidateFocusRecord() = %v", err)
	}

	want := monthly * (hours / pluginsdk.HoursPerMonth)
	assertFocusCosts(t, record, want)
	assertFocusQuantity(t, record, hours, want)
	assertFocusIdentity(t, record, billingAccountID)
	assertFocusPeriods(t, record, window.start, hours)
	assertFocusClassification(t, record)

	fallback, err := buildFocusRecord(
		focusVMDescriptor(nil),
		focusQuote(monthly, "USD", ""),
		window,
		billingAccountID,
		"",
	)
	if err != nil {
		t.Fatalf("empty detail buildFocusRecord() failed: %v", err)
	}
	if fallback.GetChargeDescription() != "compute/VirtualMachine" {
		t.Fatalf("empty detail description = %q", fallback.GetChargeDescription())
	}
	if fallback.GetResourceId() != "" {
		t.Fatalf("empty resource id = %q", fallback.GetResourceId())
	}
}

func TestFocusServiceCategory(t *testing.T) {
	t.Parallel()

	categories := focusCategoryCases()
	types := SupportedResourceTypes()
	if len(types) != len(categories) {
		t.Fatalf("SupportedResourceTypes() = %v, category table has %d", types, len(categories))
	}

	for _, resourceType := range types {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()

			want, ok := categories[resourceType]
			if !ok {
				t.Fatalf("no category for %s", resourceType)
			}
			record, err := buildFocusRecord(
				focusTypedDescriptor(resourceType),
				focusQuote(1, "USD", "detail"),
				focusWindow(1),
				"ba-focus-test",
				"",
			)
			if err != nil {
				t.Fatalf("buildFocusRecord(%s) failed: %v", resourceType, err)
			}
			if record.GetServiceCategory() != want {
				t.Fatalf("category = %s, want %s", record.GetServiceCategory(), want)
			}
			if record.GetServiceCategory() == finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER &&
				record.GetExtendedColumns()[focusExtServiceCategory] == "" {
				t.Fatal("category OTHER must carry the FOCUS category in x_ServiceCategory")
			}
		})
	}
}

func TestFocusSpotPricingCategory(t *testing.T) {
	t.Parallel()

	const hours = 24.0
	monthly := 0.0104 * pluginsdk.HoursPerMonth
	quote := focusQuote(monthly, "USD", "spot detail")
	window := focusWindow(hours)
	want := monthly * (hours / pluginsdk.HoursPerMonth)

	standard, err := buildFocusRecord(focusVMDescriptor(nil), quote, window, "ba-focus-test", "")
	if err != nil {
		t.Fatalf("standard buildFocusRecord() failed: %v", err)
	}
	spot, err := buildFocusRecord(
		focusVMDescriptor(map[string]string{"priority": "Spot"}),
		quote,
		window,
		"ba-focus-test",
		"",
	)
	if err != nil {
		t.Fatalf("spot buildFocusRecord() failed: %v", err)
	}
	if spot.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC {
		t.Fatalf("spot pricing category = %s", spot.GetPricingCategory())
	}
	if standard.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
		t.Fatalf("standard pricing category = %s", standard.GetPricingCategory())
	}
	assertFocusCosts(t, spot, want)
	assertFocusCosts(t, standard, want)
	if spot.GetBilledCost() != standard.GetBilledCost() {
		t.Fatalf("spot cost %v changed from %v", spot.GetBilledCost(), standard.GetBilledCost())
	}
}

func TestFocusNativeFunctionWebApp(t *testing.T) {
	t.Parallel()

	record, err := buildFocusRecord(
		&finfocusv1.ResourceDescriptor{
			Provider:     "azure-native",
			ResourceType: "azure-native:web:WebApp",
			Region:       "eastus",
			Tags:         map[string]string{"kind": "FunctionApp"},
		},
		focusQuote(1.8, "USD", "function detail"),
		focusWindow(pluginsdk.HoursPerMonth),
		"ba-focus-test",
		"fn-native",
	)
	if err != nil {
		t.Fatalf("buildFocusRecord() failed: %v", err)
	}
	if record.GetServiceCategory() != finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE {
		t.Fatalf("category = %s, want COMPUTE", record.GetServiceCategory())
	}

	bare, err := buildFocusRecord(
		&finfocusv1.ResourceDescriptor{
			Provider:     "azure-native",
			ResourceType: "azure-native:web:WebApp",
			Region:       "eastus",
		},
		focusQuote(1.8, "USD", "site detail"),
		focusWindow(pluginsdk.HoursPerMonth),
		"ba-focus-test",
		"site",
	)
	if err == nil || bare != nil {
		t.Fatalf("bare web app record = %v, err = %v, want an error and no record", bare, err)
	}
}

func TestFocusUnknownType(t *testing.T) {
	t.Parallel()

	const resourceType = "compute/VirtualMachineScaleSet"
	record, err := buildFocusRecord(
		&finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: resourceType,
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
		focusQuote(1, "USD", "detail"),
		focusWindow(1),
		"ba-focus-test",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), resourceType) {
		t.Fatalf("error = %v, want it to name %s", err, resourceType)
	}
	if record != nil && record.GetServiceCategory() == finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER {
		t.Fatal("unknown type must not default to OTHER")
	}
}

func TestFocusEmptyCurrency(t *testing.T) {
	t.Parallel()

	record, err := buildFocusRecord(
		focusVMDescriptor(nil),
		focusQuote(1, "", "detail"),
		focusWindow(1),
		"ba-focus-test",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "billing_currency") {
		t.Fatalf("empty currency error = %v, want builder billing_currency error", err)
	}
	if record != nil && record.GetBillingCurrency() == "USD" {
		t.Fatal("empty currency must not be replaced with USD")
	}
}

func focusCategoryCases() map[string]finfocusv1.FocusServiceCategory {
	return map[string]finfocusv1.FocusServiceCategory{
		"compute/VirtualMachine":             finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
		"containerservice/KubernetesCluster": finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
		"cosmosdb/Account":                   finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE,
		"network/LoadBalancer":               finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_NETWORK,
		"sql/Database":                       finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE,
		"storage/BlobStorage":                finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE,
		"storage/ManagedDisk":                finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
		"storage/StorageAccount":             finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE,
		"web/AppServicePlan":                 finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER,
		"web/FunctionApp":                    finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
	}
}

func focusVMDescriptor(tags map[string]string) *finfocusv1.ResourceDescriptor {
	return &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Region:       "eastus",
		Sku:          "Standard_B1s",
		Tags:         tags,
	}
}

func focusTypedDescriptor(resourceType string) *finfocusv1.ResourceDescriptor {
	sku := "Standard_B1s"
	switch resourceType {
	case "storage/StorageAccount":
		sku = "Hot LRS"
	case canonicalKubernetesCluster:
		sku = aksLabelStandard
	case "web/AppServicePlan":
		sku = "B1"
	case "web/FunctionApp":
		sku = "Y1"
	}
	return &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: resourceType,
		Region:       "eastus",
		Sku:          sku,
	}
}

func focusQuote(monthly float64, currency, detail string) monthlyQuote {
	return monthlyQuote{
		monthly:       monthly,
		currency:      currency,
		billingDetail: detail,
		region:        "eastus",
		sku:           "Standard_B1s",
		resourceType:  "compute/VirtualMachine",
	}
}

func focusWindow(hours float64) actualWindow {
	return actualWindow{
		start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		hours: hours,
	}
}

func assertFocusCosts(t *testing.T, record *finfocusv1.FocusCostRecord, want float64) {
	t.Helper()

	if math.Abs(record.GetBilledCost()-want) > 1e-9 ||
		math.Abs(record.GetEffectiveCost()-want) > 1e-9 ||
		math.Abs(record.GetListCost()-want) > 1e-9 {
		t.Fatalf("costs billed=%v effective=%v list=%v, want %v",
			record.GetBilledCost(),
			record.GetEffectiveCost(),
			record.GetListCost(),
			want,
		)
	}
}

func assertFocusQuantity(t *testing.T, record *finfocusv1.FocusCostRecord, hours, want float64) {
	t.Helper()

	if math.Abs(record.GetListUnitPrice()*record.GetPricingQuantity()-want) > 1e-9 {
		t.Fatalf(
			"list unit price %v * quantity %v is not %v",
			record.GetListUnitPrice(),
			record.GetPricingQuantity(),
			want,
		)
	}
	if record.GetPricingQuantity() != hours || record.GetConsumedQuantity() != hours {
		t.Fatalf("quantities = %v and %v, want %v", record.GetPricingQuantity(), record.GetConsumedQuantity(), hours)
	}
	if record.GetPricingUnit() != focusUnitHours || record.GetConsumedUnit() != focusUnitHours {
		t.Fatalf("units = %q and %q, want %s", record.GetPricingUnit(), record.GetConsumedUnit(), focusUnitHours)
	}
	if record.GetBillingCurrency() != "USD" {
		t.Fatalf("currency = %q", record.GetBillingCurrency())
	}
}

func assertFocusIdentity(t *testing.T, record *finfocusv1.FocusCostRecord, billingAccountID string) {
	t.Helper()

	if record.GetChargeDescription() != "vm detail" {
		t.Fatalf("charge description = %q", record.GetChargeDescription())
	}
	if record.GetServiceName() != "Virtual Machines" {
		t.Fatalf("service name = %q", record.GetServiceName())
	}
	if record.GetRegionId() != "eastus" || record.GetRegionName() != "eastus" || record.GetSkuId() != "Standard_B1s" {
		t.Fatalf("location/sku = %s %s %s", record.GetRegionId(), record.GetRegionName(), record.GetSkuId())
	}
	if record.GetResourceId() != "vm-1" {
		t.Fatalf("resource id = %q", record.GetResourceId())
	}
	if record.GetBillingAccountId() != billingAccountID {
		t.Fatalf("billing account id = %q", record.GetBillingAccountId())
	}
	if record.GetInvoiceId() != "" {
		t.Fatalf("invoice id = %q, want unset", record.GetInvoiceId())
	}
	if record.GetCommitmentDiscountId() != "" || record.GetCommitmentDiscountType() != "" {
		t.Fatal("commitment discount must stay unset")
	}
}

func assertFocusPeriods(t *testing.T, record *finfocusv1.FocusCostRecord, start time.Time, hours float64) {
	t.Helper()

	end := start.Add(time.Duration(hours * float64(time.Hour)))
	if !record.GetBillingPeriodStart().AsTime().Equal(start) ||
		!record.GetBillingPeriodEnd().AsTime().Equal(end) ||
		!record.GetChargePeriodStart().AsTime().Equal(start) ||
		!record.GetChargePeriodEnd().AsTime().Equal(end) {
		t.Fatalf("periods = %v-%v charge %v-%v",
			record.GetBillingPeriodStart().AsTime(),
			record.GetBillingPeriodEnd().AsTime(),
			record.GetChargePeriodStart().AsTime(),
			record.GetChargePeriodEnd().AsTime(),
		)
	}
}

func assertFocusClassification(t *testing.T, record *finfocusv1.FocusCostRecord) {
	t.Helper()

	if record.GetChargeCategory() != finfocusv1.FocusChargeCategory_FOCUS_CHARGE_CATEGORY_USAGE ||
		record.GetChargeClass() != finfocusv1.FocusChargeClass_FOCUS_CHARGE_CLASS_REGULAR ||
		record.GetChargeFrequency() != finfocusv1.FocusChargeFrequency_FOCUS_CHARGE_FREQUENCY_USAGE_BASED ||
		record.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
		t.Fatalf("charge classification = %s %s %s pricing %s",
			record.GetChargeCategory(),
			record.GetChargeClass(),
			record.GetChargeFrequency(),
			record.GetPricingCategory(),
		)
	}
}

func TestBuildFocusRecord_FOCUS13Providers_SetsMandatoryNames(t *testing.T) {
	t.Parallel()

	const accountID = "ba-focus-test"
	record, err := buildFocusRecord(
		focusVMDescriptor(nil),
		focusQuote(0.0104*pluginsdk.HoursPerMonth, "USD", "vm detail"),
		focusWindow(24),
		accountID,
		"vm-1",
	)
	if err != nil {
		t.Fatalf("buildFocusRecord() failed: %v", err)
	}

	names := map[string]string{
		"service_provider_name": record.GetServiceProviderName(),
		"host_provider_name":    record.GetHostProviderName(),
		"invoice_issuer":        record.GetInvoiceIssuer(),
		"provider_name":         record.GetProviderName(),
		"publisher":             record.GetPublisher(),
	}
	for field, got := range names {
		if got != focusProviderMicrosoft {
			t.Errorf("%s = %q, want %q", field, got, focusProviderMicrosoft)
		}
	}
	if record.GetBillingAccountName() != accountID {
		t.Errorf("billing_account_name = %q, want the account id %q", record.GetBillingAccountName(), accountID)
	}
}

func TestFocusPricingBasis_Quotes_ReturnMeterOrWindowBasis(t *testing.T) {
	t.Parallel()

	const hours = 24.0
	vmHourly := 0.0104
	diskMonthly := 19.71
	months := hours / pluginsdk.HoursPerMonth
	tests := []struct {
		name      string
		meters    []quoteMeter
		monthly   float64
		wantQty   float64
		wantUnit  string
		wantPrice float64
	}{
		{
			name:      "single hourly meter",
			meters:    []quoteMeter{{key: breakdownCompute, price: vmHourly, unit: "1 Hour"}},
			monthly:   vmHourly * pluginsdk.HoursPerMonth,
			wantQty:   hours,
			wantUnit:  focusUnitHours,
			wantPrice: vmHourly,
		},
		{
			name:      "plural hours unit",
			meters:    []quoteMeter{{key: breakdownCompute, price: vmHourly, unit: "1 Hours"}},
			monthly:   vmHourly * pluginsdk.HoursPerMonth,
			wantQty:   hours,
			wantUnit:  focusUnitHours,
			wantPrice: vmHourly,
		},
		{
			name:      "scale set of three bills instance hours",
			meters:    []quoteMeter{{key: breakdownCompute, price: vmHourly, unit: "1 Hour", count: 3}},
			monthly:   vmHourly * pluginsdk.HoursPerMonth * 3,
			wantQty:   hours * 3,
			wantUnit:  focusUnitHours,
			wantPrice: vmHourly,
		},
		{
			name:      "per month meter bills units per month",
			meters:    []quoteMeter{{key: breakdownStorage, price: diskMonthly, unit: "1/Month"}},
			monthly:   diskMonthly,
			wantQty:   months,
			wantUnit:  focusUnitUnitsPerMonth,
			wantPrice: diskMonthly,
		},
		{
			name:      "one month meter bills months",
			meters:    []quoteMeter{{key: breakdownStorage, price: diskMonthly, unit: "1 Month"}},
			monthly:   diskMonthly,
			wantQty:   months,
			wantUnit:  focusUnitMonths,
			wantPrice: diskMonthly,
		},
		{
			name:     "monthly total that is not the meter uses the window",
			meters:   []quoteMeter{{key: breakdownCompute, price: vmHourly, unit: "1 Hour"}},
			monthly:  vmHourly * pluginsdk.HoursPerMonth * 2,
			wantQty:  hours,
			wantUnit: focusUnitHours,
		},
		{
			name: "several meters use the window",
			meters: []quoteMeter{
				{key: "control_plane", price: 0.10, unit: "1 Hour"},
				{key: "node_pool_pool_1", price: 0.096, unit: "1 Hour"},
			},
			monthly:  273,
			wantQty:  hours,
			wantUnit: focusUnitHours,
		},
		{
			name:     "zero price meter uses the window",
			meters:   []quoteMeter{{key: breakdownCompute, price: 0, unit: "1 Hour"}},
			wantQty:  hours,
			wantUnit: focusUnitHours,
		},
		{
			name:     "gb month meter uses the window",
			meters:   []quoteMeter{{key: breakdownStorage, price: 0.0208, unit: "1 GB/Month"}},
			monthly:  2.08,
			wantQty:  hours,
			wantUnit: focusUnitHours,
		},
		{
			name:     "per hour request unit meter uses the window",
			meters:   []quoteMeter{{key: "ru", price: 0.008, unit: "1/Hour"}},
			monthly:  0.008 * 4 * pluginsdk.HoursPerMonth,
			wantQty:  hours,
			wantUnit: focusUnitHours,
		},
		{
			name:     "no meters use the window",
			monthly:  73,
			wantQty:  hours,
			wantUnit: focusUnitHours,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			qty, unit, price := focusPricingBasis(tt.meters, tt.monthly, hours)
			if unit != tt.wantUnit {
				t.Errorf("unit = %q, want %q", unit, tt.wantUnit)
			}
			if qty != tt.wantQty {
				t.Errorf("quantity = %v, want exactly %v", qty, tt.wantQty)
			}
			if price != tt.wantPrice {
				t.Errorf("unit price = %v, want %v", price, tt.wantPrice)
			}
		})
	}
}

func TestBuildFocusRecord_CostColumns_AgreeWithUnitPrice(t *testing.T) {
	t.Parallel()

	const hours = 24.0
	diskMonthly := 19.71
	quote := focusQuote(diskMonthly, "USD", "disk detail")
	quote.meters = []quoteMeter{{key: breakdownStorage, price: diskMonthly, unit: "1/Month"}}
	record, err := buildFocusRecord(
		focusTypedDescriptor("storage/ManagedDisk"),
		quote,
		focusWindow(hours),
		"ba-focus-test",
		"disk-1",
	)
	if err != nil {
		t.Fatalf("buildFocusRecord() failed: %v", err)
	}

	cost := diskMonthly * hours / pluginsdk.HoursPerMonth
	assertFocusCosts(t, record, cost)
	if math.Abs(record.GetContractedCost()-cost) > 1e-9 {
		t.Errorf("contracted cost = %v, want %v", record.GetContractedCost(), cost)
	}
	if record.GetContractedUnitPrice() != record.GetListUnitPrice() || record.GetListUnitPrice() != diskMonthly {
		t.Errorf("unit prices contracted=%v list=%v, want %v",
			record.GetContractedUnitPrice(), record.GetListUnitPrice(), diskMonthly)
	}
	if record.GetPricingUnit() != focusUnitUnitsPerMonth || record.GetConsumedUnit() != focusUnitUnitsPerMonth {
		t.Errorf(
			"units = %q and %q, want %s",
			record.GetPricingUnit(),
			record.GetConsumedUnit(),
			focusUnitUnitsPerMonth,
		)
	}
	qty := record.GetPricingQuantity()
	if math.Abs(record.GetListUnitPrice()*qty-record.GetListCost()) > 1e-9 ||
		math.Abs(record.GetContractedUnitPrice()*qty-record.GetContractedCost()) > 1e-9 {
		t.Errorf("unit price * quantity %v does not match list %v or contracted %v",
			record.GetListUnitPrice()*qty, record.GetListCost(), record.GetContractedCost())
	}
	if record.GetConsumedQuantity() != qty {
		t.Errorf("consumed quantity = %v, want pricing quantity %v", record.GetConsumedQuantity(), qty)
	}
}

func TestFocusServiceClass_EverySupportedType_MatchesMicrosoftMapping(t *testing.T) {
	t.Parallel()

	compute := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE
	storage := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE
	database := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE
	network := finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_NETWORK
	vms := focusService{name: "Virtual Machines", category: compute, subcategory: "Virtual Machines"}
	scaleSets := focusService{name: "Virtual Machine Scale Sets", category: compute, subcategory: "Virtual Machines"}
	appService := focusService{
		name:           "Azure App Service",
		category:       finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER,
		subcategory:    "Other (Other)",
		rawCategory:    "Web",
		rawSubcategory: "Application Platforms",
	}
	functions := focusService{name: "Functions", category: compute, subcategory: "Serverless Compute"}
	accounts := focusService{name: "Storage Accounts", category: storage, subcategory: "Storage Platforms"}
	tests := []struct {
		name         string
		resourceType string
		tags         map[string]string
		sku          string
		want         focusService
	}{
		{resourceType: "compute/VirtualMachine", want: vms},
		{resourceType: "azure:compute/linuxVirtualMachineScaleSet:LinuxVirtualMachineScaleSet", want: scaleSets},
		{resourceType: "azure:compute/windowsVirtualMachine:WindowsVirtualMachine", want: vms},
		{resourceType: "azure-native:compute:VirtualMachineScaleSet", want: scaleSets},
		{resourceType: "storage/ManagedDisk", want: vms},
		{resourceType: "storage/StorageAccount", want: accounts},
		{resourceType: "storage/BlobStorage", want: accounts},
		{resourceType: "web/AppServicePlan", want: appService},
		{resourceType: "web/FunctionApp", want: functions},
		{name: "function app on a plan", resourceType: "web/FunctionApp", sku: "B1", want: appService},
		{
			resourceType: "azure-native:web:WebApp",
			tags:         map[string]string{"kind": "FunctionApp"},
			want:         functions,
		},
		{
			resourceType: "containerservice/KubernetesCluster",
			want:         focusService{name: "Azure Kubernetes Service", category: compute, subcategory: "Containers"},
		},
		{
			resourceType: "sql/Database",
			want: focusService{
				name:        "Azure SQL Database",
				category:    database,
				subcategory: "Relational Databases",
			},
		},
		{
			resourceType: "cosmosdb/Account",
			want:         focusService{name: "Cosmos DB", category: database, subcategory: "NoSQL Databases"},
		},
		{
			resourceType: "network/LoadBalancer",
			want:         focusService{name: "Load Balancer", category: network, subcategory: "Application Networking"},
		},
	}

	covered := map[string]bool{}
	for _, tt := range tests {
		covered[tt.resourceType] = true
		name := tt.name
		if name == "" {
			name = tt.resourceType
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := focusServiceClass(tt.resourceType, tt.tags, tt.sku)
			if err != nil {
				t.Fatalf("focusServiceClass() failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("service = %+v, want %+v", got, tt.want)
			}
			if parent, ok := focus13SubcategoryParents()[got.subcategory]; !ok || parent != got.category {
				t.Fatalf("subcategory %q is not a FOCUS 1.3 child of %s", got.subcategory, got.category)
			}
		})
	}
	for _, resourceType := range SupportedResourceTypes() {
		if !covered[resourceType] {
			t.Errorf("no service class case for supported type %s", resourceType)
		}
	}
}

func TestBuildFocusRecord_EverySupportedType_PassesAggregateValidation(t *testing.T) {
	t.Parallel()

	for _, resourceType := range SupportedResourceTypes() {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()

			quote := focusQuote(73, "USD", "detail")
			quote.meters = []quoteMeter{{key: breakdownCompute, price: 0.1, unit: "1 Hour"}}
			record, err := buildFocusRecord(
				focusTypedDescriptor(resourceType),
				quote,
				focusWindow(24),
				"ba-focus-test",
				"res-1",
			)
			if err != nil {
				t.Fatalf("buildFocusRecord(%s) failed: %v", resourceType, err)
			}
			errs := pluginsdk.ValidateFocusRecordWithOptions(record, pluginsdk.ValidationOptions{
				Mode: pluginsdk.ValidationModeAggregate,
			})
			if len(errs) != 0 {
				t.Fatalf("validation errors = %v", errs)
			}
			if record.GetServiceSubcategory() == "" {
				t.Fatal("service_subcategory is empty")
			}
		})
	}
}

func TestBuildFocusRecord_CategoryOutsideEnum_CarriesFocusValuesInExtendedColumns(t *testing.T) {
	t.Parallel()

	web := map[string]string{
		focusExtServiceCategory:    "Web",
		focusExtServiceSubcategory: "Application Platforms",
	}
	tests := []struct {
		name         string
		resourceType string
		sku          string
		wantCategory finfocusv1.FocusServiceCategory
		wantSub      string
		wantColumns  map[string]string
	}{
		{
			name:         "app service plan is web",
			resourceType: "web/AppServicePlan",
			sku:          "P1v3",
			wantCategory: finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER,
			wantSub:      "Other (Other)",
			wantColumns:  web,
		},
		{
			name:         "function app on a plan is web",
			resourceType: "web/FunctionApp",
			sku:          "B1",
			wantCategory: finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER,
			wantSub:      "Other (Other)",
			wantColumns:  web,
		},
		{
			name:         "virtual machine stays in the enum",
			resourceType: "compute/VirtualMachine",
			sku:          "Standard_B1s",
			wantCategory: finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
			wantSub:      "Virtual Machines",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			descriptor := focusTypedDescriptor(tt.resourceType)
			descriptor.Sku = tt.sku
			quote := focusQuote(73, "USD", "detail")
			record, err := buildFocusRecord(descriptor, quote, focusWindow(24), "ba-1", "res-1")
			if err != nil {
				t.Fatalf("buildFocusRecord() failed: %v", err)
			}
			if record.GetServiceCategory() != tt.wantCategory || record.GetServiceSubcategory() != tt.wantSub {
				t.Fatalf("category = %s / %q, want %s / %q",
					record.GetServiceCategory(), record.GetServiceSubcategory(), tt.wantCategory, tt.wantSub)
			}
			for _, key := range []string{focusExtServiceCategory, focusExtServiceSubcategory} {
				if got, want := record.GetExtendedColumns()[key], tt.wantColumns[key]; got != want {
					t.Errorf("extended column %s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

// focus13SubcategoryParents lists the FOCUS 1.3 ServiceSubcategory allowed
// values this plugin uses, with their only parent ServiceCategory.
func focus13SubcategoryParents() map[string]finfocusv1.FocusServiceCategory {
	return map[string]finfocusv1.FocusServiceCategory{
		"Virtual Machines":       finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
		"Serverless Compute":     finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
		"Containers":             finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
		"Other (Other)":          finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER,
		"Storage Platforms":      finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE,
		"Relational Databases":   finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE,
		"NoSQL Databases":        finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE,
		"Application Networking": finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_NETWORK,
	}
}
