package pubsub

import (
	"time"
)

type Option func(*options)

type SubscriptionOption func(*subscriptionOptions)

type PublishOption func(*publishOptions)

type options struct {
	logger           Logger
	hooks            Hooks
	defaultAck       time.Duration
	defaultProcess   time.Duration
	defaultExtension time.Duration
	defaultWorkers   int
	defaultBuffer    int
	retryPolicy      RetryPolicy
	encoder          Encoder
	decoder          Decoder
	dedupe           DeduplicationConfig
}

type subscriptionOptions struct {
	name            string
	ackDeadline     time.Duration
	processTimeout  time.Duration
	maxExtension    time.Duration
	workers         int
	buffer          int
	retryPolicy     RetryPolicy
	deadLetterTopic string
	dedupe          DeduplicationConfig
}

type publishOptions struct {
	orderingKey string
	attributes  map[string]string
	retryPolicy RetryPolicy
	encoder     Encoder
}

type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
	Jitter         float64
}

type DeduplicationConfig struct {
	Enabled bool
	TTL     time.Duration
	Size    int
}

func defaultOptions() options {
	return options{
		defaultAck:       20 * time.Second,
		defaultProcess:   30 * time.Second,
		defaultExtension: 60 * time.Second,
		defaultWorkers:   8,
		defaultBuffer:    512,
		retryPolicy: RetryPolicy{
			MaxAttempts:    5,
			InitialBackoff: 500 * time.Millisecond,
			MaxBackoff:     30 * time.Second,
			Multiplier:     2,
			Jitter:         0.2,
		},
		dedupe: DeduplicationConfig{Enabled: true, TTL: 5 * time.Minute, Size: 4096},
	}
}

func defaultSubscriptionOptions(parent options, topic string) subscriptionOptions {
	return subscriptionOptions{
		name:           topic,
		ackDeadline:    parent.defaultAck,
		processTimeout: parent.defaultProcess,
		maxExtension:   parent.defaultExtension,
		workers:        parent.defaultWorkers,
		buffer:         parent.defaultBuffer,
		retryPolicy:    parent.retryPolicy,
		dedupe:         parent.dedupe,
	}
}

func defaultPublishOptions(parent options) publishOptions {
	return publishOptions{
		attributes:  map[string]string{},
		retryPolicy: parent.retryPolicy,
		encoder:     parent.encoder,
	}
}

func WithLogger(logger Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

func WithHooks(h Hooks) Option {
	return func(o *options) {
		o.hooks = h
	}
}

func WithDefaultAckDeadline(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.defaultAck = d
		}
	}
}

func WithDefaultProcessTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.defaultProcess = d
		}
	}
}

func WithDefaultExtensionLimit(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.defaultExtension = d
		}
	}
}

func WithDefaultWorkers(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.defaultWorkers = n
		}
	}
}

func WithDefaultBuffer(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.defaultBuffer = n
		}
	}
}

func WithRetryPolicy(policy RetryPolicy) Option {
	return func(o *options) {
		o.retryPolicy = policy.normalized()
	}
}

func WithEncoder(enc Encoder) Option {
	return func(o *options) {
		o.encoder = enc
	}
}

func WithDecoder(dec Decoder) Option {
	return func(o *options) {
		o.decoder = dec
	}
}

func WithDeduplication(cfg DeduplicationConfig) Option {
	return func(o *options) {
		o.dedupe = cfg
	}
}

func WithSubscriptionAckDeadline(d time.Duration) SubscriptionOption {
	return func(o *subscriptionOptions) {
		if d > 0 {
			o.ackDeadline = d
		}
	}
}

func WithSubscriptionProcessTimeout(d time.Duration) SubscriptionOption {
	return func(o *subscriptionOptions) {
		if d > 0 {
			o.processTimeout = d
		}
	}
}

func WithSubscriptionMaxExtension(d time.Duration) SubscriptionOption {
	return func(o *subscriptionOptions) {
		if d > 0 {
			o.maxExtension = d
		}
	}
}

func WithSubscriptionConcurrency(n int) SubscriptionOption {
	return func(o *subscriptionOptions) {
		if n > 0 {
			o.workers = n
		}
	}
}

func WithSubscriptionBuffer(n int) SubscriptionOption {
	return func(o *subscriptionOptions) {
		if n > 0 {
			o.buffer = n
		}
	}
}

func WithSubscriptionRetry(policy RetryPolicy) SubscriptionOption {
	return func(o *subscriptionOptions) {
		o.retryPolicy = policy.normalized()
	}
}

func WithSubscriptionDeadLetter(topic string) SubscriptionOption {
	return func(o *subscriptionOptions) {
		o.deadLetterTopic = topic
	}
}

func WithSubscriptionDeduplication(cfg DeduplicationConfig) SubscriptionOption {
	return func(o *subscriptionOptions) {
		o.dedupe = cfg
	}
}

func WithOrderingKey(key string) PublishOption {
	return func(o *publishOptions) {
		o.orderingKey = key
	}
}

func WithAttributes(attrs map[string]string) PublishOption {
	return func(o *publishOptions) {
		if len(attrs) == 0 {
			return
		}
		if o.attributes == nil {
			o.attributes = map[string]string{}
		}
		for k, v := range attrs {
			o.attributes[k] = v
		}
	}
}

func WithPublishRetry(policy RetryPolicy) PublishOption {
	return func(o *publishOptions) {
		o.retryPolicy = policy.normalized()
	}
}

func WithPublishEncoder(enc Encoder) PublishOption {
	return func(o *publishOptions) {
		o.encoder = enc
	}
}

func (r RetryPolicy) normalized() RetryPolicy {
	if r.Multiplier <= 0 {
		r.Multiplier = 2
	}
	if r.InitialBackoff <= 0 {
		r.InitialBackoff = 200 * time.Millisecond
	}
	if r.MaxBackoff <= 0 {
		r.MaxBackoff = 30 * time.Second
	}
	if r.MaxAttempts <= 0 {
		r.MaxAttempts = 5
	}
	return r
}
