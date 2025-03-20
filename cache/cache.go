package cache

import (
	"context"
	"encoding/json"
	"time"
)

type Cache interface {
	Set(ctx context.Context, key string, value string, expiry time.Duration) error
	SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error)
	Get(ctx context.Context, key string) (string, error)
	Sets(ctx context.Context, kvs map[string]string, expiry time.Duration) error
	SetsNX(ctx context.Context, kvs map[string]string, expiry time.Duration) (map[string]bool, error)
	Gets(ctx context.Context, keys []string) (map[string]string, error)
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

func SetTyped[T any](ctx context.Context, cache Cache, key string, value T, expiry time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return ErrJsonMarshal
	}

	return cache.Set(ctx, key, string(data), expiry)
}

func SetNXTyped[T any](ctx context.Context, cache Cache, key string, value T, expiry time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, ErrJsonMarshal
	}

	return cache.SetNX(ctx, key, string(data), expiry)
}

func GetTyped[T any](ctx context.Context, cache Cache, key string) (T, error) {
	var result T

	value, err := cache.Get(ctx, key)
	if err != nil {
		return result, err
	}

	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return result, ErrJsonUnmarshal
	}

	return result, nil
}

func SetsTyped[T any](ctx context.Context, cache Cache, kvs map[string]T, expiry time.Duration) error {
	jsonKvs := make(map[string]string, len(kvs))
	for key, value := range kvs {
		data, err := json.Marshal(value)
		if err != nil {
			return ErrJsonMarshal
		}
		jsonKvs[key] = string(data)
	}
	return cache.Sets(ctx, jsonKvs, expiry)
}

func SetsNXTyped[T any](ctx context.Context, cache Cache, kvs map[string]T, expiry time.Duration) (map[string]bool, error) {
	jsonKvs := make(map[string]string, len(kvs))
	for key, value := range kvs {
		data, err := json.Marshal(value)
		if err != nil {
			return nil, ErrJsonMarshal
		}
		jsonKvs[key] = string(data)
	}
	return cache.SetsNX(ctx, jsonKvs, expiry)
}

func GetsTyped[T any](ctx context.Context, cache Cache, keys []string) (map[string]T, error) {
	results, err := cache.Gets(ctx, keys)
	if err != nil {
		return nil, err
	}

	resultMap := make(map[string]T, len(results))
	for key, value := range results {
		var result T
		if err := json.Unmarshal([]byte(value), &result); err != nil {
			return nil, ErrJsonUnmarshal
		}
		resultMap[key] = result
	}

	return resultMap, nil
}

func Delete(ctx context.Context, cache Cache, key string) error {
	return cache.Delete(ctx, key)
}

func Clear(ctx context.Context, cache Cache) error {
	return cache.Clear(ctx)
}
