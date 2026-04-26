package pubsub

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisDedupeStore returns a DedupeStore backed by Redis SETNX. Use it
// when handler side-effects (sending a Slack/Telegram notification, calling
// a third-party payment API, etc.) cannot tolerate duplicate execution
// across pods or process restarts.
//
// keyPrefix should namespace the keys per service AND per topic, e.g.
// "push:dedupe:wallet.balance.update.sub.push". Without a per-topic suffix,
// two subscriptions sharing the same Redis client would collide on
// platform-assigned message IDs. A trailing colon is stripped so that both
// "svc:dedupe:topic" and "svc:dedupe:topic:" produce identical keys.
//
// The redis.Cmdable parameter accepts both *redis.Client and
// redis.UniversalClient (which is required for cluster deployments).
//
// Panics if client is nil — using a non-functional dedupe store would lead
// to silent unbounded duplicate side effects, which is exactly the failure
// mode this type exists to prevent.
func NewRedisDedupeStore(client redis.Cmdable, keyPrefix string) DedupeStore {
	if client == nil {
		panic("pubsub.NewRedisDedupeStore: nil client")
	}
	return &redisDedupeStore{
		client: client,
		prefix: strings.TrimRight(keyPrefix, ":"),
	}
}

type redisDedupeStore struct {
	client redis.Cmdable
	prefix string
}

func (d *redisDedupeStore) Seen(ctx context.Context, id string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	key := d.prefix + ":" + id
	// SETNX returns true when the key was created (first sight), false when
	// it already existed (already seen, redelivery).
	created, err := d.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return !created, nil
}
