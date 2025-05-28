package util

import "strings"

// IsCurrencyWithK checks if the currency is a currency with (K) or (k) suffix
func IsCurrencyWithK(currency string) bool {
	return strings.HasSuffix(currency, "(K)") || strings.HasSuffix(currency, "(k)")
}

// GetBaseCurrency returns the base currency of a currency with (K) or (k) suffix
func GetBaseCurrency(currency string) string {
	if strings.HasSuffix(currency, "(K)") {
		return strings.TrimSuffix(currency, "(K)")
	}
	if strings.HasSuffix(currency, "(k)") {
		return strings.TrimSuffix(currency, "(k)")
	}
	return currency
}
