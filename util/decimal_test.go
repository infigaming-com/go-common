package util

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNewDecimal(t *testing.T) {
	tcs := []struct {
		name        string
		value       any
		expectValue decimal.Decimal
		expectErr   bool
	}{
		{
			name:        "string",
			value:       "123",
			expectValue: decimal.NewFromInt(123),
			expectErr:   false,
		},
		{
			name:        "int64",
			value:       int64(123),
			expectValue: decimal.NewFromInt(123),
			expectErr:   false,
		},
		{
			name:        "float64",
			value:       float64(123.45),
			expectValue: decimal.NewFromFloat(123.45),
			expectErr:   false,
		},
		{
			name:        "invalid",
			value:       "invalid",
			expectValue: decimal.Zero,
			expectErr:   true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actualValue, actualErr := NewDecimal(tc.value)
			if tc.expectErr {
				assert.Error(t, actualErr)
			} else {
				assert.NoError(t, actualErr)
			}
			assert.Equal(t, tc.expectValue, actualValue)
		})
	}
}

func TestNewSumDecimal(t *testing.T) {
	tcs := []struct {
		name        string
		values      []any
		expectValue decimal.Decimal
		expectErr   bool
	}{
		{
			name:        "string",
			values:      []any{"123", "456"},
			expectValue: decimal.NewFromInt(579),
			expectErr:   false,
		},
		{
			name:        "int64",
			values:      []any{int64(123), int64(456)},
			expectValue: decimal.NewFromInt(579),
			expectErr:   false,
		},
		{
			name:        "float64",
			values:      []any{float64(123.45), float64(456.78)},
			expectValue: decimal.NewFromFloat(580.23),
			expectErr:   false,
		},
		{
			name:        "mixed-1",
			values:      []any{"123", 456, float64(123.45)},
			expectValue: decimal.NewFromFloat(702.45),
			expectErr:   false,
		},
		{
			name:        "mixed-2",
			values:      []any{123, 456, 123.45},
			expectValue: decimal.NewFromFloat(702.45),
			expectErr:   false,
		},
		{
			name:        "mixed-3",
			values:      []any{123, int64(456), float64(123.45)},
			expectValue: decimal.NewFromFloat(702.45),
			expectErr:   false,
		},
		{
			name:        "invalid",
			values:      []any{"invalid", "invalid"},
			expectValue: decimal.Zero,
			expectErr:   true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actualValue, actualErr := NewSumDecimal(tc.values)
			if tc.expectErr {
				assert.Error(t, actualErr)
			} else {
				assert.NoError(t, actualErr)
			}
			assert.Equal(t, tc.expectValue, actualValue)
		})
	}
}

func TestExportDecimal(t *testing.T) {
	tcs := []struct {
		name        string
		targetType  string
		value       decimal.Decimal
		expectValue any
		expectErr   bool
	}{
		{
			name:        "string",
			targetType:  "string",
			value:       decimal.NewFromInt(123),
			expectValue: "123",
			expectErr:   false,
		},
		{
			name:        "int",
			targetType:  "int",
			value:       decimal.NewFromInt(123),
			expectValue: int(123),
			expectErr:   false,
		},
		{
			name:        "int32",
			targetType:  "int32",
			value:       decimal.NewFromInt(123),
			expectValue: int32(123),
			expectErr:   false,
		},
		{
			name:        "uint32",
			targetType:  "uint32",
			value:       decimal.NewFromInt(123),
			expectValue: uint32(123),
			expectErr:   false,
		},
		{
			name:        "int64",
			targetType:  "int64",
			value:       decimal.NewFromInt(123),
			expectValue: int64(123),
			expectErr:   false,
		},
		{
			name:        "uint64",
			targetType:  "uint64",
			value:       decimal.NewFromInt(123),
			expectValue: uint64(123),
			expectErr:   false,
		},
		{
			name:        "float32",
			targetType:  "float32",
			value:       decimal.NewFromInt(123),
			expectValue: float32(123),
			expectErr:   false,
		},
		{
			name:        "float64",
			targetType:  "float64",
			value:       decimal.NewFromInt(123),
			expectValue: float64(123),
			expectErr:   false,
		},
		{
			name:        "negative int",
			targetType:  "int",
			value:       decimal.NewFromInt(-123),
			expectValue: int(-123),
			expectErr:   false,
		},
		{
			name:        "negative int32",
			targetType:  "int32",
			value:       decimal.NewFromInt(-123),
			expectValue: int32(-123),
			expectErr:   false,
		},
		{
			name:        "negative int64",
			targetType:  "int64",
			value:       decimal.NewFromInt(-123),
			expectValue: int64(-123),
			expectErr:   false,
		},
		{
			name:        "negative float32",
			targetType:  "float32",
			value:       decimal.NewFromInt(-123),
			expectValue: float32(-123),
			expectErr:   false,
		},
		{
			name:        "negative float64",
			targetType:  "float64",
			value:       decimal.NewFromInt(-123),
			expectValue: float64(-123),
			expectErr:   false,
		},
		{
			name:        "decimal places string",
			targetType:  "string",
			value:       decimal.NewFromFloat(123.45),
			expectValue: "123.45",
			expectErr:   false,
		},
		{
			name:        "decimal places float32",
			targetType:  "float32",
			value:       decimal.NewFromFloat(123.45),
			expectValue: float32(123.45),
			expectErr:   false,
		},
		{
			name:        "decimal places float64",
			targetType:  "float64",
			value:       decimal.NewFromFloat(123.45),
			expectValue: float64(123.45),
			expectErr:   false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var actualValue any
			var actualErr error

			switch tc.targetType {
			case "string":
				val, err := ExportDecimal[string](tc.value)
				actualValue = val
				actualErr = err
			case "int":
				val, err := ExportDecimal[int](tc.value)
				actualValue = val
				actualErr = err
			case "int32":
				val, err := ExportDecimal[int32](tc.value)
				actualValue = val
				actualErr = err
			case "uint32":
				val, err := ExportDecimal[uint32](tc.value)
				actualValue = val
				actualErr = err
			case "int64":
				val, err := ExportDecimal[int64](tc.value)
				actualValue = val
				actualErr = err
			case "uint64":
				val, err := ExportDecimal[uint64](tc.value)
				actualValue = val
				actualErr = err
			case "float32":
				val, err := ExportDecimal[float32](tc.value)
				actualValue = val
				actualErr = err
			case "float64":
				val, err := ExportDecimal[float64](tc.value)
				actualValue = val
				actualErr = err
			default:
				t.Fatalf("unsupported target type: %s", tc.targetType)
			}

			if tc.expectErr {
				assert.Error(t, actualErr)
			} else {
				assert.NoError(t, actualErr)
			}
			assert.Equal(t, tc.expectValue, actualValue)
		})
	}
}
