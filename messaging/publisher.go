package messaging

import "context"

// PublishOption defines a function that modifies publish options
type PublishOption func(*publishOptions)

// publishOptions holds options for the Publish method
type publishOptions struct {
	keyGenerator KeyGenerator
}

// WithPublishKeyGenerator sets a key generator for a specific publish operation
func WithPublishKeyGenerator(keyGenerator KeyGenerator) PublishOption {
	return func(o *publishOptions) {
		o.keyGenerator = keyGenerator
	}
}

type Publisher interface {
	// Publish sends a message to the specified topic
	// Options can be provided to customize the publish operation
	Publish(ctx context.Context, msg Message, opts ...PublishOption) error
}
