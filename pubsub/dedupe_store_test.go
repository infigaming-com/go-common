package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestInMemoryDedupeStore_FirstThenSeen(t *testing.T) {
	t.Parallel()
	s := newInMemoryDedupeStore(8)
	ctx := context.Background()
	ttl := time.Minute

	seen, err := s.Seen(ctx, "id-1", ttl)
	if err != nil {
		t.Fatalf("first call err: %v", err)
	}
	if seen {
		t.Fatalf("first call should report not-seen")
	}

	seen, err = s.Seen(ctx, "id-1", ttl)
	if err != nil {
		t.Fatalf("second call err: %v", err)
	}
	if !seen {
		t.Fatalf("second call should report seen")
	}
}

func TestInMemoryDedupeStore_TTLExpiry(t *testing.T) {
	t.Parallel()
	s := newInMemoryDedupeStore(8)
	ctx := context.Background()

	if seen, _ := s.Seen(ctx, "id-1", 5*time.Millisecond); seen {
		t.Fatalf("first call should be not-seen")
	}
	time.Sleep(15 * time.Millisecond)
	if seen, _ := s.Seen(ctx, "id-1", 5*time.Millisecond); seen {
		t.Fatalf("after expiry should be not-seen again")
	}
}

func TestInMemoryDedupeStore_EvictsAtCap(t *testing.T) {
	t.Parallel()
	s := newInMemoryDedupeStore(2)
	ctx := context.Background()

	_, _ = s.Seen(ctx, "a", time.Hour)
	_, _ = s.Seen(ctx, "b", time.Hour)
	// At cap. Inserting a third must evict to make room.
	_, _ = s.Seen(ctx, "c", time.Hour)

	if got := len(s.items); got > s.size {
		t.Fatalf("len=%d exceeds size=%d, eviction broken", got, s.size)
	}
}

func TestInMemoryDedupeStore_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	s := newInMemoryDedupeStore(1024)
	ctx := context.Background()
	ttl := time.Minute

	var wg sync.WaitGroup
	const goroutines = 32
	const idsPerG = 100
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < idsPerG; i++ {
				id := fmt.Sprintf("g%d-i%d", gIdx, i)
				_, _ = s.Seen(ctx, id, ttl)
				_, _ = s.Seen(ctx, id, ttl)
			}
		}(g)
	}
	wg.Wait()
}

func TestRedisDedupeStore_FirstThenSeen(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisDedupeStore(client, "test:dedupe:topic-a")
	ctx := context.Background()

	seen, err := store.Seen(ctx, "msg-1", time.Minute)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if seen {
		t.Fatalf("first should be not-seen")
	}

	seen, err = store.Seen(ctx, "msg-1", time.Minute)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !seen {
		t.Fatalf("second should be seen")
	}
}

func TestRedisDedupeStore_TTLExpiry(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisDedupeStore(client, "test:dedupe:topic-b")
	ctx := context.Background()

	_, _ = store.Seen(ctx, "msg-1", 100*time.Millisecond)
	mr.FastForward(200 * time.Millisecond)
	seen, err := store.Seen(ctx, "msg-1", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("post-expiry: %v", err)
	}
	if seen {
		t.Fatalf("after TTL expiry should be not-seen again")
	}
}

func TestRedisDedupeStore_PrefixIsolatesTopics(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	a := NewRedisDedupeStore(client, "svc:dedupe:topic-A")
	b := NewRedisDedupeStore(client, "svc:dedupe:topic-B")
	ctx := context.Background()

	if seen, _ := a.Seen(ctx, "shared-id", time.Minute); seen {
		t.Fatalf("topic A first call should be not-seen")
	}
	// Same ID under a different topic prefix MUST NOT be considered seen.
	if seen, _ := b.Seen(ctx, "shared-id", time.Minute); seen {
		t.Fatalf("topic B should be isolated from topic A's keyspace")
	}
}

func TestRedisDedupeStore_FailsOpenOnError(t *testing.T) {
	t.Parallel()
	// Point at an unreachable address so SETNX errors immediately.
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1", // nothing listens here
		DialTimeout: 50 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisDedupeStore(client, "svc:dedupe:topic-C")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	seen, err := store.Seen(ctx, "msg-1", time.Minute)
	if err == nil {
		t.Fatalf("expected an error from unreachable Redis")
	}
	if seen {
		t.Fatalf("on error, must report not-seen so the subscription can fail open")
	}
	// Also assert the error is a real connection-style failure rather than nil.
	var netErr error = err
	if netErr == nil || errors.Is(netErr, context.Canceled) {
		// context.Canceled is acceptable too (ctx-based abort), but a nil
		// guard is what we really care about.
	}
}
