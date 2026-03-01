//go:build ignore
// +build ignore

// Package contracts defines the API surface for the OData filter builder.
// This file is a design contract, not compilable production code.
// It documents the public API that will be implemented in internal/azureclient/filter.go.
package contracts

// --- Value Types ---

// FilterCondition represents a single OData equality condition.
// It is an immutable value type created by package-level constructor functions.
//
// Example: Region("eastus") creates FilterCondition{Field: "armRegionName", Value: "eastus"}
type FilterCondition struct {
	Field string // OData field name (e.g., "armRegionName")
	Value string // Filter value (e.g., "eastus")
}

// --- Package-Level Constructors ---
// These return FilterCondition values for use with FilterBuilder.Or().

// Region creates a condition for armRegionName.
func Region(value string) FilterCondition

// Service creates a condition for serviceName.
func Service(value string) FilterCondition

// SKU creates a condition for armSkuName.
func SKU(value string) FilterCondition

// PriceType creates a condition for priceType.
func PriceType(value string) FilterCondition

// ProductName creates a condition for productName.
func ProductName(value string) FilterCondition

// CurrencyCode creates a condition for currencyCode.
func CurrencyCode(value string) FilterCondition

// Condition creates a condition for an arbitrary OData field.
func Condition(field, value string) FilterCondition

// --- Builder Type ---

// FilterBuilder constructs OData $filter expressions for the Azure Retail Prices API.
//
// The builder provides a fluent/chainable interface where each method returns
// the builder for method chaining. Criteria are combined with AND logic by default.
// Use Or() to create parenthesized OR groups.
//
// The builder always includes a default priceType filter of "Consumption"
// unless explicitly overridden via Type().
//
// Output is deterministic regardless of the order in which methods are called.
type FilterBuilder struct {
	// Internal fields (not exported):
	// andConditions map[string]string       - named AND conditions (field → value, last-write-wins)
	// orGroups      [][]FilterCondition     - OR-grouped condition sets
	// typeValue     *string                 - pricing type override (nil = default "Consumption")
	// genericFields []FilterCondition       - arbitrary field conditions via Field()
}

// NewFilterBuilder creates a new FilterBuilder with default settings.
// The default pricing type is "Consumption" (pay-as-you-go).
func NewFilterBuilder() *FilterBuilder

// --- Named Convenience Methods (AND conditions, chainable) ---

// Region adds an armRegionName filter criterion (AND logic, last-write-wins).
func (b *FilterBuilder) Region(value string) *FilterBuilder

// Service adds a serviceName filter criterion (AND logic, last-write-wins).
func (b *FilterBuilder) Service(value string) *FilterBuilder

// SKU adds an armSkuName filter criterion (AND logic, last-write-wins).
func (b *FilterBuilder) SKU(value string) *FilterBuilder

// Type sets the priceType filter criterion, overriding the default "Consumption".
// Empty or whitespace-only values are ignored (default still applies).
func (b *FilterBuilder) Type(value string) *FilterBuilder

// ProductName adds a productName filter criterion (AND logic, last-write-wins).
func (b *FilterBuilder) ProductName(value string) *FilterBuilder

// CurrencyCode adds a currencyCode filter criterion (AND logic, last-write-wins).
func (b *FilterBuilder) CurrencyCode(value string) *FilterBuilder

// --- Generic Field Method (AND condition, chainable) ---

// Field adds an arbitrary OData field filter criterion (AND logic).
// Both field name and value must be non-empty and non-whitespace.
// Unlike named methods, generic fields do not use last-write-wins;
// multiple calls with the same field name are both included.
func (b *FilterBuilder) Field(name, value string) *FilterBuilder

// --- OR Grouping Method ---

// Or creates a parenthesized OR group from the given conditions.
// When mixed with AND conditions, OR groups are automatically parenthesized.
// Empty or whitespace-only conditions within the group are silently omitted.
// If all conditions are empty, the entire group is omitted.
//
// Example:
//
//	builder.Or(Region("eastus"), Region("westus2"))
//	// produces: (armRegionName eq 'eastus' or armRegionName eq 'westus2')
func (b *FilterBuilder) Or(conditions ...FilterCondition) *FilterBuilder

// --- Build Method ---

// Build produces the OData $filter expression string.
//
// The output is deterministic: all parts are sorted alphabetically by their
// primary field name (first condition's field for OR groups).
//
// The default priceType filter ("Consumption") is included unless overridden.
//
// Empty/whitespace values are silently omitted. Single quotes in values are
// escaped per OData conventions (doubled: '').
//
// If no criteria are set (all empty), returns the minimal valid filter:
// "priceType eq 'Consumption'"
func (b *FilterBuilder) Build() string

// --- Internal Helpers (not exported) ---

// escapeODataValue escapes single quotes in a value per OData v4 conventions.
// "O'Brien" → "O''Brien"
// func escapeODataValue(s string) string

// isBlank returns true if s is empty or contains only whitespace.
// func isBlank(s string) bool
