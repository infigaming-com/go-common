package lock

import (
	"context"
	"errors"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
)

type redisLock struct {
	rs *redsync.Redsync
}

func defaultLockOptions() *LockOptions {
	return &LockOptions{
		expiry:     8 * time.Second,
		retryDelay: 50 * time.Millisecond,
		retries:    32,
	}
}

func NewRedisLock(client *redis.Client) (Lock, error) {
	pool := goredis.NewPool(client)
	rs := redsync.New(pool)
	return &redisLock{rs: rs}, nil
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
			return errors.New("failed to unlock")
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

	mutex := l.rs.NewMutex(key,
		redsync.WithExpiry(options.expiry),
		redsync.WithRetryDelay(options.retryDelay),
		redsync.WithTries(options.retries),
	)
	err := mutex.LockContext(ctx)
	if err != nil {
		var errTaken *redsync.ErrTaken
		if errors.As(err, &errTaken) {
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

	mutex := l.rs.NewMutex(key,
		redsync.WithExpiry(options.expiry),
		redsync.WithTries(1),
	)

	err := mutex.LockContext(ctx)
	if err != nil {
		var errTaken *redsync.ErrTaken
		if errors.As(err, &errTaken) {
			return nil, ErrLockNotAcquired
		}
		return nil, err
	}

	return createUnlock(mutex), nil
}
