package util

import (
	"strings"

	"github.com/samber/lo"
)

// IsKCurrency checks if the currency is a currency with (K) or (k) suffix
func IsKCurrency(currency string) bool {
	currency = strings.ToUpper(currency)
	return strings.HasSuffix(currency, "(K)")
}

// NeedConvertToK checks if the currency needs to be converted to K
func NeedConvertToKCurrency(currency string) bool {
	return lo.Contains([]string{"VND", "IDR", "MMK"}, strings.ToUpper(currency))
}

// GetKCurrency returns the K currency of a currency
func GetKCurrency(currency string) string {
	currency = strings.ToUpper(currency)
	if IsKCurrency(currency) {
		return currency
	}
	return currency + "(K)"
}

// GetBaseCurrency returns the base currency of a currency with (K) or (k) suffix
func GetBaseCurrency(currency string) string {
	currency = strings.ToUpper(currency)
	if strings.HasSuffix(currency, "(K)") {
		return strings.TrimSuffix(currency, "(K)")
	}
	return currency
}
