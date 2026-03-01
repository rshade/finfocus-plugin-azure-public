package azureclient

import (
	"testing"
	"time"
)

func TestEscapeODataValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "no quotes", input: "eastus", want: "eastus"},
		{name: "single quote", input: "O'Brien", want: "O''Brien"},
		{name: "multiple quotes", input: "a'b'c", want: "a''b''c"},
		{name: "consecutive quotes", input: "a''b", want: "a''''b"},
		{name: "value with spaces", input: "Virtual Machines", want: "Virtual Machines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeODataValue(tt.input)
			if got != tt.want {
				t.Fatalf("escapeODataValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsBlank(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty string", input: "", want: true},
		{name: "spaces only", input: "   ", want: true},
		{name: "tabs only", input: "\t\t", want: true},
		{name: "mixed whitespace", input: " \t\n ", want: true},
		{name: "non blank value", input: "eastus", want: false},
		{name: "value with surrounding whitespace", input: "  eastus  ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlank(tt.input)
			if got != tt.want {
				t.Fatalf("isBlank(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConditionConstructors(t *testing.T) {
	tests := []struct {
		name string
		got  FilterCondition
		want FilterCondition
	}{
		{
			name: "Region",
			got:  Region("eastus"),
			want: FilterCondition{Field: "armRegionName", Value: "eastus"},
		},
		{
			name: "Service",
			got:  Service("Virtual Machines"),
			want: FilterCondition{Field: "serviceName", Value: "Virtual Machines"},
		},
		{
			name: "SKU",
			got:  SKU("Standard_B1s"),
			want: FilterCondition{Field: "armSkuName", Value: "Standard_B1s"},
		},
		{
			name: "PriceType",
			got:  PriceType("Consumption"),
			want: FilterCondition{Field: "priceType", Value: "Consumption"},
		},
		{
			name: "ProductName",
			got:  ProductName("Virtual Machines BS Series"),
			want: FilterCondition{Field: "productName", Value: "Virtual Machines BS Series"},
		},
		{
			name: "CurrencyCode",
			got:  CurrencyCode("USD"),
			want: FilterCondition{Field: "currencyCode", Value: "USD"},
		},
		{
			name: "Condition",
			got:  Condition("meterName", "B1s"),
			want: FilterCondition{Field: "meterName", Value: "B1s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s() = %#v, want %#v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestFilterBuilderSingleFieldFilters(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name:  "region",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("eastus") },
			want:  "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
		{
			name:  "service",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Service("Virtual Machines") },
			want:  "priceType eq 'Consumption' and serviceName eq 'Virtual Machines'",
		},
		{
			name:  "sku",
			build: func(b *FilterBuilder) *FilterBuilder { return b.SKU("Standard_B1s") },
			want:  "armSkuName eq 'Standard_B1s' and priceType eq 'Consumption'",
		},
		{
			name:  "product name",
			build: func(b *FilterBuilder) *FilterBuilder { return b.ProductName("Virtual Machines BS Series") },
			want:  "priceType eq 'Consumption' and productName eq 'Virtual Machines BS Series'",
		},
		{
			name:  "currency code",
			build: func(b *FilterBuilder) *FilterBuilder { return b.CurrencyCode("USD") },
			want:  "currencyCode eq 'USD' and priceType eq 'Consumption'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterBuilderMultiFieldFilters(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name: "2 fields",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Region("eastus").SKU("Standard_B1s")
			},
			want: "armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s' and priceType eq 'Consumption'",
		},
		{
			name: "3 fields",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Region("eastus").Service("Virtual Machines").SKU("Standard_B1s")
			},
			want: "armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s' and priceType eq 'Consumption' and serviceName eq 'Virtual Machines'",
		},
		{
			name: "all named fields",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.
					Region("eastus").
					SKU("Standard_B1s").
					Service("Virtual Machines").
					ProductName("Virtual Machines BS Series").
					CurrencyCode("USD")
			},
			want: "armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s' and currencyCode eq 'USD' and priceType eq 'Consumption' and productName eq 'Virtual Machines BS Series' and serviceName eq 'Virtual Machines'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterBuilderTypeDefaultAndOverride(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name:  "default consumption with criteria",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("eastus") },
			want:  "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
		{
			name:  "explicit override",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("eastus").Type("Reservation") },
			want:  "armRegionName eq 'eastus' and priceType eq 'Reservation'",
		},
		{
			name:  "empty type preserves default",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("eastus").Type("") },
			want:  "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
		{
			name:  "whitespace type preserves default",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("eastus").Type("  \t ") },
			want:  "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterBuilderFieldMethod(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name: "generic field with named field",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Region("eastus").Field("meterName", "B1s")
			},
			want: "armRegionName eq 'eastus' and meterName eq 'B1s' and priceType eq 'Consumption'",
		},
		{
			name: "empty name omitted",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Region("eastus").Field("", "B1s")
			},
			want: "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
		{
			name: "empty value omitted",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Region("eastus").Field("meterName", "")
			},
			want: "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
		{
			name: "same generic field included twice",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Field("meterName", "B2s").Field("meterName", "B1s")
			},
			want: "meterName eq 'B1s' and meterName eq 'B2s' and priceType eq 'Consumption'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterBuilderBuildMinimal(t *testing.T) {
	got := NewFilterBuilder().Build()
	want := "priceType eq 'Consumption'"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}

func TestFilterBuilderLastWriteWins(t *testing.T) {
	got := NewFilterBuilder().Region("eastus").Region("westus2").Build()
	want := "armRegionName eq 'westus2' and priceType eq 'Consumption'"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}

func TestFilterBuilderOrGroups(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name: "two regions in or group are parenthesized",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Or(Region("eastus"), Region("westus2"))
			},
			want: "(armRegionName eq 'eastus' or armRegionName eq 'westus2') and priceType eq 'Consumption'",
		},
		{
			name: "single valid or condition has no parentheses",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Or(Region("eastus"))
			},
			want: "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
		{
			name: "empty conditions inside or are omitted",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Or(Region(""), Condition("   ", "x"), Region("eastus"))
			},
			want: "armRegionName eq 'eastus' and priceType eq 'Consumption'",
		},
		{
			name: "all empty or group omitted",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Or(Region(""), Condition("", ""), Service(" \t "))
			},
			want: "priceType eq 'Consumption'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterBuilderMixedAndOr(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name: "or group mixed with and condition",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.
					Or(Region("eastus"), Region("westus2")).
					Service("Virtual Machines")
			},
			want: "(armRegionName eq 'eastus' or armRegionName eq 'westus2') and priceType eq 'Consumption' and serviceName eq 'Virtual Machines'",
		},
		{
			name: "multiple or groups are sorted with and",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.
					Or(Service("Virtual Machines"), Service("Storage")).
					Or(Region("eastus"), Region("westus2"))
			},
			want: "(armRegionName eq 'eastus' or armRegionName eq 'westus2') and priceType eq 'Consumption' and (serviceName eq 'Storage' or serviceName eq 'Virtual Machines')",
		},
		{
			name: "or group parenthesized when mixed with generic and",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.
					Field("meterName", "B1s").
					Or(CurrencyCode("USD"), CurrencyCode("EUR"))
			},
			want: "(currencyCode eq 'EUR' or currencyCode eq 'USD') and meterName eq 'B1s' and priceType eq 'Consumption'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterBuilderDeterministicOrdering(t *testing.T) {
	first := NewFilterBuilder().
		Region("eastus").
		Service("Virtual Machines").
		Or(SKU("Standard_B2s"), SKU("Standard_B1s")).
		Field("meterName", "B1s").
		Build()

	second := NewFilterBuilder().
		Field("meterName", "B1s").
		Or(SKU("Standard_B1s"), SKU("Standard_B2s")).
		Service("Virtual Machines").
		Region("eastus").
		Build()

	if first != second {
		t.Fatalf("expected deterministic output, got\nfirst:  %q\nsecond: %q", first, second)
	}

	got := NewFilterBuilder().
		Or(Service("Storage"), Service("Virtual Machines")).
		Or(Region("westus2"), Region("eastus")).
		Build()
	want := "(armRegionName eq 'eastus' or armRegionName eq 'westus2') and priceType eq 'Consumption' and (serviceName eq 'Storage' or serviceName eq 'Virtual Machines')"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}

func TestFilterBuilderFluentChaining(t *testing.T) {
	builder := NewFilterBuilder()
	if got := builder.Region("eastus"); got != builder {
		t.Fatal("Region() should return the same builder pointer")
	}
	if got := builder.Service("Virtual Machines"); got != builder {
		t.Fatal("Service() should return the same builder pointer")
	}
	if got := builder.SKU("Standard_B1s"); got != builder {
		t.Fatal("SKU() should return the same builder pointer")
	}
	if got := builder.ProductName("Virtual Machines BS Series"); got != builder {
		t.Fatal("ProductName() should return the same builder pointer")
	}
	if got := builder.CurrencyCode("USD"); got != builder {
		t.Fatal("CurrencyCode() should return the same builder pointer")
	}
	if got := builder.Field("meterName", "B1s"); got != builder {
		t.Fatal("Field() should return the same builder pointer")
	}
	if got := builder.Or(Region("eastus"), Region("westus2")); got != builder {
		t.Fatal("Or() should return the same builder pointer")
	}
	if got := builder.Type("Reservation"); got != builder {
		t.Fatal("Type() should return the same builder pointer")
	}

	got := NewFilterBuilder().
		Region("eastus").
		Service("Virtual Machines").
		SKU("Standard_B1s").
		Type("Reservation").
		Build()
	want := "armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s' and priceType eq 'Reservation' and serviceName eq 'Virtual Machines'"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}

func TestFilterBuilderEscapingInOutput(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name:  "single quote in region",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("O'Brien") },
			want:  "armRegionName eq 'O''Brien' and priceType eq 'Consumption'",
		},
		{
			name:  "multiple quotes in service",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Service("test'service'name") },
			want:  "priceType eq 'Consumption' and serviceName eq 'test''service''name'",
		},
		{
			name:  "quotes and spaces in product field",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Field("productName", "SQL Server O'Brien Edition") },
			want:  "priceType eq 'Consumption' and productName eq 'SQL Server O''Brien Edition'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterBuilderEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		build func(*FilterBuilder) *FilterBuilder
		want  string
	}{
		{
			name:  "whitespace region omitted",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("  ") },
			want:  "priceType eq 'Consumption'",
		},
		{
			name:  "empty generic field omitted",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Field("", "value") },
			want:  "priceType eq 'Consumption'",
		},
		{
			name: "all empty values still returns minimal filter",
			build: func(b *FilterBuilder) *FilterBuilder {
				return b.Region("").Service(" ").Type("  ").Field("", "").Or(Condition("", ""))
			},
			want: "priceType eq 'Consumption'",
		},
		{
			name:  "same named field set twice uses last value",
			build: func(b *FilterBuilder) *FilterBuilder { return b.Region("eastus").Region("westus2") },
			want:  "armRegionName eq 'westus2' and priceType eq 'Consumption'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build(NewFilterBuilder()).Build()
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}

	builder := NewFilterBuilder().
		Region("eastus").
		Service("Virtual Machines").
		Or(CurrencyCode("USD"), CurrencyCode("EUR"))
	first := builder.Build()
	second := builder.Build()
	if first != second {
		t.Fatalf("Build() should be idempotent: first=%q second=%q", first, second)
	}
}

func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		filter := NewFilterBuilder().
			Region("eastus").
			Service("Virtual Machines").
			SKU("Standard_B1s").
			ProductName("Virtual Machines BS Series").
			CurrencyCode("USD").
			Field("meterName", "B1s").
			Field("serviceFamily", "Compute").
			Field("skuName", "B1s").
			Or(Region("eastus"), Region("westus2")).
			Type("Reservation").
			Build()
		if filter == "" {
			b.Fatal("Build() returned empty output")
		}
	}

	elapsed := b.Elapsed()
	avg := elapsed / time.Duration(b.N)
	if avg >= time.Millisecond {
		b.Fatalf("average Build() latency %s exceeded 1ms target", avg)
	}
}
