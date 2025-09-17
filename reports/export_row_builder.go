package reports

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// fieldIndexCache caches type field indexes for faster lookup
var fieldIndexCache sync.Map

type fieldIndex struct {
	index   int
	isValid bool
}

// RowBuilder builds formatted rows from structured data
type RowBuilder struct {
	fields []FieldConfig
}

// FieldConfig defines how to extract and format a field
type FieldConfig struct {
	Field      string    // Field name or path (e.g., "UserId" or "User.Name", could be field's name or json tag name)
	Formatter  Formatter // Formatter to apply (nil means use OriginalFormatter)
	UseContext bool      // Whether to use FormatWithContext if available
}

// NewRowBuilder creates a new row builder
func NewRowBuilder() *RowBuilder {
	return &RowBuilder{
		fields: make([]FieldConfig, 0),
	}
}

// Add adds a field with its formatter
func (b *RowBuilder) Add(field string, formatter Formatter) *RowBuilder {
	if formatter == nil {
		formatter = &OriginalFormatter{}
	}
	b.fields = append(b.fields, FieldConfig{
		Field:      field,
		Formatter:  formatter,
		UseContext: false,
	})
	return b
}

// AddWithContext adds a field that will use FormatWithContext if the formatter supports it
func (b *RowBuilder) AddWithContext(field string, formatter Formatter) *RowBuilder {
	if formatter == nil {
		formatter = &OriginalFormatter{}
	}
	b.fields = append(b.fields, FieldConfig{
		Field:      field,
		Formatter:  formatter,
		UseContext: true,
	})
	return b
}

// AddMultiple adds multiple fields with the same formatter
func (b *RowBuilder) AddMultiple(fields []string, formatter Formatter) *RowBuilder {
	for _, field := range fields {
		b.Add(field, formatter)
	}
	return b
}

// getFieldIndexes gets cached field indexes for a type
func getFieldIndexes(t reflect.Type) map[string]fieldIndex {
	if cached, ok := fieldIndexCache.Load(t); ok {
		return cached.(map[string]fieldIndex)
	}

	indexes := make(map[string]fieldIndex)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Store by field name
		indexes[field.Name] = fieldIndex{index: i, isValid: true}

		// Store by json tag
		if tag := field.Tag.Get("json"); tag != "" {
			jsonName := strings.SplitN(tag, ",", 2)[0]
			if jsonName != "" && jsonName != "-" {
				indexes[jsonName] = fieldIndex{index: i, isValid: true}
			}
		}
	}

	fieldIndexCache.Store(t, indexes)
	return indexes
}

// Build processes the data and returns formatted rows
func (b *RowBuilder) Build(data interface{}) ([][]string, error) {
	// Convert data to slice
	slice := reflect.ValueOf(data)
	if slice.Kind() != reflect.Slice {
		return nil, fmt.Errorf("data must be a slice")
	}

	if slice.Len() == 0 {
		return [][]string{}, nil
	}

	// Get the type from the first element and cache field indexes
	firstItem := slice.Index(0)
	itemType := firstItem.Type()
	if itemType.Kind() == reflect.Ptr {
		itemType = itemType.Elem()
	}
	indexes := getFieldIndexes(itemType)

	// Pre-compile field accessors for all configured fields
	type fieldAccessor struct {
		fieldIndex int
		formatter  Formatter
		useContext bool
	}
	accessors := make([]fieldAccessor, len(b.fields))

	for i, field := range b.fields {
		if idx, ok := indexes[field.Field]; ok && idx.isValid {
			accessors[i] = fieldAccessor{
				fieldIndex: idx.index,
				formatter:  field.Formatter,
				useContext: field.UseContext,
			}
		} else {
			return nil, fmt.Errorf("field %s not found", field.Field)
		}
	}

	rows := make([][]string, 0, slice.Len())

	// Process each item
	for i := 0; i < slice.Len(); i++ {
		item := slice.Index(i)
		itemInterface := item.Interface()

		// Handle pointer if necessary
		itemValue := item
		if itemValue.Kind() == reflect.Ptr {
			if itemValue.IsNil() {
				// Skip nil items or handle as needed
				row := make([]string, len(b.fields))
				rows = append(rows, row)
				continue
			}
			itemValue = itemValue.Elem()
		}

		row := make([]string, len(b.fields))

		for j, accessor := range accessors {
			// Extract value using cached index
			value := itemValue.Field(accessor.fieldIndex).Interface()

			// Apply formatter with context support if requested
			var formatted string
			var err error

			if accessor.useContext {
				// User explicitly wants context formatting
				if contextFormatter, ok := accessor.formatter.(ContextFormatter); ok {
					formatted, err = contextFormatter.FormatWithContext(value, itemInterface)
				} else {
					// Fallback to regular Format if formatter doesn't support context
					if accessor.formatter != nil {
						formatted, err = accessor.formatter.Format(value)
					} else {
						formatted, err = (&OriginalFormatter{}).Format(value)
					}
				}
			} else {
				// Use regular Format
				if accessor.formatter != nil {
					formatted, err = accessor.formatter.Format(value)
				} else {
					formatted, err = (&OriginalFormatter{}).Format(value)
				}
			}

			if err != nil {
				// Fallback to string representation
				formatted = fmt.Sprintf("%v", value)
			}

			row[j] = formatted
		}

		rows = append(rows, row)
	}

	return rows, nil
}

