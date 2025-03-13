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

// KafkaPublisherConfig extends KafkaConfig with publisher-specific settings
type KafkaPublisherConfig struct {
	kafkaConfig  KafkaConfig  // Embed the common Kafka config
	keyGenerator KeyGenerator // Function to generate keys from messages
	partitioner  string       // Partitioning strategy: "hash", "random", "roundrobin", "manual"
}

// KafkaPublisher implements the Publisher interface for Kafka
type KafkaPublisher struct {
	config   KafkaPublisherConfig
	producer sarama.SyncProducer
}

// KafkaPublisherOption defines a function that modifies KafkaPublisherConfig
type KafkaPublisherOption func(*KafkaPublisherConfig)

// WithPublisherLogger sets the logger for the publisher
func WithPublisherLogger(logger *zap.Logger) KafkaPublisherOption {
	return func(c *KafkaPublisherConfig) {
		c.kafkaConfig.lg = logger
	}
}

// WithPublisherBrokers sets the broker addresses for the publisher
func WithPublisherBrokers(brokers []string) KafkaPublisherOption {
	return func(c *KafkaPublisherConfig) {
		c.kafkaConfig.brokers = brokers
	}
}

// WithPublisherClientID sets the client ID for the publisher
func WithPublisherClientID(clientID string) KafkaPublisherOption {
	return func(c *KafkaPublisherConfig) {
		c.kafkaConfig.clientID = clientID
	}
}

func WithPublisherSaslPlain(username, password string) KafkaPublisherOption {
	return func(c *KafkaPublisherConfig) {
		c.kafkaConfig.saslPlainAuth = NewKafkaPlainAuth(username, password)
		c.kafkaConfig.useSaslPlain = true
	}
}

func WithPublisherGmkAuth() KafkaPublisherOption {
	return func(config *KafkaPublisherConfig) {
		config.kafkaConfig.saslTokenProvider = NewGmkTokenProvider(config.kafkaConfig.lg)
	}
}

func WithPublisherSessionTimeout(timeout time.Duration) KafkaPublisherOption {
	return func(c *KafkaPublisherConfig) {
		c.kafkaConfig.sessionTimeout = timeout
	}
}

func WithKeyGenerator(keyGenerator KeyGenerator) KafkaPublisherOption {
	return func(c *KafkaPublisherConfig) {
		c.keyGenerator = keyGenerator
	}
}

func WithPartitioner(partitioner string) KafkaPublisherOption {
	return func(c *KafkaPublisherConfig) {
		c.partitioner = partitioner
	}
}

func defaultPublisherConfig() KafkaPublisherConfig {
	return KafkaPublisherConfig{
		kafkaConfig: defaultKafkaConfig(),
		keyGenerator: func(msg Message) string {
			return ""
		},
		partitioner: PartitionerHash,
	}
}

// NewKafkaPublisher creates a new Kafka publisher
func NewKafkaPublisher(publisherOpts ...KafkaPublisherOption) (Publisher, func() error, error) {
	config := defaultPublisherConfig()
	for _, opt := range publisherOpts {
		opt(&config)
	}

	producerConfig := sarama.NewConfig()
	producerConfig.Producer.Return.Successes = true // sync producer
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.ClientID = config.kafkaConfig.clientID
	producerConfig.Version = sarama.V2_3_0_0

	// Configure TLS and SASL if a token provider is set
	if config.kafkaConfig.saslTokenProvider != nil {
		// Enable TLS with secure configuration
		producerConfig.Net.TLS.Enable = true
		producerConfig.Net.TLS.Config = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// Enable SASL with OAUTHBEARER
		producerConfig.Net.SASL.Enable = true
		producerConfig.Net.SASL.Mechanism = sarama.SASLTypeOAuth
		producerConfig.Net.SASL.TokenProvider = config.kafkaConfig.saslTokenProvider

		// Set SASL handshake to true for GCP Managed Kafka
		producerConfig.Net.SASL.Handshake = true
	} else if config.kafkaConfig.useSaslPlain && config.kafkaConfig.saslPlainAuth != nil {
		// Enable TLS with secure configuration
		producerConfig.Net.TLS.Enable = true
		producerConfig.Net.TLS.Config = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// Enable SASL with PLAIN
		producerConfig.Net.SASL.Enable = true
		producerConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext

		// Set SASL credentials
		username, password, err := config.kafkaConfig.saslPlainAuth.Credentials()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get SASL/PLAIN credentials: %w", err)
		}
		producerConfig.Net.SASL.User = username
		producerConfig.Net.SASL.Password = password
		producerConfig.Net.SASL.Handshake = true
	}

	// Set partitioner based on strategy
	switch config.partitioner {
	case PartitionerHash:
		producerConfig.Producer.Partitioner = sarama.NewHashPartitioner
	case PartitionerRandom:
		producerConfig.Producer.Partitioner = sarama.NewRandomPartitioner
	case PartitionerRoundRobin:
		producerConfig.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	case PartitionerManual:
		producerConfig.Producer.Partitioner = sarama.NewManualPartitioner
	default:
		producerConfig.Producer.Partitioner = sarama.NewHashPartitioner
	}

	// Create and connect producer
	producer, err := sarama.NewSyncProducer(config.kafkaConfig.brokers, producerConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create producer: %w", err)
	}

	kp := &KafkaPublisher{
		config:   config,
		producer: producer,
	}

	// Return the publisher and a closer function
	closer := func() error {
		config.kafkaConfig.lg.Info(
			"closing kafka producer",
			zap.Strings("brokers", config.kafkaConfig.brokers),
			zap.String("client_id", config.kafkaConfig.clientID),
		)
		return producer.Close()
	}

	return kp, closer, nil
}

// Publish sends a message to the specified topic
// Options can be provided to customize the publish operation
func (k *KafkaPublisher) Publish(ctx context.Context, msg Message, opts ...PublishOption) error {
	if k.producer == nil {
		return fmt.Errorf("producer not initialized")
	}

	// Apply publish options
	options := publishOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	headers := make([]sarama.RecordHeader, 0, len(msg.Metadata))
	for key, value := range msg.Metadata {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte(key),
			Value: []byte(fmt.Sprintf("%v", value)),
		})
	}

	// Create producer message
	producerMsg := &sarama.ProducerMessage{
		Topic:   msg.Topic,
		Value:   sarama.ByteEncoder(payloadBytes),
		Headers: headers,
	}

	// get message level key generator first
	// and if not exists, use the publisher level key generator
	var keyGenerator KeyGenerator
	if options.keyGenerator != nil {
		keyGenerator = options.keyGenerator
	} else if k.config.keyGenerator != nil {
		keyGenerator = k.config.keyGenerator
	}

	if keyGenerator != nil {
		messageKey := keyGenerator(msg)
		if messageKey != "" {
			producerMsg.Key = sarama.StringEncoder(messageKey)
		}
	}

	_, _, err = k.producer.SendMessage(producerMsg)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	return nil
}
