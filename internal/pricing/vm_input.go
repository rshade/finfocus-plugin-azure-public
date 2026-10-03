package pricing

import (
	"fmt"
	"strconv"
	"strings"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// vmSKU reads the size the real Pulumi providers send. The descriptor Sku
// wins. Classic Linux and Windows VMs use size. Native VMs use
// hardwareProfile.vmSize, which today's core flattens to the tag
// hardwareProfile. Scale sets use sku.name or skuName when Sku is empty.
func vmSKU(resource *finfocusv1.ResourceDescriptor) string {
	if resource == nil {
		return ""
	}
	if sku := strings.TrimSpace(resource.GetSku()); sku != "" {
		return sku
	}
	tags := resource.GetTags()
	if sku := firstNonEmptyTag(
		tags,
		"sku",
		"vmSize",
		"armSkuName",
		"size",
		"hardwareProfile.vmSize",
		"sku.name",
		"skuName",
	); sku != "" {
		return sku
	}
	profile := strings.TrimSpace(tags["hardwareProfile"])
	if profile == "" || strings.Contains(profile, " ") || strings.Contains(profile, "map[") {
		return ""
	}
	return profile
}

// vmOSWindows reports a Windows guest. Classic uses the token. Native uses
// osProfile.windowsConfiguration, including the core's collapsed osProfile
// string. Legacy VMs use osProfileWindowsConfig.
func vmOSWindows(resource *finfocusv1.ResourceDescriptor) bool {
	if resource == nil {
		return false
	}
	lower := strings.ToLower(resource.GetResourceType())
	if isWindowsVirtualMachineResourceType(lower) ||
		resourceSegment(lower, "compute/windowsvirtualmachinescaleset") {
		return true
	}
	tags := resource.GetTags()
	if value, ok := tags["osProfileWindowsConfig"]; ok && value != formattedNil {
		return true
	}
	for key, value := range tags {
		if key != "osProfile.windowsConfiguration" && !strings.HasPrefix(key, "osProfile.windowsConfiguration.") {
			continue
		}
		if strings.TrimSpace(value) != "" && value != formattedNil {
			return true
		}
	}
	return strings.Contains(strings.ToLower(tags["osProfile"]), "windowsconfiguration")
}

// vmHybridLicense reports Windows_Server or Windows_Client on a Windows VM.
// Those values are Hybrid Benefit: the licence is already paid, so the
// compute rate is the base (Linux) rate. The returned name is the note's
// license word.
func vmHybridLicense(resource *finfocusv1.ResourceDescriptor) (string, bool) {
	if !vmOSWindows(resource) {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(resource.GetTags()["licenseType"])) {
	case "windows_server":
		return "Windows_Server", true
	case "windows_client":
		return "Windows_Client", true
	default:
		return "", false
	}
}

func hybridBenefitNote(license string) string {
	return fmt.Sprintf(
		"licenseType %s is Azure Hybrid Benefit: the licence is already paid, "+
			"so the compute rate is the base (Linux) rate. Confidence Medium "+
			"(Azure documentation, not verified against the Calculator)",
		license,
	)
}

// vmInstanceCount reads classic instances, then native sku.capacity.
// A missing count is one instance. The core drops sku.capacity when it
// collapses an object sku to its name.
func vmInstanceCount(tags map[string]string) (int, error) {
	raw := firstNonEmptyTag(tags, "instances", "sku.capacity")
	if raw == "" {
		return 1, nil
	}
	count, err := strconv.Atoi(raw)
	if err != nil || count < 0 {
		return 0, status.Errorf(codes.InvalidArgument, "unsupported instance count %q", raw)
	}
	return count, nil
}

func vmQuoteDetail(spot bool, service, sku, region, license string, count int) string {
	detail := vmBillingDetail(spot, service, sku, region)
	if count > 1 {
		detail = fmt.Sprintf("%s, %d instances", detail, count)
	}
	if license != "" {
		detail += ". " + hybridBenefitNote(license)
	}
	return detail
}

// vmPriorityRaw is the top-level priority, else the scale set profile priority.
// Top-level wins. virtualMachineProfile.priority is the native scale set path.
func vmPriorityRaw(tags map[string]string) string {
	if tags == nil {
		return ""
	}
	if priority := strings.TrimSpace(tags["priority"]); priority != "" && priority != formattedNil {
		return priority
	}
	return tags["virtualMachineProfile.priority"]
}
