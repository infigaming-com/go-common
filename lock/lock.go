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
	timeout    time.Duration
	retryDelay time.Duration
	retries    int
}

type LockOption func(*LockOptions)

func WithTimeout(timeout time.Duration) LockOption {
	return func(o *LockOptions) {
		o.timeout = timeout
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
