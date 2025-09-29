package reports

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Formatter is the interface for data formatters
type Formatter interface {
	Format(value interface{}) (string, error)
}

// ContextFormatter is a formatter that has access to the full row context
type ContextFormatter interface {
	Formatter
	FormatWithContext(value interface{}, context interface{}) (string, error)
}

// FieldFormatter handles specific field transformation
type FieldFormatter struct {
	formatters map[string]Formatter
}

// NewFieldFormatter creates a new field formatter
func NewFieldFormatter() *FieldFormatter {
	return &FieldFormatter{
		formatters: make(map[string]Formatter),
	}
}

// RegisterFormatter registers a formatter for a specific field
func (f *FieldFormatter) RegisterFormatter(field string, formatter Formatter) {
	f.formatters[field] = formatter
}

// FormatField formats a field using its registered formatter
func (f *FieldFormatter) FormatField(field string, value interface{}) (string, error) {
	formatter, ok := f.formatters[field]
	if !ok {
		// If no formatter is registered, return the value as string
		return fmt.Sprintf("%v", value), nil
	}
	return formatter.Format(value)
}

// DateTimeFormatter formats timestamps with timezone
type DateTimeFormatter struct {
	TimeZone   string // e.g., "UTC+2"
	TimeFormat string // e.g., "2006-01-02 15:04:05"
}

func (f *DateTimeFormatter) Format(value interface{}) (string, error) {
	var timestamp int64
	switch v := value.(type) {
	case int64:
		timestamp = v
	case int:
		timestamp = int64(v)
	default:
		return "", fmt.Errorf("invalid timestamp type: %T", value)
	}

	// Parse timezone offset
	location := time.UTC
	if f.TimeZone != "" && strings.HasPrefix(f.TimeZone, "UTC") {
		offsetStr := strings.TrimPrefix(f.TimeZone, "UTC")
		if offsetStr != "" {
			offsetHours, err := strconv.Atoi(offsetStr)
			if err == nil {
				location = time.FixedZone(f.TimeZone, offsetHours*3600)
			}
		}
	}

	t := time.UnixMilli(timestamp).In(location)
	format := f.TimeFormat
	if format == "" {
		format = "2006-01-02 15:04:05" // Default format
	}
	return t.Format(format), nil
}

// Currency defines currency formatting information
type Currency struct {
	Code             string
	Symbol           string
	DecimalPlaces    int32
	ThousandsSeparator   string
	DecimalSeparator string
}

// CurrencyAmountFormatter formats currency amounts (in string format) with sign and separators
type CurrencyAmountFormatter struct {
	ShowSign                bool
	DefaultDecimalPlaces    int32  // Default decimal places if currency not found
	DefaultThousandsSeparator   string // Default thousand separator, e.g., "," for 100,000 or "." for 100.000
	DefaultDecimalSeparator string // Default decimal separator, e.g., "." for 100.00 or "," for 100,00

	// Dynamic currency support
	CurrencyMap map[string]*Currency     // Map of currency code to currency info
	GetCurrency func(interface{}) string // Function to extract currency from context
}

func (f *CurrencyAmountFormatter) Format(value interface{}) (string, error) {
	// Use default settings when no context available
	return f.formatAmount(value, f.DefaultDecimalPlaces, f.DefaultThousandsSeparator, f.DefaultDecimalSeparator)
}

// FormatWithContext formats amount using currency-specific settings if available
func (f *CurrencyAmountFormatter) FormatWithContext(value interface{}, context interface{}) (string, error) {
	// Start with defaults
	decimalPlaces := f.DefaultDecimalPlaces
	commaSep := f.DefaultThousandsSeparator
	decimalSep := f.DefaultDecimalSeparator

	// Override with currency-specific settings if available
	if f.CurrencyMap != nil && f.GetCurrency != nil && context != nil {
		currencyCode := f.GetCurrency(context)
		if currencyCode != "" {
			if currency, exists := f.CurrencyMap[currencyCode]; exists && currency != nil {
				decimalPlaces = currency.DecimalPlaces
				// Only override separators if explicitly set
				if currency.ThousandsSeparator != "" {
					commaSep = currency.ThousandsSeparator
				}
				if currency.DecimalSeparator != "" {
					decimalSep = currency.DecimalSeparator
				}
			}
		}
	}

	return f.formatAmount(value, decimalPlaces, commaSep, decimalSep)
}

func (f *CurrencyAmountFormatter) formatAmount(value interface{}, decimalPlaces int32, commaSep, decimalSep string) (string, error) {
	var amount decimal.Decimal
	switch v := value.(type) {
	case string:
		var err error
		amount, err = decimal.NewFromString(v)
		if err != nil {
			return "", err
		}
	case decimal.Decimal:
		amount = v
	default:
		return "", fmt.Errorf("invalid amount type: %T", value)
	}

	// Round to decimal places
	if decimalPlaces > 0 {
		amount = amount.Round(decimalPlaces)
	}

	// Format the number
	formatted := amount.StringFixed(decimalPlaces)

	// Apply custom separators
	formatted = f.formatWithSeparators(formatted, commaSep, decimalSep)

	// Add sign if needed and positive
	if f.ShowSign && amount.IsPositive() && !amount.IsZero() {
		formatted = "+" + formatted
	}

	return formatted, nil
}

// formatWithSeparators formats with explicit separator parameters
func (f *CurrencyAmountFormatter) formatWithSeparators(s string, commaSep, decimalSep string) string {
	// Default separators if not specified
	if commaSep == "" {
		commaSep = ","
	}
	if decimalSep == "" {
		decimalSep = "."
	}

	// Split by the default decimal point
	parts := strings.Split(s, ".")
	intPart := parts[0]

	// Handle negative numbers
	isNegative := strings.HasPrefix(intPart, "-")
	if isNegative {
		intPart = intPart[1:]
	}

	// Add thousand separators
	result := ""
	for i, digit := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result += commaSep
		}
		result += string(digit)
	}

	// Add back negative sign if needed
	if isNegative {
		result = "-" + result
	}

	// Add decimal part with custom separator if exists
	if len(parts) > 1 {
		result += decimalSep + parts[1]
	}

	return result
}

// MapFormatter maps string values to their display representations
type MapFormatter struct {
	Mappings map[string]string
}

func (f *MapFormatter) Format(value interface{}) (string, error) {
	inputStr, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid value type: %T", value)
	}

	mappedValue, exists := f.Mappings[inputStr]
	if !exists {
		return inputStr, nil // Return original if no mapping
	}
	return mappedValue, nil
}

// PrefixSuffixFormatter adds prefix and/or suffix to values
type PrefixSuffixFormatter struct {
	Prefix string
	Suffix string
}

func (f *PrefixSuffixFormatter) Format(value interface{}) (string, error) {
	if value == nil {
		return f.Prefix + f.Suffix, nil
	}
	return f.Prefix + fmt.Sprintf("%v", value) + f.Suffix, nil
}

// OriginalFormatter returns values as-is without any formatting
type OriginalFormatter struct{}

func (f *OriginalFormatter) Format(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	return fmt.Sprintf("%v", value), nil
}
