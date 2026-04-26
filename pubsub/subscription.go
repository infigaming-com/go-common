package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/infigaming-com/go-common/pubsub/internal/backoff"
	"github.com/infigaming-com/go-common/pubsub/internal/worker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxInactivityCheckInterval caps how often the watchdog polls LastActivity.
// Short enough to react promptly in prod, while the actual interval shrinks
// further when inactivityTimeout is small (e.g. in tests).
const maxInactivityCheckInterval = 30 * time.Second

// inactivityCheckInterval returns a poll cadence that is always at most half
// the inactivity window so the watchdog can detect a timeout within one window.
func inactivityCheckInterval(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return maxInactivityCheckInterval
	}
	half := timeout / 2
	if half < 10*time.Millisecond {
		half = 10 * time.Millisecond
	}
	if half > maxInactivityCheckInterval {
		return maxInactivityCheckInterval
	}
	return half
}

type Subscription interface {
	Topic() string
	Stop(ctx context.Context) error
	Health() SubscriptionHealth
}

type SubscriptionHealth struct {
	Topic         string
	Workers       int
	Buffered      int
	Failures      int
	CircuitOpen   bool
	LastError     string
	LastMessageID string
	LastActivity  time.Time
}

type subscription struct {
	client    *Client
	options   subscriptionOptions
	handler   Handler
	ctx       context.Context
	cancel    context.CancelFunc
	pool      *worker.Pool
	buffer    chan *TransportMessage
	backoff   *backoff.Exponential
	breaker   *breaker
	dedupe    DedupeStore
	dedupeTTL time.Duration
	hooks     Hooks
	logger    Logger
	transport Transport

	mu     sync.RWMutex
	health SubscriptionHealth
	closed bool
	wg     sync.WaitGroup
}

func newSubscription(parent context.Context, client *Client, topic string, handler Handler, opts subscriptionOptions) *subscription {
	subCtx, cancel := context.WithCancel(parent)
	p := worker.New(opts.workers, opts.buffer)
	h := SubscriptionHealth{Topic: topic, Workers: opts.workers}

	// An explicitly-injected DedupeStore wins (typically Redis-backed for
	// cross-pod, cross-restart correctness). Otherwise fall back to the
	// legacy in-memory cache when DeduplicationConfig.Enabled is true so
	// existing callers see no behaviour change.
	var dedupe DedupeStore
	switch {
	case opts.dedupeStore != nil:
		dedupe = opts.dedupeStore
	case opts.dedupe.Enabled:
		dedupe = newInMemoryDedupeStore(opts.dedupe.Size)
	}

	return &subscription{
		client:    client,
		options:   opts,
		handler:   handler,
		ctx:       subCtx,
		cancel:    cancel,
		pool:      p,
		buffer:    make(chan *TransportMessage, opts.buffer),
		backoff:   backoff.New(backoff.Config{Initial: opts.retryPolicy.InitialBackoff, Max: opts.retryPolicy.MaxBackoff, Multiplier: opts.retryPolicy.Multiplier, Jitter: opts.retryPolicy.Jitter}),
		breaker:   newBreaker(5, opts.retryPolicy.InitialBackoff*2),
		dedupe:    dedupe,
		dedupeTTL: opts.dedupe.TTL,
		hooks:     client.opts.hooks,
		logger:    client.logger(),
		transport: client.transport,
		health:    h,
	}
}

func (s *subscription) Topic() string { return s.options.name }

func (s *subscription) start() {
	s.wg.Add(2)
	go s.receiver()
	go s.dispatcher()
}

func (s *subscription) receiver() {
	defer s.wg.Done()
	for {
		if s.ctx.Err() != nil {
			return
		}

		// Each iteration opens a fresh Receive call under a child context the
		// watchdog can cancel independently of the parent subscription context.
		receiveCtx, cancelReceive := context.WithCancel(s.ctx)
		watchdogDone := make(chan struct{})
		reason := make(chan string, 1)
		go s.streamWatchdog(receiveCtx, cancelReceive, reason, watchdogDone)

		err := s.transport.Subscribe(receiveCtx, s.options.name, TransportSubscribeOptions{
			AckDeadline:  s.options.ackDeadline,
			MaxExtension: s.options.maxExtension,
			Parallelism:  s.options.workers,
		}, s.handleTransportMessage)
		cancelReceive()
		<-watchdogDone

		// Parent context cancelled => we are shutting down; exit cleanly.
		if s.ctx.Err() != nil {
			return
		}

		// Transports may return nil for a clean close (e.g. inmem shutdown).
		// Mirror the pre-watchdog behaviour and exit.
		if err == nil {
			s.backoff.Reset()
			return
		}

		// Watchdog-initiated refresh: receiveCtx was cancelled intentionally.
		// Treat this as a normal reconnect (no backoff penalty) and loop.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			select {
			case r := <-reason:
				s.logger.Info(s.ctx, "subscription stream refreshed", "topic", s.Topic(), "reason", r)
				s.backoff.Reset()
				continue
			default:
				// Canceled without a watchdog reason and the parent ctx is
				// still live -- this should not happen with the default
				// transports. Fail loud and treat as transient so we reconnect
				// instead of silently exiting (the very failure mode this
				// watchdog was introduced to prevent).
				s.logger.Error(s.ctx, "subscription cancelled without watchdog reason, reconnecting",
					"topic", s.Topic(), "err", err)
			}
		}

		// Real transport error: classify and back off.
		s.onReceiveError(err)
		delay := s.backoff.Next()
		s.logReceiveError(err, delay)

		sleepCtx, cancel := context.WithTimeout(s.ctx, delay)
		select {
		case <-sleepCtx.Done():
		case <-s.ctx.Done():
		}
		cancel()
	}
}

// streamWatchdog forces the active Receive call to terminate when it looks
// stuck: either after a fixed refresh interval (preventive) or after no
// message activity for inactivityTimeout (reactive). The goroutine exits as
// soon as receiveCtx is done.
func (s *subscription) streamWatchdog(
	receiveCtx context.Context,
	cancelReceive context.CancelFunc,
	reason chan<- string,
	done chan<- struct{},
) {
	defer close(done)

	refresh := s.options.streamRefreshInterval
	inactivity := s.options.inactivityTimeout
	if refresh <= 0 && inactivity <= 0 {
		<-receiveCtx.Done()
		return
	}

	streamStart := time.Now()
	var refreshC <-chan time.Time
	if refresh > 0 {
		t := time.NewTicker(refresh)
		defer t.Stop()
		refreshC = t.C
	}
	var inactivityC <-chan time.Time
	if inactivity > 0 {
		t := time.NewTicker(inactivityCheckInterval(inactivity))
		defer t.Stop()
		inactivityC = t.C
	}

	trigger := func(r string) {
		select {
		case reason <- r:
		default:
		}
		cancelReceive()
	}

	for {
		select {
		case <-receiveCtx.Done():
			return
		case <-refreshC:
			trigger("periodic_refresh")
			return
		case <-inactivityC:
			s.mu.RLock()
			last := s.health.LastActivity
			s.mu.RUnlock()
			var stale bool
			if !last.IsZero() {
				stale = time.Since(last) > inactivity
			} else {
				// No message ever observed on this stream. Only flag as stuck
				// after twice the inactivity window to avoid false positives
				// on genuinely idle topics during their first minutes.
				stale = time.Since(streamStart) > inactivity*2
			}
			if stale {
				lastStr := "never"
				if !last.IsZero() {
					lastStr = last.Format(time.RFC3339)
				}
				s.logger.Error(s.ctx, "subscription stream inactive, forcing reconnect",
					"topic", s.Topic(),
					"last_activity", lastStr,
					"stream_age", time.Since(streamStart).String())
				trigger("inactivity_timeout")
				return
			}
		}
	}
}

// logReceiveError downgrades transient transport failures to WARN and surfaces
// permanent configuration errors (NotFound, PermissionDenied, ...) as ERROR so
// log-based alerts can catch them.
func (s *subscription) logReceiveError(err error, delay time.Duration) {
	code := grpcCode(err)
	switch code {
	case codes.NotFound, codes.PermissionDenied, codes.Unauthenticated,
		codes.InvalidArgument, codes.FailedPrecondition:
		s.logger.Error(s.ctx, "subscription misconfigured, retry is futile until fixed",
			"topic", s.Topic(),
			"code", code.String(),
			"delay", delay.String(),
			"err", err)
	default:
		s.logger.Warn(s.ctx, "subscription reconnect",
			"topic", s.Topic(),
			"delay", delay.String(),
			"err", err)
	}
}

func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if st, ok := status.FromError(err); ok {
		return st.Code()
	}
	return codes.Unknown
}

func (s *subscription) dispatcher() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			s.pool.Close()
			s.pool.Wait()
			return
		case msg, ok := <-s.buffer:
			if !ok {
				s.pool.Close()
				s.pool.Wait()
				return
			}
			s.schedule(msg)
		}
	}
}

func (s *subscription) handleTransportMessage(ctx context.Context, raw *TransportMessage) error {
	if raw == nil {
		return nil
	}
	// Mark the stream as alive the moment a message arrives from the transport,
	// independent of whether the handler succeeds. The watchdog reads this to
	// decide whether StreamingPull has gone silent.
	s.touchActivity()
	if s.breaker.open() {
		s.logger.Warn(ctx, "subscription circuit open", "topic", s.Topic(), "message", raw.ID)
		return raw.Nack()
	}
	if s.dedupe != nil {
		seen, err := s.dedupe.Seen(ctx, raw.ID, s.dedupeTTL)
		if err != nil {
			// Fail open: a flaky dedupe store should never block delivery.
			// Reprocessing a message is far cheaper than dropping one.
			s.logger.Warn(ctx, "subscription dedupe check failed, processing message",
				"topic", s.Topic(), "message", raw.ID, "err", err)
		} else if seen {
			s.logger.Debug(ctx, "subscription dedupe drop", "topic", s.Topic(), "message", raw.ID)
			if err := raw.Ack(); err != nil {
				s.logger.Error(ctx, "dedupe ack failed", "topic", s.Topic(), "message", raw.ID, "err", err)
				return err
			}
			return nil
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	case s.buffer <- raw:
		meta := MessageMetadata{ID: raw.ID, Attempt: raw.Attempt, Attributes: cloneMap(raw.Attributes)}
		if s.hooks.OnReceive != nil {
			s.hooks.OnReceive(ctx, s.Topic(), meta)
		}
		return nil
	}
}

func (s *subscription) touchActivity() {
	s.mu.Lock()
	s.health.LastActivity = time.Now()
	s.mu.Unlock()
}

func (s *subscription) schedule(raw *TransportMessage) {
	meta := MessageMetadata{ID: raw.ID, Attempt: raw.Attempt, Attributes: cloneMap(raw.Attributes)}
	msg := newMessage(raw, s.client.decoder())
	deadlineCtx, cancel := context.WithTimeout(s.ctx, s.options.processTimeout)
	err := s.pool.Submit(deadlineCtx, func(execCtx context.Context) {
		defer cancel()
		s.process(execCtx, msg, meta)
	})
	if err != nil {
		cancel()
		s.logger.Error(s.ctx, "failed to submit message", "topic", s.Topic(), "message", raw.ID, "err", err)
		_ = msg.Nack()
	}
}

func (s *subscription) process(ctx context.Context, msg *Message, meta MessageMetadata) {
	start := time.Now()
	if msg == nil {
		return
	}
	extendStop := make(chan struct{})
	var extendWG sync.WaitGroup
	if s.options.maxExtension > 0 {
		extendWG.Add(1)
		go s.extendLoop(ctx, msg, extendStop, &extendWG, meta)
	}
	defer func() {
		close(extendStop)
		extendWG.Wait()
	}()

	err := s.handler.Handle(ctx, msg)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("handler timeout: %w", ctx.Err())
	}
	if err == nil {
		s.onSuccess(ctx, msg, meta, start)
		return
	}
	var permErr permanentError
	if errors.As(err, &permErr) {
		s.onPermanentFailure(ctx, msg, meta, permErr.Err)
		return
	}
	s.onFailure(ctx, msg, meta, err)
}

func (s *subscription) extendLoop(ctx context.Context, msg *Message, stop <-chan struct{}, wg *sync.WaitGroup, meta MessageMetadata) {
	defer wg.Done()
	deadline := s.options.ackDeadline
	if deadline <= 0 {
		return
	}
	max := s.options.maxExtension
	if max <= 0 {
		return
	}
	interval := deadline / 2
	if interval <= 0 {
		interval = deadline
	}
	var extended time.Duration
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if extended >= max {
				return
			}
			err := msg.Extend(deadline)
			if err != nil {
				s.logger.Warn(ctx, "extend failed", "topic", s.Topic(), "message", msg.ID(), "err", err)
				return
			}
			extended += deadline
			if s.hooks.OnAckExtend != nil {
				s.hooks.OnAckExtend(ctx, s.Topic(), meta, deadline.String())
			}
		}
	}
}

func (s *subscription) onSuccess(ctx context.Context, msg *Message, meta MessageMetadata, start time.Time) {
	if err := msg.Ack(); err != nil {
		s.logger.Error(ctx, "ack failed", "topic", s.Topic(), "message", msg.ID(), "err", err)
	}
	s.breaker.reset()
	s.recordHealth(meta.ID, false, "")
	if s.hooks.OnSuccess != nil {
		s.hooks.OnSuccess(ctx, s.Topic(), meta)
	}
	s.logger.Debug(ctx, "message processed", "topic", s.Topic(), "message", msg.ID(), "duration", time.Since(start))
}

func (s *subscription) onPermanentFailure(ctx context.Context, msg *Message, meta MessageMetadata, err error) {
	s.logger.Warn(ctx, "permanent failure", "topic", s.Topic(), "message", msg.ID(), "err", err)
	s.forwardDeadLetter(ctx, msg, meta)
	if err := msg.Ack(); err != nil {
		s.logger.Error(ctx, "ack after permanent failure", "topic", s.Topic(), "message", msg.ID(), "err", err)
	}
	s.recordHealth(meta.ID, false, err.Error())
	if s.hooks.OnFailure != nil {
		s.hooks.OnFailure(ctx, s.Topic(), meta, err)
	}
}

func (s *subscription) onFailure(ctx context.Context, msg *Message, meta MessageMetadata, err error) {
	attempt := meta.Attempt + 1
	if attempt >= s.options.retryPolicy.MaxAttempts {
		s.onPermanentFailure(ctx, msg, meta, err)
		return
	}
	if s.hooks.OnRetry != nil {
		s.hooks.OnRetry(ctx, s.Topic(), meta, attempt, "")
	}
	s.breaker.fail()
	s.recordHealth(meta.ID, true, err.Error())
	if nackErr := msg.Nack(); nackErr != nil {
		s.logger.Error(ctx, "nack failed", "topic", s.Topic(), "message", msg.ID(), "err", nackErr)
	}
	if s.hooks.OnFailure != nil {
		s.hooks.OnFailure(ctx, s.Topic(), meta, err)
	}
}

func (s *subscription) forwardDeadLetter(ctx context.Context, msg *Message, meta MessageMetadata) {
	if s.options.deadLetterTopic == "" {
		return
	}
	payload := map[string]any{
		"message_id": msg.ID(),
		"attributes": msg.Attributes(),
		"data":       msg.Data(),
	}
	_, err := s.client.Publish(ctx, s.options.deadLetterTopic, payload, WithAttributes(map[string]string{"source": s.Topic()}))
	if err != nil {
		s.logger.Error(ctx, "dead letter publish failed", "topic", s.Topic(), "message", msg.ID(), "err", err)
	}
}

func (s *subscription) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		s.client.remove(s)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *subscription) Health() SubscriptionHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.health
}

func (s *subscription) recordHealth(messageID string, failure bool, lastErr string) {
	s.mu.Lock()
	s.health.Buffered = len(s.buffer)
	if failure {
		s.health.Failures++
	}
	s.health.CircuitOpen = s.breaker.open()
	s.health.LastError = lastErr
	s.health.LastMessageID = messageID
	s.health.LastActivity = time.Now()
	s.mu.Unlock()
}

func (s *subscription) onReceiveError(err error) {
	if s.hooks.OnConnectionErr != nil {
		s.hooks.OnConnectionErr(s.ctx, s.Topic(), err)
	}
}

type breaker struct {
	mu        sync.Mutex
	failures  int
	threshold int
	trippedAt time.Time
	window    time.Duration
}

func newBreaker(threshold int, window time.Duration) *breaker {
	if threshold <= 0 {
		threshold = 5
	}
	if window <= 0 {
		window = 10 * time.Second
	}
	return &breaker{threshold: threshold, window: window}
}

func (b *breaker) fail() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.trippedAt = time.Now()
	}
}

func (b *breaker) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.trippedAt = time.Time{}
}

func (b *breaker) open() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.trippedAt.IsZero() {
		return false
	}
	if time.Since(b.trippedAt) > b.window {
		b.failures = 0
		b.trippedAt = time.Time{}
		return false
	}
	return true
}
