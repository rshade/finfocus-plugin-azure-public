package pricing

import (
	"fmt"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const vmPrioritySpot = "Spot"

// descriptorSpot reports whether the descriptor tag priority selects Spot.
// An empty priority is on-demand. Any other non-empty value is InvalidArgument
// and the message names that value.
func descriptorSpot(resource *finfocusv1.ResourceDescriptor) (bool, error) {
	if resource == nil {
		return false, nil
	}
	return prioritySpot(resource.GetTags()["priority"])
}

// prioritySpot reports whether raw selects Spot. An empty value is on-demand.
// Any other non-empty value is InvalidArgument and the message names it.
func prioritySpot(raw string) (bool, error) {
	priority := strings.TrimSpace(raw)
	if priority == "" || priority == "<nil>" {
		return false, nil
	}
	if strings.EqualFold(priority, vmPrioritySpot) {
		return true, nil
	}
	return false, status.Errorf(codes.InvalidArgument, "unsupported priority %q", priority)
}

// estimateSpot reads the EstimateCost attribute priority. The same values as
// the descriptor tag apply: empty is on-demand, Spot is Spot.
func estimateSpot(req *finfocusv1.EstimateCostRequest) (bool, error) {
	if req == nil || req.GetAttributes() == nil {
		return false, nil
	}
	raw, ok := req.GetAttributes().AsMap()["priority"]
	if !ok || raw == nil {
		return false, nil
	}
	return prioritySpot(fmt.Sprint(raw))
}

// selectVMItem picks the non-Windows on-demand or Spot row.
// An empty productName is kept so a single test row still prices.
// Low Priority is neither on-demand nor Spot. Spot is a whole word in
// skuName or meterName, not a letter sequence inside another word.
// No match returns ErrNotFound instead of a zero price.
func selectVMItem(items []azureclient.PriceItem, spot bool) (azureclient.PriceItem, error) {
	for _, item := range items {
		if !vmProductAllowed(item.ProductName) || isLowPriorityVM(item) {
			continue
		}
		if isSpotVM(item) != spot {
			continue
		}
		return item, nil
	}
	return azureclient.PriceItem{}, azureclient.ErrNotFound
}

func vmProductAllowed(productName string) bool {
	if strings.TrimSpace(productName) == "" {
		return true
	}
	return !strings.Contains(strings.ToLower(productName), "windows")
}

func isLowPriorityVM(item azureclient.PriceItem) bool {
	name := strings.ToLower(item.SkuName + " " + item.MeterName)
	return strings.Contains(name, "low priority")
}

func isSpotVM(item azureclient.PriceItem) bool {
	return hasVMWord(item.SkuName, "spot") || hasVMWord(item.MeterName, "spot")
}

func hasVMWord(value, word string) bool {
	return strings.Contains(" "+strings.ToLower(value)+" ", " "+strings.ToLower(word)+" ")
}

func vmBillingDetail(spot bool, service, sku, region string) string {
	kind := "On-demand"
	if spot {
		kind = vmPrioritySpot
	}
	return fmt.Sprintf("%s %s %s in %s, 730 hrs/month", kind, service, sku, region)
}
