package util

import (
	"context"
	"fmt"
)

type CtxKey string

const (
	CorrelationIdKey CtxKey = "CorrelationId"
)

func ValueToCtx[T any](ctx context.Context, key CtxKey, value T) context.Context {
	return context.WithValue(ctx, key, value)
}

func ValueFromCtx[T any](ctx context.Context, key CtxKey) (T, error) {
	valueFromCtx := ctx.Value(key)
	if valueFromCtx == nil {
		return *new(T), NewUtilError(ErrCodeValueNotFoundInContext, fmt.Sprintf("%v not found in context", key), nil, nil)
	}

	// Try direct type assertion first (works for structs and interfaces)
	if value, ok := valueFromCtx.(T); ok {
		return value, nil
	}

	// Try through any interface (works for primitive types)
	if value, ok := valueFromCtx.(any).(T); ok {
		return value, nil
	}

	return *new(T), NewUtilError(ErrCodeInvalidValueInContext, fmt.Sprintf("%v is not of type %T on context", key, new(T)), nil, nil)
}

func CorrelationIdToCtx(ctx context.Context, correlationId string) context.Context {
	return ValueToCtx(ctx, CorrelationIdKey, correlationId)
}

func CorrelationIdFromCtx(ctx context.Context) (string, error) {
	value, err := ValueFromCtx[string](ctx, CorrelationIdKey)
	if err != nil {
		return "", err
	}
	return value, nil
}
