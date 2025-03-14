package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/infigaming-com/go-common/messaging"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/oauth2/google"
)

const (
	broker    = "bootstrap.aggregator-dev.europe-west1.managedkafka.robust-metrics-445612-t0.cloud.goog:9092"
	topic     = "test-topic"
	projectID = "robust-metrics-445612-t0" // Extract project ID for clarity
)

type ExampleMessage struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

var lg *zap.Logger

func init() {
	// Load .env file from the example directory
	envPath := "messaging/example/.env"
	if err := godotenv.Load(envPath); err != nil {
		// Don't panic, just log the error as we might have environment variables set another way
		fmt.Printf("Warning: Error loading .env file from %s: %v\n", envPath, err)
	}

	// Create a production logger with better defaults
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.OutputPaths = []string{"stdout"}
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel) // Set to debug level for more information
	var err error
	lg, err = config.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer lg.Sync()

	// Log that we've loaded environment variables
	lg.Info("Environment variables loaded",
		zap.String("env_file", envPath))

	// Verify required environment variables are set
	requiredEnvVars := []string{"KAFKA_USERNAME", "KAFKA_PASSWORD"}
	for _, envVar := range requiredEnvVars {
		if value := os.Getenv(envVar); value == "" {
			lg.Fatal("Required environment variable not set",
				zap.String("variable", envVar))
		} else {
			// Log that we found the variable (but not its value for security)
			lg.Info("Found required environment variable",
				zap.String("variable", envVar))
		}
	}

	// Verify GCP credentials at startup
	ctx := context.Background()
	lg.Info("Verifying GCP credentials")

	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		lg.Fatal("Failed to get GCP credentials",
			zap.Error(err))
	}
	lg.Info("Found GCP credentials",
		zap.String("project_id", creds.ProjectID))

	// Test token acquisition
	lg.Info("Acquiring OAuth token")
	token, err := creds.TokenSource.Token()
	if err != nil {
		lg.Fatal("Failed to get token",
			zap.Error(err))
	}
	lg.Info("Successfully acquired token",
		zap.String("token_type", token.TokenType),
		zap.Time("expires", token.Expiry),
		zap.Bool("valid", token.Valid()))
}

func main() {
	defer lg.Sync()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Create a WaitGroup to wait for both publisher and subscriber
	var wg sync.WaitGroup

	// Start publisher
	wg.Add(1)
	go func() {
		defer wg.Done()
		runPublisher(ctx)
	}()

	// Start subscriber
	wg.Add(1)
	go func() {
		defer wg.Done()
		runSubscriber(ctx)
	}()

	// Wait for interrupt signal
	<-sigChan
	lg.Info("Received interrupt signal, shutting down...")
	cancel()

	// Wait for publisher and subscriber to finish
	wg.Wait()
	lg.Info("Application shutdown complete")
}

func runPublisher(ctx context.Context) {
	lg.Info("Creating publisher",
		zap.String("broker", broker),
		zap.String("client_id", "example-publisher"),
		zap.String("project_id", projectID))

	// Create publisher with more configuration
	publisher, cleanup, err := messaging.NewKafkaPublisher(
		messaging.WithPublisherLogger(lg),
		messaging.WithPublisherBrokers([]string{broker}),
		messaging.WithPublisherClientID("example-publisher"),
		// Use SASL/PLAIN authentication instead of OAuth
		messaging.WithPublisherSaslPlain(os.Getenv("KAFKA_USERNAME"), os.Getenv("KAFKA_PASSWORD")),
		// Add session timeout
		messaging.WithPublisherSessionTimeout(30*time.Second),
	)
	if err != nil {
		lg.Fatal("Failed to create publisher",
			zap.Error(err),
			zap.String("broker", broker),
			zap.String("project_id", projectID),
		)
	}
	defer cleanup()

	lg.Info("Publisher created successfully")

	// Publish messages every second
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	messageCount := 0
	for {
		select {
		case <-ctx.Done():
			lg.Info("Publisher shutting down")
			return
		case <-ticker.C:
			messageCount++
			msg := ExampleMessage{
				ID:        fmt.Sprintf("msg-%d", messageCount),
				Content:   fmt.Sprintf("Message content %d", messageCount),
				Timestamp: time.Now(),
			}

			// Create message with metadata
			kafkaMsg := messaging.Message{
				Topic:   topic,
				Payload: msg, // Send the struct directly, no need for manual JSON marshaling
				Metadata: map[string]interface{}{
					"version": "1.0",
					"source":  "example-app",
					"count":   messageCount,
				},
			}

			// Publish message
			publishStart := time.Now()
			err = publisher.Publish(ctx, kafkaMsg)
			publishDuration := time.Since(publishStart)

			if err != nil {
				lg.Error("Failed to publish message",
					zap.String("id", msg.ID),
					zap.Error(err),
					zap.Duration("duration", publishDuration))
				continue
			}

			lg.Info("Published message",
				zap.String("id", msg.ID),
				zap.Time("timestamp", msg.Timestamp),
				zap.Duration("duration", publishDuration))
		}
	}
}

func runSubscriber(ctx context.Context) {
	lg.Info("Creating subscriber",
		zap.String("broker", broker),
		zap.String("client_id", "example-subscriber"),
		zap.String("group_id", "example-group"),
		zap.String("project_id", projectID))

	// Create subscriber
	subscriber, cleanup, err := messaging.NewKafkaSubscriber(
		messaging.WithSubscriberLogger(lg),
		messaging.WithSubscriberBrokers([]string{broker}),
		messaging.WithSubscriberClientID("example-subscriber"),
		messaging.WithGroupID("example-group"),
		// Use SASL/PLAIN authentication instead of OAuth
		messaging.WithSubscriberSaslPlain(os.Getenv("KAFKA_USERNAME"), os.Getenv("KAFKA_PASSWORD")),
		// Add session timeout
		messaging.WithSubscriberSessionTimeout(30*time.Second),
	)
	if err != nil {
		lg.Fatal("Failed to create subscriber",
			zap.Error(err),
			zap.String("broker", broker),
			zap.String("project_id", projectID),
		)
	}
	defer cleanup()

	lg.Info("Subscriber created successfully")

	// Monitor subscriber errors
	go func() {
		for err := range subscriber.Errors() {
			lg.Error("Subscriber error", zap.Error(err))
		}
	}()

	// Subscribe to topic
	lg.Info("Subscribing to topic", zap.String("topic", topic))
	err = subscriber.Subscribe(ctx, topic, func(msg messaging.Message) error {
		receiveTime := time.Now()

		// Convert the payload to JSON bytes first
		payloadBytes, err := json.Marshal(msg.Payload)
		if err != nil {
			lg.Error("Failed to marshal payload to JSON", zap.Error(err))
			return fmt.Errorf("failed to marshal payload to JSON: %w", err)
		}

		// Parse the message payload
		var exampleMsg ExampleMessage
		if err := json.Unmarshal(payloadBytes, &exampleMsg); err != nil {
			lg.Error("Failed to unmarshal message", zap.Error(err))
			return fmt.Errorf("failed to unmarshal message: %w", err)
		}

		// Calculate message latency
		latency := receiveTime.Sub(exampleMsg.Timestamp)

		// Log received message
		lg.Info("Received message",
			zap.String("id", exampleMsg.ID),
			zap.String("content", exampleMsg.Content),
			zap.Time("timestamp", exampleMsg.Timestamp),
			zap.Duration("latency", latency),
			zap.Any("metadata", msg.Metadata))

		return nil
	})
	if err != nil {
		lg.Fatal("Failed to subscribe",
			zap.Error(err),
			zap.String("topic", topic))
	}

	lg.Info("Successfully subscribed to topic", zap.String("topic", topic))

	// Wait for context cancellation
	<-ctx.Done()
	lg.Info("Subscriber shutting down")
}
