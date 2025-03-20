package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisCacheConfig struct {
	Addr           string `mapstructure:"ADDR"`
	DB             int64  `mapstructure:"DB"`
	ConnectTimeout int64  `mapstructure:"CONNECT_TIMEOUT"`
}

type redisCache struct {
	lg     *zap.Logger
	client *redis.Client
}

func NewRedisCache(lg *zap.Logger, cfg *RedisCacheConfig) (Cache, func()) {
	client := redis.NewClient(&redis.Options{
		Addr: cfg.Addr,
		DB:   int(cfg.DB),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		lg.Fatal("failed to connect to redis for cache", zap.String("addr", cfg.Addr), zap.Int("db", int(cfg.DB)), zap.Error(err))
	}
	lg.Info("connected to redis for cache", zap.String("addr", cfg.Addr), zap.Int("db", int(cfg.DB)))

	return &redisCache{
			lg:     lg,
			client: client,
		}, func() {
			client.Close()
			lg.Info("closed redis connection for cache", zap.String("addr", cfg.Addr), zap.Int("db", int(cfg.DB)))
		}
}

func (c *redisCache) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	return c.client.Set(ctx, key, value, expiry).Err()
}

func (c *redisCache) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, expiry).Result()
}

func (c *redisCache) Get(ctx context.Context, key string) (string, error) {
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrKeyNotFound
		}
		return "", err
	}

	return data, nil
}

func (c *redisCache) Sets(ctx context.Context, kvs map[string]string, expiry time.Duration) error {
	pipe := c.client.Pipeline()

	for key, value := range kvs {
		pipe.Set(ctx, key, value, expiry)
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute pipeline: %w", err)
	}

	for i, cmd := range cmds {
		if cmd.Err() != nil {
			return fmt.Errorf("failed to set item %d: %w", i, cmd.Err())
		}
	}

	return nil
}

func (c *redisCache) SetsNX(ctx context.Context, kvs map[string]string, expiry time.Duration) (map[string]bool, error) {
	results := make(map[string]bool, len(kvs))
	pipe := c.client.Pipeline()

	cmds := make(map[string]*redis.BoolCmd, len(kvs))
	for key, value := range kvs {
		cmds[key] = pipe.SetNX(ctx, key, value, expiry)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return results, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	for key, cmd := range cmds {
		success, err := cmd.Result()
		if err != nil {
			return results, fmt.Errorf("failed to get result for key %s: %w", key, err)
		}
		results[key] = success
	}

	return results, nil
}

func (c *redisCache) Gets(ctx context.Context, keys []string) (map[string]string, error) {
	results := make(map[string]string)

	pipe := c.client.Pipeline()

	for _, key := range keys {
		pipe.Get(ctx, key)
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return results, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	for i, cmd := range cmds {
		if cmd.Err() == redis.Nil {
			continue
		}
		if cmd.Err() != nil {
			continue
		}
		if strCmd, ok := cmd.(*redis.StringCmd); ok {
			value, err := strCmd.Result()
			if err == nil {
				results[keys[i]] = value
			}
		}
	}

	return results, nil
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *redisCache) Clear(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}
