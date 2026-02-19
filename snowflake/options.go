package snowflake

import "time"

// ---------- Generator Options ----------

// Option configures a Generator.
type Option func(*generatorOptions)

type generatorOptions struct {
	maxClockDrift time.Duration
	metrics       MetricsHook
	leaseCheck    *NodeLease
	now           func() time.Time
}

func defaultGeneratorOptions() *generatorOptions {
	return &generatorOptions{
		maxClockDrift: 100 * time.Millisecond,
		metrics:       noopMetrics{},
		now:           time.Now,
	}
}

// WithMaxClockDrift sets the maximum tolerable clock rollback duration.
// If clock moves back more than this, NextID returns ErrClockRollback.
// Default: 100ms.
func WithMaxClockDrift(d time.Duration) Option {
	return func(o *generatorOptions) {
		o.maxClockDrift = d
	}
}

// WithMetrics sets the metrics hook for observability.
func WithMetrics(m MetricsHook) Option {
	return func(o *generatorOptions) {
		if m != nil {
			o.metrics = m
		}
	}
}

// WithLeaseHealthCheck enables lease health checking.
// If the lease becomes unhealthy, NextID returns ErrLeaseExpired.
func WithLeaseHealthCheck(nl *NodeLease) Option {
	return func(o *generatorOptions) {
		o.leaseCheck = nl
	}
}

// WithNowFunc overrides the time source (for testing).
func WithNowFunc(fn func() time.Time) Option {
	return func(o *generatorOptions) {
		if fn != nil {
			o.now = fn
		}
	}
}

// ---------- Lease Options ----------

// LeaseOption configures a NodeLease.
type LeaseOption func(*leaseOptions)

type leaseOptions struct {
	ttl         time.Duration
	serviceName string
	keyPrefix   string
	metrics     MetricsHook
}

func defaultLeaseOptions() *leaseOptions {
	return &leaseOptions{
		ttl:         30 * time.Second,
		serviceName: "unknown",
		keyPrefix:   "snowflake:node:",
		metrics:     noopMetrics{},
	}
}

// WithLeaseTTL sets the lease TTL. Heartbeat interval is TTL/3.
// Default: 30s.
func WithLeaseTTL(d time.Duration) LeaseOption {
	return func(o *leaseOptions) {
		if d > 0 {
			o.ttl = d
		}
	}
}

// WithServiceName sets the service name used in the holder identity.
func WithServiceName(name string) LeaseOption {
	return func(o *leaseOptions) {
		if name != "" {
			o.serviceName = name
		}
	}
}

// WithLeaseKeyPrefix sets the Redis key prefix for node lease keys.
// Default: "snowflake:node:".
func WithLeaseKeyPrefix(prefix string) LeaseOption {
	return func(o *leaseOptions) {
		if prefix != "" {
			o.keyPrefix = prefix
		}
	}
}

// WithLeaseMetrics sets the metrics hook for lease operations.
func WithLeaseMetrics(m MetricsHook) LeaseOption {
	return func(o *leaseOptions) {
		if m != nil {
			o.metrics = m
		}
	}
}
