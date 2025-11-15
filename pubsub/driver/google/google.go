package google

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	gcppubsub "cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"github.com/infigaming-com/go-common/pubsub"
)

type Config struct {
	ProjectID       string
	CredentialsJSON []byte
	Endpoint        string
	UserAgent       string
	Client          *gcppubsub.Client
	Logger          pubsub.Logger
	Receive         ReceiveSettings
}

type ReceiveSettings struct {
	NumGoroutines          int
	MaxOutstandingMessages int
	MaxOutstandingBytes    int
	MaxExtension           time.Duration
}

type transport struct {
	client     *gcppubsub.Client
	ownsClient bool
	logger     pubsub.Logger
	receive    ReceiveSettings
}

func New(ctx context.Context, cfg Config) (pubsub.Transport, error) {
	var (
		client *gcppubsub.Client
		err    error
		owns   bool
	)

	if cfg.Client != nil {
		client = cfg.Client
	} else {
		if cfg.ProjectID == "" {
			return nil, errors.New("googlepubsub: project id required when client is not provided")
		}
		opts := make([]option.ClientOption, 0, 3)
		if len(cfg.CredentialsJSON) > 0 {
			opts = append(opts, option.WithCredentialsJSON(cfg.CredentialsJSON))
		}
		if cfg.Endpoint != "" {
			opts = append(opts, option.WithEndpoint(cfg.Endpoint))
		}
		if cfg.UserAgent != "" {
			opts = append(opts, option.WithUserAgent(cfg.UserAgent))
		}
		client, err = gcppubsub.NewClient(ctx, cfg.ProjectID, opts...)
		if err != nil {
			return nil, fmt.Errorf("googlepubsub: create client: %w", err)
		}
		owns = true
	}

	t := &transport{
		client:     client,
		ownsClient: owns,
		logger:     cfg.Logger,
		receive:    cfg.Receive,
	}
	if t.logger == nil {
		t.logger = noopLogger{}
	}
	return t, nil
}

func (t *transport) Publish(ctx context.Context, topic string, env *pubsub.Envelope) (string, error) {
	if topic == "" {
		return "", errors.New("googlepubsub: topic required")
	}
	if env == nil {
		env = &pubsub.Envelope{}
	}
	gTopic := t.client.Topic(topic)
	if env.OrderingKey != "" {
		gTopic.EnableMessageOrdering = true
	}
	msg := &gcppubsub.Message{
		Data:        append([]byte(nil), env.Data...),
		Attributes:  cloneMap(env.Attributes),
		OrderingKey: env.OrderingKey,
	}
	res := gTopic.Publish(ctx, msg)
	id, err := res.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("googlepubsub: publish: %w", err)
	}
	return id, nil
}

func (t *transport) Subscribe(ctx context.Context, subscription string, opts pubsub.TransportSubscribeOptions, handler pubsub.TransportHandler) error {
	if subscription == "" {
		return errors.New("googlepubsub: subscription required")
	}
	if handler == nil {
		return errors.New("googlepubsub: handler required")
	}
	sub := t.client.Subscription(subscription)
	settings := sub.ReceiveSettings
	if t.receive.NumGoroutines > 0 {
		settings.NumGoroutines = t.receive.NumGoroutines
	}
	if opts.Parallelism > 0 {
		settings.NumGoroutines = opts.Parallelism
	}
	if t.receive.MaxOutstandingMessages > 0 {
		settings.MaxOutstandingMessages = t.receive.MaxOutstandingMessages
	}
	if t.receive.MaxOutstandingBytes > 0 {
		settings.MaxOutstandingBytes = t.receive.MaxOutstandingBytes
	}
	if t.receive.MaxExtension > 0 {
		settings.MaxExtension = t.receive.MaxExtension
	}
	if opts.MaxExtension > 0 {
		settings.MaxExtension = opts.MaxExtension
	}
	sub.ReceiveSettings = settings

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu         sync.Mutex
		handlerErr error
	)

	err := sub.Receive(subCtx, func(msgCtx context.Context, m *gcppubsub.Message) {
		var (
			once sync.Once
			done = make(chan struct{})
		)

		ack := func() error {
			once.Do(func() {
				m.Ack()
				close(done)
			})
			return nil
		}

		nack := func() error {
			once.Do(func() {
				m.Nack()
				close(done)
			})
			return nil
		}

		modify := func(time.Duration) error { return nil }

		tm := &pubsub.TransportMessage{
			Envelope: pubsub.Envelope{
				ID:          m.ID,
				Data:        append([]byte(nil), m.Data...),
				Attributes:  cloneMap(m.Attributes),
				OrderingKey: m.OrderingKey,
			},
			ReceivedAt: m.PublishTime,
			Ack:        ack,
			Nack:       nack,
			Extend:     modify,
			Done:       done,
		}
		if m.DeliveryAttempt != nil {
			tm.Attempt = int(*m.DeliveryAttempt)
		}

		defer func() {
			if r := recover(); r != nil {
				t.logger.Error(msgCtx, "googlepubsub handler panic", "subscription", subscription, "panic", r)
				_ = nack()
				mu.Lock()
				if handlerErr == nil {
					handlerErr = fmt.Errorf("googlepubsub: handler panic: %v", r)
				}
				mu.Unlock()
				cancel()
			}
		}()

		if err := handler(msgCtx, tm); err != nil {
			mu.Lock()
			if handlerErr == nil {
				handlerErr = err
			}
			mu.Unlock()
			cancel()
		}
	})

	if handlerErr != nil {
		return handlerErr
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return handlerErr
		}
		return err
	}
	return nil
}

func (t *transport) Close(context.Context) error {
	if t.ownsClient {
		return t.client.Close()
	}
	return nil
}

type noopLogger struct{}

func (noopLogger) Debug(context.Context, string, ...any) {}
func (noopLogger) Info(context.Context, string, ...any)  {}
func (noopLogger) Warn(context.Context, string, ...any)  {}
func (noopLogger) Error(context.Context, string, ...any) {}

func cloneMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
