package util

import (
	"context"
	"fmt"

	"github.com/infigaming-com/go-common/errors"
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
		return *new(T), errors.NewError(ErrCodeValueNotFoundInContext, fmt.Sprintf("%v not found in context", key))
	}
	value, ok := valueFromCtx.(T)
	if !ok {
		return *new(T), errors.NewError(ErrCodeInvalidValueInContext, fmt.Sprintf("%v is not of type %T on context", key, new(T)))
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
