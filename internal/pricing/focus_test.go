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
	window := focusWindow(hours)
	// Passed only by the test. Production has no billing-account argument.
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
			if record.GetServiceCategory() == finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_OTHER {
				t.Fatal("category must not be OTHER")
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
		"sql/Database":                       finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_DATABASE,
		"storage/BlobStorage":                finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE,
		"storage/ManagedDisk":                finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE,
		"storage/StorageAccount":             finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_STORAGE,
		"web/AppServicePlan":                 finfocusv1.FocusServiceCategory_FOCUS_SERVICE_CATEGORY_COMPUTE,
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
	if resourceType == "storage/StorageAccount" {
		sku = "Hot LRS"
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
	if record.GetPricingUnit() != "hour" || record.GetConsumedUnit() != "hour" {
		t.Fatalf("units = %q and %q, want hour", record.GetPricingUnit(), record.GetConsumedUnit())
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
	if record.GetProviderName() != "azure" || record.GetServiceProviderName() != "" {
		t.Fatalf("provider = %q, service provider = %q", record.GetProviderName(), record.GetServiceProviderName())
	}
	if record.GetInvoiceId() != "" || record.GetContractedUnitPrice() != 0 || record.GetContractedCost() != 0 {
		t.Fatal("invoice, contracted unit price, and contracted cost must stay unset")
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
