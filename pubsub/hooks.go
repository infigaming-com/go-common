package pubsub

import "context"

type Logger interface {
	Debug(ctx context.Context, msg string, kv ...any)
	Info(ctx context.Context, msg string, kv ...any)
	Warn(ctx context.Context, msg string, kv ...any)
	Error(ctx context.Context, msg string, kv ...any)
}

type Hooks struct {
	OnReceive       func(ctx context.Context, topic string, meta MessageMetadata)
	OnSuccess       func(ctx context.Context, topic string, meta MessageMetadata)
	OnFailure       func(ctx context.Context, topic string, meta MessageMetadata, err error)
	OnRetry         func(ctx context.Context, topic string, meta MessageMetadata, attempt int, delay string)
	OnAckExtend     func(ctx context.Context, topic string, meta MessageMetadata, extendBy string)
	OnPublish       func(ctx context.Context, topic string, meta map[string]string)
	OnPublishFail   func(ctx context.Context, topic string, meta map[string]string, err error)
	OnConnectionErr func(ctx context.Context, topic string, err error)
}

type MessageMetadata struct {
	ID         string
	Attempt    int
	Attributes map[string]string
}
