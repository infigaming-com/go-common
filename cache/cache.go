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
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

func CacheSetTyped[T any](ctx context.Context, cache Cache, key string, value T, expiry time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return ErrJsonMarshal
	}

	return cache.Set(ctx, key, string(data), expiry)
}

func CacheGetTyped[T any](ctx context.Context, cache Cache, key string) (T, error) {
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

func CacheDelete(ctx context.Context, cache Cache, key string) error {
	return cache.Delete(ctx, key)
}

func CacheClear(ctx context.Context, cache Cache) error {
	return cache.Clear(ctx)
}
