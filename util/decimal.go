package util

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type DecimalNumberType interface {
	string | int | int32 | uint32 | int64 | uint64 | float32 | float64 | decimal.Decimal
}

func DecimalFromString(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}

	value, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to convert string to decimal: %w", err)
	}

	return value, nil
}

func DecimalSum(nums ...string) (decimal.Decimal, error) {
	sum := decimal.Zero
	for _, num := range nums {
		num, err := DecimalFromString(num)
		if err != nil {
			return decimal.Zero, err
		}
		sum = sum.Add(num)
	}
	return sum, nil
}

func NewDecimal(value any) (decimal.Decimal, error) {
	switch value := value.(type) {
	case decimal.Decimal:
		return value, nil
	case string:
		return DecimalFromString(value)
	case int:
		return decimal.NewFromInt(int64(value)), nil
	case int32:
		return decimal.NewFromInt32(value), nil
	case uint32:
		return decimal.NewFromUint64(uint64(value)), nil
	case int64:
		return decimal.NewFromInt(value), nil
	case uint64:
		return decimal.NewFromUint64(value), nil
	case float32:
		return decimal.NewFromFloat32(value), nil
	case float64:
		return decimal.NewFromFloat(value), nil
	default:
		return decimal.Zero, fmt.Errorf("invalid type: %T", value)
	}
}

func NewSumDecimal(values []any) (decimal.Decimal, error) {
	sum := decimal.Zero
	for _, value := range values {
		num, err := NewDecimal(value)
		if err != nil {
			return decimal.Zero, err
		}
		sum = sum.Add(num)
	}
	return sum, nil
}

func ExportDecimal[D DecimalNumberType](value decimal.Decimal) (D, error) {
	var zero D
	switch any(zero).(type) {
	case string:
		return any(value.String()).(D), nil
	case int:
		return any(int(value.IntPart())).(D), nil
	case int32:
		return any(int32(value.IntPart())).(D), nil
	case uint32:
		return any(uint32(value.IntPart())).(D), nil
	case int64:
		return any(value.IntPart()).(D), nil
	case uint64:
		return any(uint64(value.IntPart())).(D), nil
	case float32:
		return any(float32(value.InexactFloat64())).(D), nil
	case float64:
		return any(value.InexactFloat64()).(D), nil
	case decimal.Decimal:
		return any(value).(D), nil
	default:
		return zero, fmt.Errorf("unsupported export type: %T", zero)
	}
}
