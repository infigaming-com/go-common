package util

import (
	"github.com/shopspring/decimal"
)

func DecimalFromString(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}

	return decimal.NewFromString(s)
}
