package locker

import (
	"context"
	"errors"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const keyPrefix = "locker:"

type RedisLockerConfig struct {
	Addr           string `mapstructure:"ADDR"`
	DB             int64  `mapstructure:"DB"`
	ConnectTimeout int64  `mapstructure:"CONNECT_TIMEOUT"`
}

type redisLocker struct {
	lg      *zap.Logger
	cfg     *RedisLockerConfig
	rs      *redsync.Redsync
	options *LockerOptions
}

func getDefaultOptions() *LockerOptions {
	return &LockerOptions{
		timeout:    30 * time.Second,
		retryDelay: 200 * time.Millisecond,
		retries:    10,
	}
}

func NewRedisLocker(lg *zap.Logger, cfg *RedisLockerConfig) (Locker, func()) {
	client := goredislib.NewClient(&goredislib.Options{
		Addr: cfg.Addr,
		DB:   int(cfg.DB),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		lg.Fatal("failed to connect to redis for locker", zap.String("addr", cfg.Addr), zap.Int("db", int(cfg.DB)), zap.Error(err))
	}
	lg.Info("connected to redis for locker", zap.String("addr", cfg.Addr), zap.Int("db", int(cfg.DB)))

	pool := goredis.NewPool(client)
	rs := redsync.New(pool)

	return &redisLocker{
			lg:      lg,
			cfg:     cfg,
			rs:      rs,
			options: getDefaultOptions(),
		}, func() {
			client.Close()
			lg.Info("closed redis connection for locker", zap.String("addr", cfg.Addr), zap.Int("db", int(cfg.DB)))
		}
}

func createUnlocker(mutex *redsync.Mutex, lg *zap.Logger, key string) Unlocker {
	return func(ctx context.Context) error {
		defer func() {
			if r := recover(); r != nil {
				lg.Error("panic in unlocker", zap.String("key", key), zap.Any("recover", r))
			}
		}()

		ok, err := mutex.UnlockContext(ctx)
		if err != nil {
			lg.Error("failed to unlock", zap.String("key", key), zap.Error(err))
			return err
		}
		if !ok {
			lg.Debug("lock already released", zap.String("key", key))
			return nil
		}
		lg.Debug("lock released", zap.String("key", key))
		return nil
	}
}

func (l *redisLocker) Lock(ctx context.Context, key string, opts ...LockerOption) (Unlocker, error) {
	if key == "" {
		return nil, ErrInvalidLockerKey
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	options := *l.options
	for _, opt := range opts {
		opt(&options)
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
			l.lg.Debug("failed to acquire lock", zap.String("key", key), zap.Error(err))
			return nil, ErrLockNotAcquired
		}
		l.lg.Error("error acquiring lock", zap.String("key", key), zap.Error(err))
		return nil, err
	}

	l.lg.Debug("lock acquired", zap.String("key", key))
	return createUnlocker(mutex, l.lg, key), nil
}

func (l *redisLocker) TryLock(ctx context.Context, key string, opts ...LockerOption) (Unlocker, error) {
	if key == "" {
		return nil, ErrInvalidLockerKey
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	options := *l.options
	for _, opt := range opts {
		opt(&options)
	}

	prefixedKey := keyPrefix + key
	mutex := l.rs.NewMutex(prefixedKey,
		redsync.WithExpiry(options.timeout),
		redsync.WithTries(1),
	)

	err := mutex.LockContext(ctx)
	if err != nil {
		if errors.Is(err, redsync.ErrFailed) {
			l.lg.Debug("failed to acquire lock after retries", zap.String("key", key), zap.Error(err))
			return nil, ErrLockNotAcquired
		}
		l.lg.Error("error acquiring lock", zap.String("key", key), zap.Error(err))
		return nil, err
	}

	l.lg.Debug("lock acquired after retry", zap.String("key", key))
	return createUnlocker(mutex, l.lg, key), nil
}
