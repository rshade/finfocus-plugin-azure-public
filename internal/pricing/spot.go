package pricing

import (
	"fmt"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	vmPrioritySpot = "Spot"
	formattedNil   = "<nil>"
)

// descriptorSpot reports whether the descriptor selects Spot.
// priority wins. An empty priority falls through to pricing_model.
// pricing_model spot selects Spot. pricing_model consumption is on-demand.
// Any other non-empty value is InvalidArgument and the message names it.
func descriptorSpot(resource *finfocusv1.ResourceDescriptor) (bool, error) {
	if resource == nil {
		return false, nil
	}
	tags := resource.GetTags()
	return spotFromFields(vmPriorityRaw(tags), tags["pricing_model"])
}

// prioritySpot reports whether raw selects Spot. An empty value is on-demand.
// Any other non-empty value is InvalidArgument and the message names it.
func prioritySpot(raw string) (bool, error) {
	priority := strings.TrimSpace(raw)
	if priority == "" || priority == formattedNil {
		return false, nil
	}
	switch strings.ToLower(priority) {
	case strings.ToLower(vmPrioritySpot):
		return true, nil
	case "regular":
		// Classic preview fills the provider default Regular for on-demand.
		return false, nil
	default:
		return false, status.Errorf(codes.InvalidArgument, "unsupported priority %q", priority)
	}
}

// estimateSpot reads EstimateCost attributes priority and pricing_model.
// priority wins. The same values as the descriptor tags apply.
func estimateSpot(req *finfocusv1.EstimateCostRequest) (bool, error) {
	if req == nil || req.GetAttributes() == nil {
		return false, nil
	}
	attrs := req.GetAttributes().AsMap()
	return spotFromFields(attrString(attrs["priority"]), attrString(attrs["pricing_model"]))
}

func spotFromFields(priorityRaw, modelRaw string) (bool, error) {
	if strings.TrimSpace(priorityRaw) != "" && priorityRaw != formattedNil {
		return prioritySpot(priorityRaw)
	}
	model := strings.TrimSpace(modelRaw)
	if model == "" || model == formattedNil {
		return false, nil
	}
	if strings.EqualFold(model, vmPrioritySpot) {
		return true, nil
	}
	if strings.EqualFold(model, "consumption") {
		return false, nil
	}
	return false, status.Errorf(codes.InvalidArgument, "unsupported pricing_model %q", model)
}

func attrString(raw any) string {
	if raw == nil {
		return ""
	}
	return fmt.Sprint(raw)
}

// selectVMItem picks the non-Windows on-demand or Spot row.
// An empty productName is kept so a single test row still prices.
// Low Priority is neither on-demand nor Spot. Spot is a whole word in
// skuName or meterName, not a letter sequence inside another word.
// No match returns ErrNotFound instead of a zero price.
func selectVMItem(items []azureclient.PriceItem, spot bool) (azureclient.PriceItem, error) {
	return selectVMProduct(items, spot, false)
}

// selectVMProduct picks the on-demand or Spot row for the guest OS.
// windows selects a product whose name contains Windows. The Linux call
// keeps an empty productName so a single test row still prices.
func selectVMProduct(items []azureclient.PriceItem, spot, windows bool) (azureclient.PriceItem, error) {
	for _, item := range items {
		if !vmProductMatches(item.ProductName, windows) || isLowPriorityVM(item) {
			continue
		}
		if isSpotVM(item) != spot {
			continue
		}
		return item, nil
	}
	return azureclient.PriceItem{}, azureclient.ErrNotFound
}

func vmProductMatches(productName string, windows bool) bool {
	if strings.TrimSpace(productName) == "" {
		return !windows
	}
	hasWindows := strings.Contains(strings.ToLower(productName), "windows")
	return hasWindows == windows
}

func vmProductAllowed(productName string) bool {
	return vmProductMatches(productName, false)
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
