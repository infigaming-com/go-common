package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/coocood/freecache"
)

type freeCache struct {
	cache *freecache.Cache
}

// NewFreeCache creates a new FreeCache instance with the specified size in bytes
// Recommended size: 100MB = 100 * 1024 * 1024
func NewFreeCache(cache *freecache.Cache) Cache {
	return &freeCache{cache: cache}
}

func (c *freeCache) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	ttlSeconds := int(expiry.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 0 // No expiry
	}

	err := c.cache.Set([]byte(key), []byte(value), ttlSeconds)
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

func (c *freeCache) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	return false, ErrNotSupport
}

func (c *freeCache) Get(ctx context.Context, key string) (string, error) {
	data, err := c.cache.Get([]byte(key))
	if err != nil {
		if err == freecache.ErrNotFound {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return string(data), nil
}

func (c *freeCache) Sets(ctx context.Context, kvs map[string]string, expiry time.Duration) error {
	ttlSeconds := int(expiry.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 0 // No expiry
	}

	for key, value := range kvs {
		err := c.cache.Set([]byte(key), []byte(value), ttlSeconds)
		if err != nil {
			return fmt.Errorf("failed to set key %s: %w", key, err)
		}
	}
	return nil
}

func (c *freeCache) SetsNX(ctx context.Context, kvs map[string]string, expiry time.Duration) (map[string]bool, error) {
	return nil, ErrNotSupport
}

func (c *freeCache) Gets(ctx context.Context, keys []string) (map[string]string, error) {
	results := make(map[string]string)

	for _, key := range keys {
		data, err := c.cache.Get([]byte(key))
		if err != nil {
			if err == freecache.ErrNotFound {
				return nil, ErrKeyNotFound
			}
			return nil, fmt.Errorf("failed to get key %s: %w", key, err)
		}
		results[key] = string(data)
	}

	return results, nil
}

func (c *freeCache) Delete(ctx context.Context, key string) error {
	affected := c.cache.Del([]byte(key))
	if affected {
		return nil
	}
	return fmt.Errorf("key %s not found", key)
}

func (c *freeCache) Clear(ctx context.Context) error {
	c.cache.Clear()
	return nil
}
