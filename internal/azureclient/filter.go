package azureclient

import (
	"fmt"
	"sort"
	"strings"
)

const defaultPriceType = "Consumption"

// FilterCondition represents a single OData equality condition.
type FilterCondition struct {
	// Field is the OData field name (for example, armRegionName).
	Field string
	// Value is the OData value.
	Value string
}

// Region creates a condition for armRegionName.
func Region(value string) FilterCondition {
	return FilterCondition{Field: "armRegionName", Value: value}
}

// Service creates a condition for serviceName.
func Service(value string) FilterCondition {
	return FilterCondition{Field: "serviceName", Value: value}
}

// SKU creates a condition for armSkuName.
func SKU(value string) FilterCondition {
	return FilterCondition{Field: "armSkuName", Value: value}
}

// PriceType creates a condition for priceType.
func PriceType(value string) FilterCondition {
	return FilterCondition{Field: "priceType", Value: value}
}

// ProductName creates a condition for productName.
func ProductName(value string) FilterCondition {
	return FilterCondition{Field: "productName", Value: value}
}

// CurrencyCode creates a condition for currencyCode.
func CurrencyCode(value string) FilterCondition {
	return FilterCondition{Field: "currencyCode", Value: value}
}

// Condition creates a condition for an arbitrary OData field name.
func Condition(field, value string) FilterCondition {
	return FilterCondition{Field: field, Value: value}
}

// FilterBuilder builds deterministic OData $filter expressions.
//
// Named methods contribute AND conditions with last-write-wins semantics.
// Or() contributes OR groups that are parenthesized when they contain
// multiple conditions. The builder always includes a priceType filter:
// "Consumption" by default, or the value set via Type().
type FilterBuilder struct {
	andConditions map[string]string
	orGroups      [][]FilterCondition
	typeValue     *string
	genericFields []FilterCondition
}

type filterPart struct {
	primaryField string
	expression   string
}

// NewFilterBuilder creates a new filter builder.
func NewFilterBuilder() *FilterBuilder {
	return &FilterBuilder{
		andConditions: make(map[string]string),
		orGroups:      make([][]FilterCondition, 0),
		genericFields: make([]FilterCondition, 0),
	}
}

// Region sets armRegionName using AND semantics.
func (b *FilterBuilder) Region(value string) *FilterBuilder {
	return b.setNamedCondition("armRegionName", value)
}

// Service sets serviceName using AND semantics.
func (b *FilterBuilder) Service(value string) *FilterBuilder {
	return b.setNamedCondition("serviceName", value)
}

// SKU sets armSkuName using AND semantics.
func (b *FilterBuilder) SKU(value string) *FilterBuilder {
	return b.setNamedCondition("armSkuName", value)
}

// Type sets an explicit priceType. Blank values are ignored.
func (b *FilterBuilder) Type(value string) *FilterBuilder {
	if b == nil || isBlank(value) {
		return b
	}

	b.typeValue = &value
	return b
}

// ProductName sets productName using AND semantics.
func (b *FilterBuilder) ProductName(value string) *FilterBuilder {
	return b.setNamedCondition("productName", value)
}

// CurrencyCode sets currencyCode using AND semantics.
func (b *FilterBuilder) CurrencyCode(value string) *FilterBuilder {
	return b.setNamedCondition("currencyCode", value)
}

// Field appends an arbitrary AND condition. Blank names/values are ignored.
func (b *FilterBuilder) Field(name, value string) *FilterBuilder {
	if b == nil || isBlank(name) || isBlank(value) {
		return b
	}

	b.genericFields = append(b.genericFields, FilterCondition{Field: name, Value: value})
	return b
}

// Or appends a new OR group containing non-blank conditions.
func (b *FilterBuilder) Or(conditions ...FilterCondition) *FilterBuilder {
	if b == nil || len(conditions) == 0 {
		return b
	}

	group := make([]FilterCondition, 0, len(conditions))
	for _, condition := range conditions {
		if isBlank(condition.Field) || isBlank(condition.Value) {
			continue
		}
		group = append(group, condition)
	}

	if len(group) == 0 {
		return b
	}

	b.orGroups = append(b.orGroups, group)
	return b
}

// Build renders the OData $filter expression.
//
// Output is deterministic and sorted by primary field name, then expression.
// Values are escaped using OData single-quote escaping (single quote -> double).
func (b *FilterBuilder) Build() string {
	if b == nil {
		return fmt.Sprintf("priceType eq '%s'", defaultPriceType)
	}

	parts := b.buildParts()

	expressions := make([]string, 0, len(parts))
	for _, part := range parts {
		expressions = append(expressions, part.expression)
	}
	return strings.Join(expressions, " and ")
}

func (b *FilterBuilder) buildParts() []filterPart {
	parts := make([]filterPart, 0, len(b.andConditions)+len(b.genericFields)+len(b.orGroups)+1)
	parts = append(parts, b.andParts()...)
	parts = append(parts, b.genericParts()...)
	parts = append(parts, b.orParts()...)
	parts = append(parts, b.typePart())
	sortFilterParts(parts)
	return parts
}

func (b *FilterBuilder) andParts() []filterPart {
	parts := make([]filterPart, 0, len(b.andConditions))
	for field, value := range b.andConditions {
		if isBlank(field) || isBlank(value) {
			continue
		}
		parts = append(parts, filterPart{
			primaryField: field,
			expression:   formatCondition(field, value),
		})
	}
	return parts
}

func (b *FilterBuilder) genericParts() []filterPart {
	parts := make([]filterPart, 0, len(b.genericFields))
	for _, condition := range b.genericFields {
		if isBlank(condition.Field) || isBlank(condition.Value) {
			continue
		}
		parts = append(parts, filterPart{
			primaryField: condition.Field,
			expression:   formatCondition(condition.Field, condition.Value),
		})
	}
	return parts
}

func (b *FilterBuilder) orParts() []filterPart {
	parts := make([]filterPart, 0, len(b.orGroups))
	for _, group := range b.orGroups {
		part, ok := buildORPart(group)
		if !ok {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func (b *FilterBuilder) typePart() filterPart {
	typeValue := defaultPriceType
	if b.typeValue != nil && !isBlank(*b.typeValue) {
		typeValue = *b.typeValue
	}

	return filterPart{
		primaryField: "priceType",
		expression:   formatCondition("priceType", typeValue),
	}
}

func (b *FilterBuilder) setNamedCondition(field, value string) *FilterBuilder {
	if b == nil || isBlank(value) {
		return b
	}

	b.andConditions[field] = value
	return b
}

func filterValidConditions(conditions []FilterCondition) []FilterCondition {
	valid := make([]FilterCondition, 0, len(conditions))
	for _, condition := range conditions {
		if isBlank(condition.Field) || isBlank(condition.Value) {
			continue
		}
		valid = append(valid, condition)
	}
	return valid
}

func buildORPart(group []FilterCondition) (filterPart, bool) {
	conditions := filterValidConditions(group)
	if len(conditions) == 0 {
		return filterPart{}, false
	}

	sort.SliceStable(conditions, func(i, j int) bool {
		if conditions[i].Field == conditions[j].Field {
			return conditions[i].Value < conditions[j].Value
		}
		return conditions[i].Field < conditions[j].Field
	})

	expressions := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		expressions = append(expressions, formatCondition(condition.Field, condition.Value))
	}

	expression := expressions[0]
	if len(expressions) > 1 {
		expression = "(" + strings.Join(expressions, " or ") + ")"
	}

	return filterPart{
		primaryField: conditions[0].Field,
		expression:   expression,
	}, true
}

func sortFilterParts(parts []filterPart) {
	sort.SliceStable(parts, func(i, j int) bool {
		if parts[i].primaryField == parts[j].primaryField {
			return parts[i].expression < parts[j].expression
		}
		return parts[i].primaryField < parts[j].primaryField
	})
}

func formatCondition(field, value string) string {
	return fmt.Sprintf("%s eq '%s'", field, escapeODataValue(value))
}

func escapeODataValue(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}
