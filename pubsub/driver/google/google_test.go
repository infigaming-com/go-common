package google_test

import (
	"context"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/infigaming-com/go-common/pubsub"
	"github.com/infigaming-com/go-common/pubsub/driver/google"
)

func TestTransportPublishSubscribe(t *testing.T) {
	ctx := context.Background()
	server := pstest.NewServer()
	defer server.Close()

	conn, err := grpc.DialContext(ctx, server.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	gcpClient, err := gcppubsub.NewClient(ctx, "test-project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer gcpClient.Close()

	topic, err := gcpClient.CreateTopic(ctx, "orders-topic")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if _, err := gcpClient.CreateSubscription(ctx, "orders-sub", gcppubsub.SubscriptionConfig{Topic: topic}); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	transport, err := google.New(ctx, google.Config{
		Client: gcpClient,
		Receive: google.ReceiveSettings{
			NumGoroutines:          1,
			MaxOutstandingMessages: 10,
		},
	})
	if err != nil {
		t.Fatalf("transport: %v", err)
	}

	client, err := pubsub.New(ctx, transport)
	if err != nil {
		t.Fatalf("pubsub client: %v", err)
	}

	received := make(chan string, 1)

	subscription, err := client.Subscribe("orders-sub", pubsub.HandlerFunc(func(ctx context.Context, msg *pubsub.Message) error {
		received <- string(msg.Data())
		return nil
	}))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if _, err := client.Publish(ctx, "orders-topic", map[string]string{"id": "42"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	case data := <-received:
		if data == "" {
			t.Fatal("expected data")
		}
	}

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := subscription.Stop(stopCtx); err != nil {
		t.Fatalf("stop subscription: %v", err)
	}

	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
