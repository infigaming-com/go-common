package util

import (
	"context"
	"fmt"
)

type ContextKey string

const (
	CorrelationIdKey ContextKey = "CorrelationId"
)

func valueToCtx[T any](ctx context.Context, key ContextKey, value T) context.Context {
	return context.WithValue(ctx, key, value)
}

func valueFromCtx[T any](ctx context.Context, key ContextKey) (T, error) {
	valueFromCtx := ctx.Value(key)
	if valueFromCtx == nil {
		return *new(T), fmt.Errorf("value not found in context")
	}
	value, ok := valueFromCtx.(T)
	if !ok {
		return *new(T), fmt.Errorf("invalid value in context")
	}
	return value, nil
}

func CorrelationIdToCtx(ctx context.Context, correlationId string) context.Context {
	return valueToCtx(ctx, CorrelationIdKey, correlationId)
}

func CorrelationIdFromCtx(ctx context.Context) (string, error) {
	value, err := valueFromCtx[string](ctx, CorrelationIdKey)
	if err != nil {
		return "", err
	}
	return value, nil
}
