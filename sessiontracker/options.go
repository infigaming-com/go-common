package sessiontracker

import "time"

// Option configures the Tracker.
type Option func(*Tracker)

// WithL1TTL sets the time-to-live for L1 (in-process) cache entries.
// Default: 5 minutes.
func WithL1TTL(d time.Duration) Option {
	return func(t *Tracker) {
		t.l1TTL = d
	}
}

// WithRedisKeyPrefix sets the prefix for Redis hash keys.
// Default: "session_ctx".
func WithRedisKeyPrefix(p string) Option {
	return func(t *Tracker) {
		t.redisKeyPrefix = p
	}
}

// WithL2TTL sets the time-to-live for L2 (Redis) cache entries.
// Default: 30 days.
func WithL2TTL(d time.Duration) Option {
	return func(t *Tracker) {
		t.l2TTL = d
	}
}

// WithCleanupInterval sets how often expired L1 entries are removed.
// Default: 10 minutes.
func WithCleanupInterval(d time.Duration) Option {
	return func(t *Tracker) {
		t.cleanupInterval = d
	}
}
