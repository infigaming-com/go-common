package messaging

import (
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Partitioner strategy constants
const (
	PartitionerHash       = "hash"
	PartitionerRandom     = "random"
	PartitionerRoundRobin = "roundrobin"
	PartitionerManual     = "manual"
)

// Offset reset strategy constants
const (
	OffsetResetNewest = "newest"
	OffsetResetOldest = "oldest"
)

// KafkaConfig holds common configuration for Kafka clients
type KafkaConfig struct {
	lg                *zap.Logger                // Logger instance
	brokers           []string                   // Kafka broker addresses
	clientID          string                     // Client identifier
	useSaslPlain      bool                       // Whether to use SASL/PLAIN
	saslPlainAuth     *kafkaPlainAuth            // SASL PLAIN authenticator
	saslTokenProvider sarama.AccessTokenProvider // SASL token provider for OAuth
	sessionTimeout    time.Duration              // Session timeout for Kafka clients
}

func defaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		brokers:        []string{"localhost:9092"},
		clientID:       "go-common-kafka",
		lg:             zap.NewNop(),
		useSaslPlain:   true,
		saslPlainAuth:  NewKafkaPlainAuth("", ""),
		sessionTimeout: 30 * time.Second,
	}
}
