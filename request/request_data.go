package request

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// generateRequestData generates the query parameters and request body from the given data which should be a struct.
// The query parameters are extracted from the "query" tag of the struct fields.
// The request body is extracted from the struct fields using the "json" tag.
// If the field has a "-" tag, it will be ignored.
// If the field is zero value, it will be ignored.
func generateRequestData(data any) (queryParams map[string]string, requestBody []byte, err error) {
	queryParams = make(map[string]string)

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		queryTag := field.Tag.Get("query")

		if queryTag == "" || queryTag == "-" {
			continue
		}

		fieldValue := v.Field(i)
		if fieldValue.IsZero() {
			continue
		}

		queryParams[queryTag] = fmt.Sprintf("%v", fieldValue.Interface())
	}

	requestBody, err = json.Marshal(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	return
}
