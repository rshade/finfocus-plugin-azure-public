package pricing

import (
	"sort"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// RegionPrice is one region's selected Linux on-demand VM price.
// RegionPrice leaves Found false and Price at zero when no row is selected.
type RegionPrice struct {
	Region string
	Price  float64
	Found  bool
}

// SortRegionPrices orders found rows by retail price ascending, then region name.
// SortRegionPrices lists missing regions after those and does not report a selection miss.
func SortRegionPrices(pages map[string][]azureclient.PriceItem) []RegionPrice {
	out := make([]RegionPrice, 0, len(pages))
	for region, items := range pages {
		item, err := selectVMItem(items, false)
		if err != nil {
			out = append(out, RegionPrice{Region: region})
			continue
		}
		out = append(out, RegionPrice{
			Region: region,
			Price:  item.RetailPrice,
			Found:  true,
		})
	}
	sortRegionPrices(out)
	return out
}

func sortRegionPrices(out []RegionPrice) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Found != out[j].Found {
			return out[i].Found
		}
		if out[i].Found && out[i].Price != out[j].Price {
			return out[i].Price < out[j].Price
		}
		return out[i].Region < out[j].Region
	})
}
