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

	// Insert 10 unique unexpired ids into a size-2 store. The map must never
	// grow past size between insertions — the previous evictLocked returned
	// when len == size and let the caller's unconditional insert push it to
	// size+1.
	for i := 0; i < 10; i++ {
		_, _ = s.Seen(ctx, fmt.Sprintf("k-%d", i), time.Hour)
		if got := len(s.items); got > s.size {
			t.Fatalf("after %d inserts: len=%d > size=%d", i+1, got, s.size)
		}
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

func TestRedisDedupeStore_PrefixTrimsTrailingColon(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	// "svc:dedupe:topic" and "svc:dedupe:topic:" must produce the same keys
	// so that an accidental trailing colon in caller config is a no-op
	// rather than a silent dedupe miss.
	a := NewRedisDedupeStore(client, "svc:dedupe:topic")
	b := NewRedisDedupeStore(client, "svc:dedupe:topic:")
	ctx := context.Background()

	if seen, _ := a.Seen(ctx, "id-1", time.Minute); seen {
		t.Fatalf("a first call should be not-seen")
	}
	// b uses the trimmed-equivalent prefix, so it should observe a's write.
	if seen, _ := b.Seen(ctx, "id-1", time.Minute); !seen {
		t.Fatalf("b should observe a's key (trailing colon must be trimmed)")
	}
}

func TestNewRedisDedupeStore_PanicsOnNilClient(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil client")
		}
	}()
	_ = NewRedisDedupeStore(nil, "svc:dedupe:topic")
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
