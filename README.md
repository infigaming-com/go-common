# go-common

## Pub/Sub Library Usage

The `pubsub` package provides a reusable client abstraction that encapsulates transport setup, subscriptions, message processing, and publishing with sensible defaults.

### Installation

```bash
go get github.com/infigaming-com/go-common@latest
```

### Creating a Client

```go
import (
    "context"
    "time"

    "github.com/infigaming-com/go-common/pubsub"
    googleDriver "github.com/infigaming-com/go-common/pubsub/driver/google"
)

func newClient(ctx context.Context, creds []byte) (*pubsub.Client, error) {
    transport, err := googleDriver.New(ctx, googleDriver.Config{
        ProjectID:       "your-gcp-project", // required when Client is nil
        CredentialsJSON: creds,               // omit to rely on ADC
        Receive: googleDriver.ReceiveSettings{
            NumGoroutines:          32,
            MaxOutstandingMessages: 5000,
        },
    })
    if err != nil {
        return nil, err
    }

    client, err := pubsub.New(ctx, transport,
        pubsub.WithDefaultAckDeadline(30*time.Second),
        pubsub.WithDefaultProcessTimeout(45*time.Second),
    )
    return client, err
}
```

Pass a prebuilt `*pubsub.Client` by setting `Config.Client` (for example using a `pstest` server or shared connection) or configure credentials via `option.WithCredentialsJSON`/`GOOGLE_APPLICATION_CREDENTIALS`. Use `WithLogger`, `WithHooks`, `WithRetryPolicy`, and other option helpers to integrate logging, metrics, and override defaults.

### Subscribing to a Topic

```go
type OrderCreated struct {
    ID string `json:"id"`
}

func handleOrder(ctx context.Context, msg *pubsub.Message) error {
    var payload OrderCreated
    if err := msg.Decode(ctx, &payload); err != nil {
        return pubsub.ErrPermanent(err)
    }
    // perform business logic
    return msg.Ack()
}

func registerSubscription(ctx context.Context, client *pubsub.Client) error {
    _, err := client.Subscribe("orders-sub", pubsub.HandlerFunc(handleOrder),
        pubsub.WithSubscriptionConcurrency(64),
        pubsub.WithSubscriptionAckDeadline(20*time.Second),
        pubsub.WithSubscriptionRetry(pubsub.RetryPolicy{MaxAttempts: 6}),
    )
    return err
}
```

Use subscription names that already exist in Google Cloud (e.g. `orders-sub`). Subscriptions automatically manage worker pools, ack deadlines, deduplication, and retries. Use options to configure per-topic overrides and dead-letter routing.

### Publishing Messages

```go
func publishOrder(ctx context.Context, client *pubsub.Client, id string) error {
    payload := OrderCreated{ID: id}
    _, err := client.Publish(ctx, "orders-topic", payload,
        pubsub.WithOrderingKey(id),
        pubsub.WithAttributes(map[string]string{"source": "orders-service"}),
    )
    return err
}
```

Topics must exist in Google Cloud (e.g. `orders-topic`). Publishing applies retries with exponential backoff and allows custom encoders or attributes.

### Graceful Shutdown

```go
func shutdown(ctx context.Context, client *pubsub.Client) error {
    return client.Shutdown(ctx)
}
```

Call `Shutdown` on service termination to flush in-flight messages and close transports cleanly.