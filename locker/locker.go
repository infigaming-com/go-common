package locker

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidLockerKey = errors.New("invalid locker key")
	ErrLockNotAcquired  = errors.New("lock not acquired")
)

type Unlocker func(ctx context.Context) error

type Locker interface {
	Lock(ctx context.Context, key string, opts ...LockerOption) (Unlocker, error)
	TryLock(ctx context.Context, key string, opts ...LockerOption) (Unlocker, error)
}

type LockerOptions struct {
	timeout    time.Duration
	retryDelay time.Duration
	retries    int
}

type LockerOption func(*LockerOptions)

func WithTimeout(timeout time.Duration) LockerOption {
	return func(o *LockerOptions) {
		o.timeout = timeout
	}
}

func WithRetryDelay(retryDelay time.Duration) LockerOption {
	return func(o *LockerOptions) {
		o.retryDelay = retryDelay
	}
}

func WithRetries(retries int) LockerOption {
	return func(o *LockerOptions) {
		o.retries = retries
	}
}
