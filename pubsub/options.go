package pubsub

import (
	"time"
)

type Option func(*options)

type SubscriptionOption func(*subscriptionOptions)

type PublishOption func(*publishOptions)

type options struct {
	logger                   Logger
	hooks                    Hooks
	defaultAck               time.Duration
	defaultProcess           time.Duration
	defaultExtension         time.Duration
	defaultWorkers           int
	defaultBuffer            int
	defaultStreamRefresh     time.Duration
	defaultInactivityTimeout time.Duration
	retryPolicy              RetryPolicy
	encoder                  Encoder
	decoder                  Decoder
	dedupe                   DeduplicationConfig
}

type subscriptionOptions struct {
	name           string
	ackDeadline    time.Duration
	processTimeout time.Duration
	maxExtension   time.Duration
	workers        int
	buffer         int
	// streamRefreshInterval forces the underlying Receive call to be torn down
	// and re-established on a fixed cadence. Guards against zombie
	// StreamingPull streams where the gRPC connection is alive but the server
	// stops pushing. Zero disables.
	//
	// NOTE: On GCP Pub/Sub, ack IDs are bound to the pull session. Any message
	// still in flight when a refresh happens will fail to ack (INVALID_ACK_ID)
	// and will be redelivered after the ack deadline. Handlers must therefore
	// be idempotent, and leaving the built-in deduplication cache enabled with
	// a TTL that covers the redelivery window is strongly recommended.
	// The default (30m) keeps the redelivery rate well under 50 events/day
	// per subscription; primary detection is the inactivity timeout below.
	streamRefreshInterval time.Duration
	// inactivityTimeout triggers a reconnect when no messages have been
	// observed on the stream for longer than this window. Zero disables.
	// This is the reactive detection path; streamRefreshInterval is the
	// preventive one.
	inactivityTimeout time.Duration
	retryPolicy       RetryPolicy
	deadLetterTopic   string
	dedupe            DeduplicationConfig
	// dedupeStore, if set, is consulted before in-memory dedupe and replaces
	// it. Use for shared (Redis-backed) dedupe across pods / restarts.
	dedupeStore DedupeStore
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
		defaultAck:               20 * time.Second,
		defaultProcess:           30 * time.Second,
		defaultExtension:         60 * time.Second,
		defaultWorkers:           8,
		defaultBuffer:            512,
		defaultStreamRefresh:     30 * time.Minute,
		defaultInactivityTimeout: 3 * time.Minute,
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
		name:                  topic,
		ackDeadline:           parent.defaultAck,
		processTimeout:        parent.defaultProcess,
		maxExtension:          parent.defaultExtension,
		workers:               parent.defaultWorkers,
		buffer:                parent.defaultBuffer,
		streamRefreshInterval: parent.defaultStreamRefresh,
		inactivityTimeout:     parent.defaultInactivityTimeout,
		retryPolicy:           parent.retryPolicy,
		dedupe:                parent.dedupe,
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

// WithDefaultStreamRefreshInterval sets how often every subscription tears down
// and reopens its underlying Receive call. Negative values disable; zero keeps
// the library default (5 minutes).
func WithDefaultStreamRefreshInterval(d time.Duration) Option {
	return func(o *options) {
		if d < 0 {
			o.defaultStreamRefresh = 0
			return
		}
		if d > 0 {
			o.defaultStreamRefresh = d
		}
	}
}

// WithDefaultInactivityTimeout sets the per-stream silence window after which a
// subscription forces a reconnect. Negative values disable; zero keeps the
// library default (3 minutes).
func WithDefaultInactivityTimeout(d time.Duration) Option {
	return func(o *options) {
		if d < 0 {
			o.defaultInactivityTimeout = 0
			return
		}
		if d > 0 {
			o.defaultInactivityTimeout = d
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

// WithSubscriptionDedupeStore replaces the in-memory dedupe cache with a
// caller-supplied DedupeStore for this subscription. Pair with a Redis-backed
// store (NewRedisDedupeStore) when handler side-effects must not duplicate
// across pods or process restarts (e.g. Slack/Telegram notifications,
// payment-channel calls).
//
// The TTL still comes from DeduplicationConfig.TTL — set it via
// WithSubscriptionDeduplication or rely on the client default. Setting the
// store implicitly enables dedupe regardless of DeduplicationConfig.Enabled.
func WithSubscriptionDedupeStore(store DedupeStore) SubscriptionOption {
	return func(o *subscriptionOptions) {
		o.dedupeStore = store
	}
}

// WithSubscriptionStreamRefreshInterval overrides the periodic stream refresh
// interval for a single subscription. Negative disables, zero keeps the
// client-level default.
func WithSubscriptionStreamRefreshInterval(d time.Duration) SubscriptionOption {
	return func(o *subscriptionOptions) {
		if d < 0 {
			o.streamRefreshInterval = 0
			return
		}
		if d > 0 {
			o.streamRefreshInterval = d
		}
	}
}

// WithSubscriptionInactivityTimeout overrides the inactivity reconnect window
// for a single subscription. Negative disables, zero keeps the client-level
// default.
func WithSubscriptionInactivityTimeout(d time.Duration) SubscriptionOption {
	return func(o *subscriptionOptions) {
		if d < 0 {
			o.inactivityTimeout = 0
			return
		}
		if d > 0 {
			o.inactivityTimeout = d
		}
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
