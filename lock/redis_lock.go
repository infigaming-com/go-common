package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
)

const keyPrefix = "locker:"

type RedisLockOptions struct {
	Addr           string `mapstructure:"ADDR"`
	DB             int64  `mapstructure:"DB"`
	ConnectTimeout int64  `mapstructure:"CONNECT_TIMEOUT"`
}

type redisLock struct {
	rs *redsync.Redsync
}

type redisLockOption func(*RedisLockOptions)

func WithRedisLockAddr(addr string) redisLockOption {
	return func(o *RedisLockOptions) {
		o.Addr = addr
	}
}

func WithRedisLockDB(db int64) redisLockOption {
	return func(o *RedisLockOptions) {
		o.DB = db
	}
}

func WithRedisLockConnectTimeout(connectTimeout int64) redisLockOption {
	return func(o *RedisLockOptions) {
		o.ConnectTimeout = connectTimeout
	}
}

func defaultRedisLockOptions() *RedisLockOptions {
	return &RedisLockOptions{
		Addr:           "localhost:6379",
		DB:             0,
		ConnectTimeout: 30,
	}
}

func defaultLockOptions() *LockOptions {
	return &LockOptions{
		timeout:    30 * time.Second,
		retryDelay: 200 * time.Millisecond,
		retries:    10,
	}
}

func NewRedisLock(opts ...redisLockOption) (Lock, func(), error) {
	redisLockOptions := defaultRedisLockOptions()
	for _, opt := range opts {
		opt(redisLockOptions)
	}

	client := goredislib.NewClient(&goredislib.Options{
		Addr: redisLockOptions.Addr,
		DB:   int(redisLockOptions.DB),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(redisLockOptions.ConnectTimeout)*time.Second)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to redis for lock: %w", err)
	}

	pool := goredis.NewPool(client)
	rs := redsync.New(pool)

	return &redisLock{
			rs: rs,
		}, func() {
			client.Close()
		}, nil
}

func createUnlock(mutex *redsync.Mutex) func(context.Context) error {
	return func(ctx context.Context) error {
		defer func() {
			_ = recover()
		}()

		ok, err := mutex.UnlockContext(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		return nil
	}
}

func (l *redisLock) Lock(ctx context.Context, key string, opts ...LockOption) (func(context.Context) error, error) {
	if key == "" {
		return nil, ErrInvalidLockKey
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	options := defaultLockOptions()
	for _, opt := range opts {
		opt(options)
	}

	prefixedKey := keyPrefix + key
	mutex := l.rs.NewMutex(prefixedKey,
		redsync.WithExpiry(options.timeout),
		redsync.WithRetryDelay(options.retryDelay),
		redsync.WithTries(options.retries),
	)

	err := mutex.LockContext(ctx)
	if err != nil {
		if errors.Is(err, redsync.ErrFailed) {
			return nil, ErrLockNotAcquired
		}
		return nil, err
	}

	return createUnlock(mutex), nil
}

func (l *redisLock) TryLock(ctx context.Context, key string, opts ...LockOption) (func(context.Context) error, error) {
	if key == "" {
		return nil, ErrInvalidLockKey
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	options := defaultLockOptions()
	for _, opt := range opts {
		opt(options)
	}

	prefixedKey := keyPrefix + key
	mutex := l.rs.NewMutex(prefixedKey,
		redsync.WithExpiry(options.timeout),
		redsync.WithTries(1),
	)

	err := mutex.LockContext(ctx)
	if err != nil {
		if errors.Is(err, redsync.ErrFailed) {
			return nil, ErrLockNotAcquired
		}
		return nil, err
	}

	return createUnlock(mutex), nil
}
