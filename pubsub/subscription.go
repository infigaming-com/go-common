package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/infigaming-com/go-common/pubsub/internal/backoff"
	"github.com/infigaming-com/go-common/pubsub/internal/worker"
)

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
	dedupe    *dedupeCache
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

	var dedupe *dedupeCache
	if opts.dedupe.Enabled {
		dedupe = newDedupeCache(opts.dedupe.Size, opts.dedupe.TTL)
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
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		err := s.transport.Subscribe(s.ctx, s.options.name, TransportSubscribeOptions{
			AckDeadline:  s.options.ackDeadline,
			MaxExtension: s.options.maxExtension,
			Parallelism:  s.options.workers,
		}, s.handleTransportMessage)
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			s.backoff.Reset()
			return
		}
		s.onReceiveError(err)
		delay := s.backoff.Next()
		s.logger.Warn(s.ctx, "subscription reconnect", "topic", s.Topic(), "delay", delay.String(), "err", err)
		sleepCtx, cancel := context.WithTimeout(s.ctx, delay)
		select {
		case <-sleepCtx.Done():
		case <-s.ctx.Done():
		}
		cancel()
		if s.ctx.Err() != nil {
			return
		}
	}
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
	if s.breaker.open() {
		s.logger.Warn(ctx, "subscription circuit open", "topic", s.Topic(), "message", raw.ID)
		return raw.Nack()
	}
	if s.dedupe != nil && s.dedupe.seen(raw.ID) {
		s.logger.Debug(ctx, "subscription dedupe drop", "topic", s.Topic(), "message", raw.ID)
		if err := raw.Ack(); err != nil {
			s.logger.Error(ctx, "dedupe ack failed", "topic", s.Topic(), "message", raw.ID, "err", err)
			return err
		}
		return nil
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

type dedupeCache struct {
	mu    sync.Mutex
	items map[string]time.Time
	size  int
	ttl   time.Duration
}

func newDedupeCache(size int, ttl time.Duration) *dedupeCache {
	if size <= 0 {
		size = 1024
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &dedupeCache{items: make(map[string]time.Time, size), size: size, ttl: ttl}
}

func (d *dedupeCache) seen(id string) bool {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if expiry, ok := d.items[id]; ok {
		if now.Before(expiry) {
			return true
		}
	}
	if len(d.items) >= d.size {
		d.evict(now)
	}
	d.items[id] = now.Add(d.ttl)
	return false
}

func (d *dedupeCache) evict(now time.Time) {
	for k, v := range d.items {
		if now.After(v) {
			delete(d.items, k)
		}
	}
	if len(d.items) <= d.size {
		return
	}
	for k := range d.items {
		delete(d.items, k)
		if len(d.items) <= d.size {
			return
		}
	}
}
