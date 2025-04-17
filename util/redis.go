package util

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, addr string, db int64, connectTimeout time.Duration) (*redis.Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   int(db),
	})
	timeoutCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	_, err := redisClient.Ping(timeoutCtx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis for cache: %w", err)
	}
	return redisClient, nil
}
