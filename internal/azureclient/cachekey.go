package azureclient

import "strings"

// CacheKey returns a normalized cache key for a pricing query.
func CacheKey(query PriceQuery) string {
	parts := []string{
		normalizeKeyPart(query.ArmRegionName),
		normalizeKeyPart(query.ArmSkuName),
		normalizeKeyPart(query.SkuName),
		normalizeKeyPart(query.ProductName),
		normalizeKeyPart(query.ServiceName),
		normalizeKeyPart(query.CurrencyCode),
	}
	if strings.TrimSpace(query.PriceType) != "" || strings.TrimSpace(query.APIVersion) != "" {
		parts = append(parts,
			"type="+normalizeKeyPart(query.PriceType),
			"api="+normalizeKeyPart(query.APIVersion),
		)
	}

	return strings.Join(parts, "|")
}

func normalizeKeyPart(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
