package pubsub_test

import (
	"context"
	"fmt"
	"time"

	gcppubsub "cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/infigaming-com/go-common/pubsub"
	"github.com/infigaming-com/go-common/pubsub/driver/google"
)

type orderCreated struct {
	ID string `json:"id"`
}

func ExampleClient_google() {
	ctx := context.Background()
	server := pstest.NewServer()
	defer server.Close()

	conn, err := grpc.DialContext(ctx, server.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	gcpClient, err := gcppubsub.NewClient(ctx, "test-project", option.WithGRPCConn(conn))
	if err != nil {
		panic(err)
	}
	defer gcpClient.Close()

	topic, err := gcpClient.CreateTopic(ctx, "orders-topic")
	if err != nil {
		panic(err)
	}

	_, err = gcpClient.CreateSubscription(ctx, "orders-sub", gcppubsub.SubscriptionConfig{Topic: topic})
	if err != nil {
		panic(err)
	}

	transport, err := google.New(ctx, google.Config{Client: gcpClient})
	if err != nil {
		panic(err)
	}

	client, err := pubsub.New(ctx, transport)
	if err != nil {
		panic(err)
	}

	done := make(chan struct{})

	_, err = client.Subscribe("orders-sub", pubsub.HandlerFunc(func(ctx context.Context, msg *pubsub.Message) error {
		var payload orderCreated
		if err := msg.Decode(ctx, &payload); err != nil {
			return err
		}
		fmt.Println("received", payload.ID)
		if err := msg.Ack(); err != nil {
			return err
		}
		close(done)
		return nil
	}))
	if err != nil {
		panic(err)
	}

	if _, err := client.Publish(ctx, "orders-topic", orderCreated{ID: "42"}); err != nil {
		panic(err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		panic("timeout waiting for message")
	}

	if err := client.Shutdown(ctx); err != nil {
		panic(err)
	}
	// Output: received 42
}
