package pricing

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestSortRegionPrices(t *testing.T) {
	t.Parallel()

	t.Run("fixtures", func(t *testing.T) {
		t.Parallel()

		const missingRegion = "not-a-region"
		regions := []string{"eastus", "westus2", "northeurope", missingRegion}
		pages := make(map[string][]azureclient.PriceItem, len(regions))
		selected := make(map[string]azureclient.PriceItem, len(regions)-1)

		for _, region := range regions {
			page := loadRegionFixture(t, region)
			pages[region] = page.Items
			item, err := selectVMItem(page.Items, false)
			if region == missingRegion {
				if !errors.Is(err, azureclient.ErrNotFound) {
					t.Fatalf("selectVMItem(%s) = %+v, %v; want ErrNotFound", region, item, err)
				}
				continue
			}
			if err != nil {
				t.Fatalf("selectVMItem(%s): %v", region, err)
			}
			if item.RetailPrice == 0 {
				t.Fatalf("%s selected retail price is zero", region)
			}
			selected[region] = item
		}

		got := SortRegionPrices(pages)
		if len(got) != len(regions) {
			t.Fatalf("len(SortRegionPrices) = %d, want %d (%+v)", len(got), len(regions), got)
		}

		last := got[len(got)-1]
		if last.Region != missingRegion || last.Found || last.Price != 0 {
			t.Fatalf("last = %+v, want missing %s with price left at zero", last, missingRegion)
		}

		var previous float64
		for i, row := range got[:len(got)-1] {
			item, ok := selected[row.Region]
			if !ok {
				t.Fatalf("row %d region %q was not loaded", i, row.Region)
			}
			if !row.Found {
				t.Fatalf("%s Found = false, want true", row.Region)
			}
			if row.Price != item.RetailPrice {
				t.Fatalf("%s price = %v, want %v", row.Region, row.Price, item.RetailPrice)
			}
			if i > 0 && row.Price < previous {
				t.Fatalf("price decreased from %v to %v at %s", previous, row.Price, row.Region)
			}
			if i > 0 && row.Price == previous && row.Region < got[i-1].Region {
				t.Fatalf("equal prices %v ordered %s then %s", row.Price, got[i-1].Region, row.Region)
			}
			previous = row.Price
			delete(selected, row.Region)
		}
		if len(selected) != 0 {
			t.Fatalf("regions missing from result: %v", selected)
		}
	})

	t.Run("equal prices", func(t *testing.T) {
		t.Parallel()

		const price = 0.5
		pages := map[string][]azureclient.PriceItem{
			"westus2": {linuxOnDemandItem(price)},
			"eastus":  {linuxOnDemandItem(price)},
		}
		got := SortRegionPrices(pages)
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2 (%+v)", len(got), got)
		}
		if got[0].Region != "eastus" || got[1].Region != "westus2" {
			t.Fatalf("order = %s, %s; want eastus then westus2", got[0].Region, got[1].Region)
		}
		if !got[0].Found || !got[1].Found || got[0].Price != price || got[1].Price != price {
			t.Fatalf("rows = %+v, want both found at %v", got, price)
		}
	})
}

func loadRegionFixture(t *testing.T, region string) azureclient.PriceResponse {
	t.Helper()

	path := "testdata/retail/regions/standard_b1s_" + region + ".json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var resp azureclient.PriceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if resp.NextPageLink != "" {
		t.Fatalf("%s NextPageLink = %q, want one page", region, resp.NextPageLink)
	}
	return resp
}

func linuxOnDemandItem(price float64) azureclient.PriceItem {
	return azureclient.PriceItem{
		ProductName:   "Virtual Machines BS Series",
		SkuName:       "B1s",
		MeterName:     "B1s",
		RetailPrice:   price,
		UnitOfMeasure: "1 Hour",
		Type:          "Consumption",
	}
}
