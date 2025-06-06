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
