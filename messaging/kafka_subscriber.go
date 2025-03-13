package messaging

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

type KafkaSubscriberConfig struct {
	kafkaConfig       KafkaConfig
	groupID           string
	rebalanceStrategy sarama.BalanceStrategy
	offsetReset       string
}

type KafkaSubscriber struct {
	config        KafkaSubscriberConfig
	consumerGroup sarama.ConsumerGroup
	handler       func(msg Message) error
	topic         string
	ctx           context.Context
	cancel        context.CancelFunc
	errChan       chan error
}

type KafkaSubscriberOption func(*KafkaSubscriberConfig)

func WithSubscriberLogger(logger *zap.Logger) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.kafkaConfig.lg = logger
	}
}

func WithSubscriberBrokers(brokers []string) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.kafkaConfig.brokers = brokers
	}
}

func WithSubscriberClientID(clientID string) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.kafkaConfig.clientID = clientID
	}
}

func WithSubscriberSaslPlain(username, password string) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.kafkaConfig.saslPlainAuth = NewKafkaPlainAuth(username, password)
		c.kafkaConfig.useSaslPlain = true
	}
}

func WithSubscriberGmkAuth() KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.kafkaConfig.saslTokenProvider = NewGmkTokenProvider(c.kafkaConfig.lg)
	}
}

func WithSubscriberSessionTimeout(timeout time.Duration) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.kafkaConfig.sessionTimeout = timeout
	}
}

func WithGroupID(groupID string) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.groupID = groupID
	}
}

func WithRebalanceStrategy(strategy sarama.BalanceStrategy) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.rebalanceStrategy = strategy
	}
}

// WithConsumerOffsetReset sets the offset reset strategy
// Valid values: OffsetResetNewest or OffsetResetOldest
func WithConsumerOffsetReset(reset string) KafkaSubscriberOption {
	return func(c *KafkaSubscriberConfig) {
		c.offsetReset = reset
	}
}

func defaultSubscriberConfig() KafkaSubscriberConfig {
	return KafkaSubscriberConfig{
		kafkaConfig:       defaultKafkaConfig(),
		groupID:           "default-group",
		rebalanceStrategy: sarama.NewBalanceStrategyRoundRobin(),
		offsetReset:       OffsetResetNewest,
	}
}

func NewKafkaSubscriber(subscriberOpts ...KafkaSubscriberOption) (Subscriber, func() error, error) {
	config := defaultSubscriberConfig()
	for _, opt := range subscriberOpts {
		opt(&config)
	}

	consumerConfig := sarama.NewConfig()
	consumerConfig.ClientID = config.kafkaConfig.clientID
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Consumer.Group.Rebalance.Strategy = config.rebalanceStrategy
	consumerConfig.Version = sarama.V2_3_0_0

	if config.kafkaConfig.saslTokenProvider != nil {
		consumerConfig.Net.TLS.Enable = true
		consumerConfig.Net.TLS.Config = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		consumerConfig.Net.SASL.Enable = true
		consumerConfig.Net.SASL.Mechanism = sarama.SASLTypeOAuth
		consumerConfig.Net.SASL.TokenProvider = config.kafkaConfig.saslTokenProvider
		consumerConfig.Net.SASL.Handshake = true
	} else if config.kafkaConfig.useSaslPlain && config.kafkaConfig.saslPlainAuth != nil {
		consumerConfig.Net.TLS.Enable = true
		consumerConfig.Net.TLS.Config = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		consumerConfig.Net.SASL.Enable = true
		consumerConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		username, password, err := config.kafkaConfig.saslPlainAuth.Credentials()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get SASL/PLAIN credentials: %w", err)
		}
		consumerConfig.Net.SASL.User = username
		consumerConfig.Net.SASL.Password = password
		consumerConfig.Net.SASL.Handshake = true
	}

	if config.offsetReset == OffsetResetOldest {
		consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	} else {
		consumerConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	}

	consumerGroup, err := sarama.NewConsumerGroup(config.kafkaConfig.brokers, config.groupID, consumerConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	subscriber := &KafkaSubscriber{
		config:        config,
		consumerGroup: consumerGroup,
		ctx:           ctx,
		cancel:        cancel,
		errChan:       make(chan error, 10),
	}

	// Monitor consumer group errors
	go func() {
		for err := range consumerGroup.Errors() {
			select {
			case subscriber.errChan <- err:
			default:
				// Channel full, log error
				config.kafkaConfig.lg.Error("Error channel full, dropping error",
					zap.Error(err),
					zap.String("topic", config.groupID))
			}
		}
	}()

	closer := func() error {
		cancel()
		close(subscriber.errChan)
		return consumerGroup.Close()
	}

	return subscriber, closer, nil
}

func (k *KafkaSubscriber) Subscribe(ctx context.Context, topic string, handler func(msg Message) error) error {
	if k.topic != "" {
		return fmt.Errorf("subscriber already subscribed to topic %s", k.topic)
	}

	k.handler = handler
	k.topic = topic

	go k.consumeLoop(ctx)

	return nil
}

func (k *KafkaSubscriber) Errors() <-chan error {
	return k.errChan
}

func (k *KafkaSubscriber) consumeLoop(ctx context.Context) {
	mergedCtx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-k.ctx.Done():
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()
	defer cancel()

	handler := &consumerGroupHandler{
		handler: k.handler,
		errChan: k.errChan,
	}

	for {
		select {
		case <-mergedCtx.Done():
			return
		default:
			if err := k.consumerGroup.Consume(mergedCtx, []string{k.topic}, handler); err != nil {
				k.errChan <- err
			}
		}
	}
}

type consumerGroupHandler struct {
	handler func(msg Message) error
	errChan chan<- error
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		var payload interface{}
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			h.errChan <- fmt.Errorf("failed to unmarshal message from topic %s, partition %d, offset %d: %w",
				msg.Topic, msg.Partition, msg.Offset, err)
			continue
		}

		metadata := make(map[string]interface{})
		for _, header := range msg.Headers {
			metadata[string(header.Key)] = string(header.Value)
		}

		message := Message{
			Topic:    msg.Topic,
			Payload:  payload,
			Metadata: metadata,
		}

		if err := h.handler(message); err != nil {
			h.errChan <- fmt.Errorf("error handling message from topic %s, partition %d, offset %d: %w",
				msg.Topic, msg.Partition, msg.Offset, err)
			continue
		}

		session.MarkMessage(msg, "")
	}
	return nil
}
