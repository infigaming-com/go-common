package messaging

import (
	"context"
)

type Subscriber interface {
	Subscribe(ctx context.Context, topic string, handler func(msg Message) error) error
	Errors() <-chan error // Optional: channel to receive consumer errors
}
