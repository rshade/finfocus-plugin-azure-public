package azureclient

import "testing"

func TestCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query PriceQuery
		want  string
	}{
		{
			name: "normalizes case and trims whitespace",
			query: PriceQuery{
				ArmRegionName: " EastUS ",
				ArmSkuName:    " Standard_B1s ",
				ProductName:   " Virtual Machines BS Series ",
				ServiceName:   " Virtual Machines ",
				CurrencyCode:  " USD ",
			},
			want: "eastus|standard_b1s|virtual machines bs series|virtual machines|usd",
		},
		{
			name: "keeps empty fields as empty segments",
			query: PriceQuery{
				ArmRegionName: "westus2",
				ServiceName:   "Storage",
			},
			want: "westus2|||storage|",
		},
		{
			name: "uses canonical field order",
			query: PriceQuery{
				CurrencyCode:  "USD",
				ServiceName:   "Virtual Machines",
				ProductName:   "Product Name",
				ArmSkuName:    "Standard_B1s",
				ArmRegionName: "eastus",
			},
			want: "eastus|standard_b1s|product name|virtual machines|usd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := CacheKey(tc.query); got != tc.want {
				t.Fatalf("CacheKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCacheKeyEquivalentQueriesMatch(t *testing.T) {
	t.Parallel()

	singleQuery := PriceQuery{
		ArmRegionName: "EastUS",
		ArmSkuName:    "Standard_B1s",
		ProductName:   "Virtual Machines BS Series",
		ServiceName:   "Virtual Machines",
		CurrencyCode:  "USD",
	}

	batchEquivalent := PriceQuery{
		CurrencyCode:  " usd ",
		ServiceName:   " virtual machines ",
		ProductName:   " virtual machines bs series ",
		ArmSkuName:    " standard_b1s ",
		ArmRegionName: " eastus ",
	}

	singleKey := CacheKey(singleQuery)
	batchKey := CacheKey(batchEquivalent)
	if singleKey != batchKey {
		t.Fatalf("expected equivalent queries to have same key: %q != %q", singleKey, batchKey)
	}
}
