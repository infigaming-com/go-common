# Messaging Package

A flexible messaging abstraction for Go applications with Kafka implementation.

## Features

- Simple publisher/subscriber interfaces
- Kafka implementation with consumer groups
- Automatic key generation for partitioning
- Error handling via channels
- Context-based cancellation
- Flexible configuration via options pattern
- Separation of common and specific configurations

## Configuration Structure

The package uses a simplified configuration approach with private fields:

1. **KafkaConfig**: Common configuration for all Kafka clients
   - Private fields: logger, brokers, clientID, extra
   - All fields are only accessible through option functions

2. **KafkaPublisherConfig**: Publisher-specific configuration
   - Extends KafkaConfig with private fields: keyGenerator, partitioner
   - All configuration is done through KafkaPublisherOption functions

3. **KafkaSubscriberConfig**: Subscriber-specific configuration
   - Extends KafkaConfig with private fields: groupID, offsetReset
   - All configuration is done through KafkaSubscriberOption functions

This encapsulation ensures that all configuration is done through the option functions, providing better control over validation and defaults.

## Kafka Message Keys and Partitioning

### Why Keys Matter

In Kafka, message keys are crucial for:

1. **Partition Assignment**: Messages with the same key always go to the same partition
2. **Ordering Guarantee**: Kafka only guarantees order within a partition
3. **Load Balancing**: Well-distributed keys ensure even load across partitions
4. **Consumer Parallelism**: Each partition is consumed by one consumer in a group

### Key Generation

The package uses key generator functions to determine message keys. You can specify key generators at two levels:

1. **Publisher Level**: Set when creating the publisher, applies to all messages
```go
// Define a key generator that extracts user_id from the payload
userIDKeyGenerator := func(msg messaging.Message) string {
	if payload, ok := msg.Payload.(map[string]interface{}); ok {
		if userID, exists := payload["user_id"]; exists {
			return fmt.Sprintf("%v", userID)
		}
	}
	return "" // No key if user_id not found
}

// Use the key generator when creating a publisher
publisherOpts := []messaging.KafkaPublisherOption{
	messaging.WithKeyGenerator(userIDKeyGenerator),
}
```

2. **Message Level**: Set for a specific publish operation, overrides the publisher's key generator
```go
// Define a message-specific key generator
orderIDKeyGenerator := func(msg messaging.Message) string {
	if payload, ok := msg.Payload.(map[string]interface{}); ok {
		if orderID, exists := payload["order_id"]; exists {
			return fmt.Sprintf("order-%v", orderID)
		}
	}
	return ""
}

// Use the key generator for a specific message
err = publisher.Publish(ctx, message, messaging.WithPublishKeyGenerator(orderIDKeyGenerator))
```

If no key generator is provided at either level, messages will be distributed according to the configured partitioning strategy.

### Partitioning Strategies

The package supports different partitioning strategies through constants:

- **`PartitionerHash`** (default): Consistent partition assignment based on key hash
- **`PartitionerRandom`**: Random partition assignment (ignores keys)
- **`PartitionerRoundRobin`**: Distributes messages evenly across partitions
- **`PartitionerManual`**: Allows manual partition selection

Example usage:
```go
publisherOpts := []messaging.KafkaPublisherOption{
	messaging.WithPartitionerStrategy(messaging.PartitionerHash),
}
```

### Offset Reset Strategies

For subscribers, you can specify how to handle the initial offset when no committed offset exists:

- **`OffsetResetNewest`** (default): Start consuming from the newest messages
- **`OffsetResetOldest`**: Start consuming from the oldest messages

Example usage:
```go
subscriberOpts := []messaging.KafkaSubscriberOption{
	messaging.WithConsumerOffsetReset(messaging.OffsetResetOldest),
}
```

## Authentication with GCP Managed Kafka

This package supports two methods for authenticating with GCP Managed Kafka:

1. **SASL/PLAIN Authentication**: Using the `WithPublisherSaslPlain()` or `WithSubscriberSaslPlain()` options, which use username and password for authentication. This is the simplest method and works well when you have static credentials.

~~2. **Direct OAuth Authentication**: Using the `WithPublisherGmkAuth()` or `WithSubscriberGmkAuth()` options, which use the GCP service account credentials directly.~~

### Using SASL/PLAIN Authentication

To use SASL/PLAIN authentication:

```go
publisher, cleanup, err := messaging.NewKafkaPublisher(
    messaging.WithPublisherLogger(logger),
    messaging.WithPublisherBrokers([]string{broker}),
    messaging.WithPublisherClientID("example-publisher"),
    // Use SASL/PLAIN authentication
    messaging.WithPublisherSaslPlain("username", "password"),
)
```

For GCP Managed Kafka, follow the step below to get the username and password:
1. Create a service account with `Managed Kafka Client` role.
2. Create a key file with json format from this service account and save it to file like `key.json`.
3. Run the shell command below to get the password:
```bash
$ cat key.json | base64
```
4. The username is the principal of the service account which is in the format of `service_account_name@project_id.iam.gserviceaccount.com`.

For subscribers:

```go
subscriber, cleanup, err := messaging.NewKafkaSubscriber(
    messaging.WithSubscriberLogger(logger),
    messaging.WithSubscriberBrokers([]string{broker}),
    messaging.WithSubscriberClientID("example-subscriber"),
    messaging.WithGroupID("example-group"),
    // Use SASL/PLAIN authentication
    messaging.WithSubscriberSaslPlain("username", "password"),
)
```

It's recommended to store the credentials in environment variables or a secure configuration management system rather than hardcoding them in your code.

Example using environment variables:

```go
publisher, cleanup, err := messaging.NewKafkaPublisher(
    messaging.WithPublisherLogger(logger),
    messaging.WithPublisherBrokers([]string{broker}),
    messaging.WithPublisherClientID("example-publisher"),
    // Use SASL/PLAIN authentication with environment variables
    messaging.WithPublisherSaslPlain(os.Getenv("KAFKA_USERNAME"), os.Getenv("KAFKA_PASSWORD")),
)
```

## Usage Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/infigaming-com/go-common/messaging"
)

func main() {
	// Define a key generator that extracts user_id from the payload
	userIDKeyGenerator := func(msg messaging.Message) string {
		if payload, ok := msg.Payload.(map[string]interface{}); ok {
			if userID, exists := payload["user_id"]; exists {
				return fmt.Sprintf("%v", userID)
			}
		}
		return ""
	}

	// Publisher options
	publisherOpts := []messaging.KafkaPublisherOption{
		messaging.WithPublisherBrokers([]string{"localhost:9092"}),
		messaging.WithPublisherClientID("example-client"),
		messaging.WithKeyGenerator(userIDKeyGenerator),
		messaging.WithPartitionerStrategy(messaging.PartitionerHash),
		messaging.WithPublisherSaslPlain("username", "password"),
	}

	// Create publisher with options
	publisher, pubCleanup, err := messaging.NewKafkaPublisher(publisherOpts...)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pubCleanup()

	// Publish message - key will be generated from the payload using the publisher's key generator
	ctx := context.Background()
	err = publisher.Publish(ctx, messaging.Message{
		Topic: "orders",
		Payload: map[string]interface{}{
			"order_id": "ORD-001",
			"user_id":  "user-123", // This will be used as the key
			"amount":   99.95,
		},
	})
	if err != nil {
		log.Printf("Failed to publish: %v", err)
	}
	
	// Publish another message with a message-specific key generator
	orderIDKeyGenerator := func(msg messaging.Message) string {
		if payload, ok := msg.Payload.(map[string]interface{}); ok {
			if orderID, exists := payload["order_id"]; exists {
				return fmt.Sprintf("order-%v", orderID)
			}
		}
		return ""
	}
	
	err = publisher.Publish(ctx, messaging.Message{
		Topic: "orders",
		Payload: map[string]interface{}{
			"order_id": "ORD-002",
			"user_id":  "user-456",
			"amount":   149.95,
		},
	}, messaging.WithPublishKeyGenerator(orderIDKeyGenerator))
	if err != nil {
		log.Printf("Failed to publish: %v", err)
	}
}
```

## Subscriber Example

```go
// Subscriber options
subscriberOpts := []messaging.KafkaSubscriberOption{
	messaging.WithSubscriberBrokers([]string{"localhost:9092"}),
	messaging.WithSubscriberClientID("example-client"),
	messaging.WithGroupID("example-group"),
	messaging.WithConsumerOffsetReset(messaging.OffsetResetNewest),
	messaging.WithSubscriberSaslPlain("username", "password"),
}

// Create subscriber with options
subscriber, subCleanup, err := messaging.NewKafkaSubscriber(subscriberOpts...)
if err != nil {
	log.Fatalf("Failed to create subscriber: %v", err)
}
defer subCleanup()

// Handle errors from subscriber
go func() {
	for err := range subscriber.Errors() {
		log.Printf("Kafka error: %v", err)
	}
}()

// Subscribe to topic
ctx := context.Background()
err = subscriber.Subscribe(ctx, "orders", func(msg messaging.Message) error {
	log.Printf("Received message: %v", msg.Payload)
	return nil
})
```

## Best Practices

1. **Choose Key Generators Wisely**: 
   - Extract business identifiers that naturally group related messages
   - Common keys: user ID, order ID, session ID, etc.
   - Avoid using timestamps or random values as keys
   - Return empty string when no key is appropriate

2. **Key Generation Strategy**:
   - Create key generators that are deterministic and stable
   - Consider formatting keys consistently (e.g., prefixing with type)
   - Handle missing data gracefully

3. **Partitioning Strategy**:
   - Use hash partitioning (default) when ordering matters
   - Use random or round-robin when maximum throughput is more important than ordering

4. **Error Handling**:
   - Always monitor the error channel from subscribers
   - Consider implementing retry logic for failed messages

## Configuration Options

### Publisher Options (KafkaPublisherOption)
- `WithPublisherBrokers([]string)`: Kafka broker addresses
- `WithPublisherClientID(string)`: Client identifier
- `WithPublisherLogger(*zap.Logger)`: Logger instance
- `WithPublisherSessionTimeout(time.Duration)`: Session timeout
- `WithPublisherExtraOption(key, value string)`: Add custom configuration
- `WithKeyGenerator(func(Message) string)`: Function to generate keys from messages
- `WithPartitionerStrategy(string)`: Partitioning strategy

### Publish Options (PublishOption)
- `WithPublishKeyGenerator(func(Message) string)`: Override the publisher's key generator for a specific message

### Subscriber Options (KafkaSubscriberOption)
- `WithSubscriberBrokers([]string)`: Kafka broker addresses
- `WithSubscriberClientID(string)`: Client identifier
- `WithSubscriberLogger(*zap.Logger)`: Logger instance
- `WithSubscriberSessionTimeout(time.Duration)`: Session timeout
- `WithSubscriberExtraOption(key, value string)`: Add custom configuration
- `WithGroupID(string)`: Consumer group ID
- `WithConsumerOffsetReset(string)`: Consumer offset reset strategy