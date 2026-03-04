// Package contracts defines the API contract for the ResourceDescriptor
// to PriceQuery mapping layer. This file is a design artifact — not compiled code.
//
// Location: specs/016-descriptor-filter-mapping/contracts/mapper.go
// Implements: FR-001 through FR-012 from spec.md
package contracts

import (
	"context"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// --- Exported Function Signatures ---

// MapDescriptorToQuery translates a finfocus ResourceDescriptor into an
// azureclient PriceQuery suitable for the Azure Retail Prices API.
//
// Validation is performed before mapping:
//   - Provider must be "azure" (case-insensitive)
//   - ResourceType must match a supported type (case-insensitive)
//   - Region must be resolvable (primary field or Tags["region"])
//   - SKU must be resolvable (primary field or Tags["sku"])
//
// Returns ErrUnsupportedResourceType for unknown providers or resource types.
// Returns a descriptive error naming all missing required fields.
// Returns a valid *PriceQuery with CurrencyCode defaulted to "USD" on success.
func MapDescriptorToQuery(_ *finfocusv1.ResourceDescriptor) (*azureclient.PriceQuery, error) {
	panic("contract only — see internal/pricing/mapper.go for implementation")
}

// SupportedResourceTypes returns the list of resource type identifiers that
// have a defined mapping to Azure service names. Resource types are returned
// in their canonical form (e.g., "compute/VirtualMachine").
func SupportedResourceTypes() []string {
	panic("contract only — see internal/pricing/mapper.go for implementation")
}

// --- Error Sentinels ---
//
// ErrUnsupportedResourceType — returned when provider is not "azure" or
// resource type has no mapping. Maps to gRPC codes.Unimplemented.
//
// ErrMissingRequiredFields — returned when region and/or SKU cannot be
// resolved from primary fields or tag fallback. Maps to gRPC codes.InvalidArgument.

// --- Integration Point: Calculator ---
//
// The Calculator (pricing/calculator.go) calls MapDescriptorToQuery in:
//   - Supports() — to validate whether a ResourceDescriptor can be mapped
//   - EstimateCost() — to get the PriceQuery for API lookup (future)
//   - DryRun() — to validate mapping without making API calls (future)
//
// Example usage in Supports():
//
//   func (c *Calculator) Supports(ctx context.Context, req *finfocusv1.SupportsRequest) (*finfocusv1.SupportsResponse, error) {
//       _, err := MapDescriptorToQuery(req.GetResource())
//       if err != nil {
//           return &finfocusv1.SupportsResponse{
//               Supported: false,
//               Reason:    err.Error(),
//           }, nil
//       }
//       return &finfocusv1.SupportsResponse{Supported: true}, nil
//   }

// Ensure context is used (suppresses unused import).
var _ = context.Background
