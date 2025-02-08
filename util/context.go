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
	value, ok := valueFromCtx.(T)
	if !ok {
		return *new(T), NewUtilError(ErrCodeInvalidValueInContext, fmt.Sprintf("%v is not of type %T on context", key, new(T)), nil, nil)
	}
	return value, nil
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
