package cache

import (
	"context"
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

func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *redisCache) Clear(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}
