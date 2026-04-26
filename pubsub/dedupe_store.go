package pubsub

import (
	"context"
	"sync"
	"time"
)

// DedupeStore tracks message IDs that have already been processed so that a
// redelivery (per-pod restart, ack timeout, cross-pod handoff, etc.) can be
// dropped before the handler runs.
//
// Implementations MUST be safe for concurrent use.
//
// Errors returned from Seen are logged by the subscription and treated as a
// "not seen" answer — fail-open. Dropping a message because the dedupe store
// is unhealthy is strictly worse than reprocessing it.
//
// IMPORTANT — pre-handler semantics:
//
// Seen is called BEFORE the handler runs. The dedupe key is therefore set
// even if the handler later returns an error and the message is Nacked.
// Within the TTL window, every redelivery of that id is silently dropped.
//
//	Use this ONLY for at-most-once side effects:
//	  ✓ Slack / Telegram / push notifications
//	  ✓ One-shot external API calls (payment-channel webhooks, etc.)
//
//	Do NOT enable dedupe on handlers that need at-least-once delivery:
//	  ✗ DB writes that must commit eventually
//	  ✗ Bet settlement, balance updates, anything ledger-like
//
// If a handler is at-least-once but its side effects are also expensive
// to repeat, do not enable dedupe here — instead make the handler itself
// idempotent (e.g. UNIQUE constraint, conditional UPDATE).
type DedupeStore interface {
	// Seen records id under the given TTL window and reports whether the id
	// was already present. A true return means the caller should skip the
	// message.
	Seen(ctx context.Context, id string, ttl time.Duration) (bool, error)
}

// newInMemoryDedupeStore returns the default in-process dedupe implementation.
// It does not survive a process restart and does not coordinate across pods,
// so high-stakes side-effecting handlers (notifications, payments) should
// inject a shared store such as the Redis-backed one.
func newInMemoryDedupeStore(size int) *inMemoryDedupeStore {
	if size <= 0 {
		size = 1024
	}
	return &inMemoryDedupeStore{items: make(map[string]time.Time, size), size: size}
}

type inMemoryDedupeStore struct {
	mu    sync.Mutex
	items map[string]time.Time
	size  int
}

func (d *inMemoryDedupeStore) Seen(_ context.Context, id string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if expiry, ok := d.items[id]; ok && now.Before(expiry) {
		return true, nil
	}
	if len(d.items) >= d.size {
		d.evictLocked(now)
	}
	d.items[id] = now.Add(ttl)
	return false, nil
}

// evictLocked is called with d.mu held when len(d.items) >= d.size. It first
// drops expired entries; if the cap is still hit (no expirations or all keys
// still live), it drops arbitrary entries until at least one slot is free
// (len < d.size), so the caller can insert the new id without exceeding cap.
func (d *inMemoryDedupeStore) evictLocked(now time.Time) {
	for k, v := range d.items {
		if now.After(v) {
			delete(d.items, k)
		}
	}
	for k := range d.items {
		if len(d.items) < d.size {
			return
		}
		delete(d.items, k)
	}
}
