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
// Lifecycle (managed by subscription, see subscription.process):
//
//  1. Seen(id, occupyTTL)
//     Race-blocking SETNX with a short TTL just long enough to cover the
//     longest possible handler run (ack_deadline + max_extension + slack).
//     A true return means another worker / pod is already processing the
//     same id — the caller should ack and bail.
//
//  2. After the handler returns:
//     - success or terminal failure (dead-letter / max retries):
//     Extend(id, ttl)  — bump TTL to the GCP redelivery-safety window
//     so subsequent Pub/Sub redeliveries are dropped.
//     - retryable failure (handler returned err, message will be Nack'd):
//     Delete(id)       — let the redelivery flow back into the handler
//     instead of being silently dedupe'd.
//
//     Use dedupe ONLY for at-most-once side effects:
//     ✓ Slack / Telegram / push notifications
//     ✓ One-shot external API calls (payment-channel webhooks, etc.)
//
//     Do NOT enable dedupe on handlers that need at-least-once delivery:
//     ✗ DB writes that must commit eventually
//     ✗ Bet settlement, balance updates, anything ledger-like
//
// If a handler is at-least-once but its side effects are also expensive
// to repeat, do not enable dedupe here — instead make the handler itself
// idempotent (e.g. UNIQUE constraint, conditional UPDATE).
type DedupeStore interface {
	// Seen records id under the given TTL window and reports whether the id
	// was already present. A true return means the caller should skip the
	// message.
	Seen(ctx context.Context, id string, ttl time.Duration) (bool, error)
	// Delete removes id from the store. Used by the subscription to undo a
	// pre-handler occupancy when the handler returns a retryable error, so
	// that the eventual Pub/Sub redelivery can flow into a fresh handler
	// invocation. Must be a no-op if the key is absent.
	Delete(ctx context.Context, id string) error
	// Extend resets the TTL on an existing id. Called after a handler
	// completes (successfully or via terminal failure) to widen the window
	// during which subsequent redeliveries are silently dropped. Must be a
	// no-op if the key is absent (treat as "nothing to extend").
	Extend(ctx context.Context, id string, ttl time.Duration) error
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

func (d *inMemoryDedupeStore) Delete(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.items, id)
	return nil
}

func (d *inMemoryDedupeStore) Extend(_ context.Context, id string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	// No-op when the key has already expired or was never set; the caller
	// has already moved on and re-creating the entry here would race with
	// Seen on a future redelivery.
	if _, ok := d.items[id]; !ok {
		return nil
	}
	d.items[id] = time.Now().Add(ttl)
	return nil
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
