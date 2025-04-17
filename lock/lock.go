package lock

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidLockKey  = errors.New("invalid lock key")
	ErrLockNotAcquired = errors.New("lock not acquired")
)

type Lock interface {
	Lock(ctx context.Context, key string, opts ...LockOption) (func(context.Context) error, error)
	TryLock(ctx context.Context, key string, opts ...LockOption) (func(context.Context) error, error)
}

type LockOptions struct {
	expiry     time.Duration
	retryDelay time.Duration
	retries    int
}

type LockOption func(*LockOptions)

func WithExpiry(expiry time.Duration) LockOption {
	return func(o *LockOptions) {
		o.expiry = expiry
	}
}

func WithRetryDelay(retryDelay time.Duration) LockOption {
	return func(o *LockOptions) {
		o.retryDelay = retryDelay
	}
}

func WithRetries(retries int) LockOption {
	return func(o *LockOptions) {
		o.retries = retries
	}
}
