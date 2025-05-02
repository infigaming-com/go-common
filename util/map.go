package util

func GetMapValue[T any](m map[string]any, key string, defaultValue T) T {
	if m[key] == nil {
		return defaultValue
	}

	if value, ok := m[key].(T); ok {
		return value
	}

	return defaultValue
}
