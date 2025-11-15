package pubsub

import (
	"context"
	"time"
)

// Transport represents a concrete broker implementation.
// Implementations must be safe for concurrent use.
type Transport interface {
	Publish(ctx context.Context, topic string, envelope *Envelope) (string, error)
	Subscribe(ctx context.Context, topic string, opts TransportSubscribeOptions, handler TransportHandler) error
	Close(ctx context.Context) error
}

// Envelope holds the broker-facing message.
type Envelope struct {
	ID          string
	Data        []byte
	Attributes  map[string]string
	OrderingKey string
	Attempt     int
}

// TransportMessage is passed from the transport to the library.
type TransportMessage struct {
	Envelope
	ReceivedAt time.Time
	Ack        func() error
	Nack       func() error
	Extend     func(deadline time.Duration) error
	Done       <-chan struct{}
}

// TransportHandler processes raw transport messages.
type TransportHandler func(context.Context, *TransportMessage) error

// TransportSubscribeOptions configures subscriptions at the transport level.
type TransportSubscribeOptions struct {
	AckDeadline  time.Duration
	MaxExtension time.Duration
	Parallelism  int
}
