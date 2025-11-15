package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/infigaming-com/go-common/pubsub/internal/backoff"
)

type Client struct {
	transport Transport
	opts      options
	ctx       context.Context
	cancel    context.CancelFunc

	mu     sync.RWMutex
	subs   map[*subscription]struct{}
	closed bool
}

func New(ctx context.Context, transport Transport, opts ...Option) (*Client, error) {
	if transport == nil {
		return nil, errors.New("pubsub: transport required")
	}
	base := defaultOptions()
	for _, opt := range opts {
		opt(&base)
	}
	if base.encoder == nil {
		base.encoder = jsonCodec{}
	}
	if base.decoder == nil {
		base.decoder = jsonCodec{}
	}
	clientCtx, cancel := context.WithCancel(ctx)
	return &Client{
		transport: transport,
		opts:      base,
		ctx:       clientCtx,
		cancel:    cancel,
		subs:      map[*subscription]struct{}{},
	}, nil
}

func (c *Client) Publish(ctx context.Context, topic string, payload any, opts ...PublishOption) (string, error) {
	if topic == "" {
		return "", errors.New("pubsub: topic required")
	}
	if err := c.guard(); err != nil {
		return "", err
	}
	po := defaultPublishOptions(c.opts)
	for _, opt := range opts {
		opt(&po)
	}
	encoder := po.encoder
	if encoder == nil {
		encoder = c.opts.encoder
	}
	env, err := encoder.Encode(ctx, payload)
	if err != nil {
		return "", err
	}
	if env == nil {
		env = &Envelope{}
	}
	if env.Attributes == nil {
		env.Attributes = map[string]string{}
	}
	for k, v := range po.attributes {
		env.Attributes[k] = v
	}
	if po.orderingKey != "" && env.OrderingKey == "" {
		env.OrderingKey = po.orderingKey
	}
	policy := po.retryPolicy
	bo := backoff.New(backoff.Config{Initial: policy.InitialBackoff, Max: policy.MaxBackoff, Multiplier: policy.Multiplier, Jitter: policy.Jitter})
	var attempt int
	for {
		attempt++
		id, err := c.transport.Publish(ctx, topic, env)
		if err == nil {
			if c.opts.hooks.OnPublish != nil {
				c.opts.hooks.OnPublish(ctx, topic, cloneMap(env.Attributes))
			}
			return id, nil
		}
		if isPermanent(err) || attempt >= policy.MaxAttempts {
			if c.opts.hooks.OnPublishFail != nil {
				c.opts.hooks.OnPublishFail(ctx, topic, cloneMap(env.Attributes), err)
			}
			return "", err
		}
		delay := bo.Next()
		tmr := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			tmr.Stop()
			return "", ctx.Err()
		case <-tmr.C:
		}
	}
}

func (c *Client) Subscribe(topic string, handler Handler, opts ...SubscriptionOption) (Subscription, error) {
	if topic == "" {
		return nil, errors.New("pubsub: topic required")
	}
	if handler == nil {
		return nil, errors.New("pubsub: handler required")
	}
	if err := c.guard(); err != nil {
		return nil, err
	}
	sopts := defaultSubscriptionOptions(c.opts, topic)
	for _, opt := range opts {
		opt(&sopts)
	}
	sub := newSubscription(c.ctx, c, topic, handler, sopts)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New("pubsub: client closed")
	}
	c.subs[sub] = struct{}{}
	c.mu.Unlock()
	sub.start()
	return sub, nil
}

func (c *Client) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	subs := make([]*subscription, 0, len(c.subs))
	for sub := range c.subs {
		subs = append(subs, sub)
	}
	c.subs = map[*subscription]struct{}{}
	c.mu.Unlock()
	c.cancel()
	for _, sub := range subs {
		_ = sub.Stop(ctx)
	}
	return c.transport.Close(ctx)
}

func (c *Client) guard() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return errors.New("pubsub: client closed")
	}
	return nil
}

func (c *Client) decoder() Decoder {
	if c.opts.decoder != nil {
		return c.opts.decoder
	}
	return jsonCodec{}
}

func (c *Client) logger() Logger {
	if c.opts.logger != nil {
		return c.opts.logger
	}
	return noopLogger{}
}

type noopLogger struct{}

func (noopLogger) Debug(context.Context, string, ...any) {}
func (noopLogger) Info(context.Context, string, ...any)  {}
func (noopLogger) Warn(context.Context, string, ...any)  {}
func (noopLogger) Error(context.Context, string, ...any) {}

func isPermanent(err error) bool {
	var perm permanentError
	return errors.As(err, &perm)
}

func (c *Client) remove(sub *subscription) {
	c.mu.Lock()
	delete(c.subs, sub)
	c.mu.Unlock()
}

func (c *Client) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("pubsub Client subscriptions=%d", len(c.subs))
}
