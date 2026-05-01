package pubsub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockTransport is a controllable Transport for exercising watchdog and
// error-classification paths without a real broker.
type mockTransport struct {
	subscribeCalls atomic.Int32
	lastCtx        atomic.Value // context.Context of the most recent Subscribe call
	// subscribeFn runs on every Subscribe call. Defaults to blocking until ctx is done.
	subscribeFn func(ctx context.Context, handler TransportHandler) error
}

func (m *mockTransport) Publish(_ context.Context, _ string, _ *Envelope) (string, error) {
	return "", errors.New("mock: publish not supported")
}

func (m *mockTransport) Subscribe(ctx context.Context, _ string, _ TransportSubscribeOptions, h TransportHandler) error {
	m.subscribeCalls.Add(1)
	m.lastCtx.Store(ctx)
	if m.subscribeFn != nil {
		return m.subscribeFn(ctx, h)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (m *mockTransport) Close(_ context.Context) error { return nil }

// recordingLogger collects log entries so tests can assert on severity.
type recordingLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

type logEntry struct {
	level string
	msg   string
	kv    []any
}

func (r *recordingLogger) add(level, msg string, kv ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, logEntry{level: level, msg: msg, kv: kv})
}

func (r *recordingLogger) Debug(_ context.Context, msg string, kv ...any) { r.add("debug", msg, kv...) }
func (r *recordingLogger) Info(_ context.Context, msg string, kv ...any)  { r.add("info", msg, kv...) }
func (r *recordingLogger) Warn(_ context.Context, msg string, kv ...any)  { r.add("warn", msg, kv...) }
func (r *recordingLogger) Error(_ context.Context, msg string, kv ...any) { r.add("error", msg, kv...) }

func (r *recordingLogger) has(level, msgSubstring string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.level == level && containsStr(e.msg, msgSubstring) {
			return true
		}
	}
	return false
}

func (r *recordingLogger) countMessage(msgSubstring string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.entries {
		if containsStr(e.msg, msgSubstring) {
			n++
		}
	}
	return n
}

func containsStr(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// spyDedupeStore wraps an inMemoryDedupeStore so tests can assert on the
// Seen/Extend/Delete call sequence the subscription drives. Forwards real
// behavior so race-blocking via SETNX continues to work.
type spyDedupeStore struct {
	inner                         DedupeStore
	mu                            sync.Mutex
	seen, extend, delete          int
	lastSeenTTL, lastExtendTTL    time.Duration
	seenIDs, extendIDs, deleteIDs []string
}

func newSpyDedupe(size int) *spyDedupeStore {
	return &spyDedupeStore{inner: newInMemoryDedupeStore(size)}
}

func (s *spyDedupeStore) Seen(ctx context.Context, id string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	s.seen++
	s.lastSeenTTL = ttl
	s.seenIDs = append(s.seenIDs, id)
	s.mu.Unlock()
	return s.inner.Seen(ctx, id, ttl)
}

func (s *spyDedupeStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	s.delete++
	s.deleteIDs = append(s.deleteIDs, id)
	s.mu.Unlock()
	return s.inner.Delete(ctx, id)
}

func (s *spyDedupeStore) Extend(ctx context.Context, id string, ttl time.Duration) error {
	s.mu.Lock()
	s.extend++
	s.lastExtendTTL = ttl
	s.extendIDs = append(s.extendIDs, id)
	s.mu.Unlock()
	return s.inner.Extend(ctx, id, ttl)
}

func (s *spyDedupeStore) snapshot() (seen, extend, delete int, lastSeenTTL, lastExtendTTL time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen, s.extend, s.delete, s.lastSeenTTL, s.lastExtendTTL
}

// deliverOne installs a subscribeFn that delivers a single TransportMessage
// (with the supplied ID) on the first stream and then blocks. ackCh receives
// "ack" or "nack" based on which terminal the subscription chose, so tests
// can synchronize on completion without scanning logs.
func deliverOne(id string, ackCh chan<- string) func(ctx context.Context, h TransportHandler) error {
	var delivered atomic.Bool
	return func(ctx context.Context, h TransportHandler) error {
		if delivered.CompareAndSwap(false, true) {
			_ = h(ctx, &TransportMessage{
				Envelope:   Envelope{ID: id},
				ReceivedAt: time.Now(),
				Ack:        func() error { ackCh <- "ack"; return nil },
				Nack:       func() error { ackCh <- "nack"; return nil },
				Extend:     func(time.Duration) error { return nil },
				Done:       make(chan struct{}),
			})
		}
		<-ctx.Done()
		return ctx.Err()
	}
}

// TestDedupe_HandlerSuccess_ExtendsTTL verifies the post-handler flow on the
// happy path: Seen with the short occupancy TTL, then Extend to the
// configured long TTL, then Ack. No Delete.
func TestDedupe_HandlerSuccess_ExtendsTTL(t *testing.T) {
	const longTTL = 7200 * time.Second
	ackCh := make(chan string, 4)
	transport := &mockTransport{subscribeFn: deliverOne("msg-success", ackCh)}
	logger := &recordingLogger{}
	spy := newSpyDedupe(8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sub, err := client.Subscribe(
		"topic-success",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
		WithSubscriptionDeduplication(DeduplicationConfig{Enabled: true, TTL: longTTL, Size: 16}),
		WithSubscriptionDedupeStore(spy),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Stop(ctx)

	select {
	case got := <-ackCh:
		if got != "ack" {
			t.Fatalf("expected ack on success, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("handler/ack did not complete in 2s")
	}

	// Drain a second tick to avoid race with Extend running just after Ack.
	time.Sleep(50 * time.Millisecond)

	seen, extend, deleted, seenTTL, extendTTL := spy.snapshot()
	if seen != 1 || extend != 1 || deleted != 0 {
		t.Fatalf("Seen=%d Extend=%d Delete=%d (want 1/1/0)", seen, extend, deleted)
	}
	if seenTTL != dedupeOccupyTTL {
		t.Fatalf("Seen TTL=%v, want %v (occupancy)", seenTTL, dedupeOccupyTTL)
	}
	if extendTTL != longTTL {
		t.Fatalf("Extend TTL=%v, want %v (configured long)", extendTTL, longTTL)
	}
}

// TestDedupe_RetryableFailure_DeletesKey covers the retry path: handler
// returns a non-permanent error, the subscription Delete()s the dedupe key,
// then Nacks. Without Delete the inbound redelivery would be silently
// dedupe'd and the at-least-once handler would never get a second chance.
func TestDedupe_RetryableFailure_DeletesKey(t *testing.T) {
	ackCh := make(chan string, 4)
	transport := &mockTransport{subscribeFn: deliverOne("msg-retry", ackCh)}
	logger := &recordingLogger{}
	spy := newSpyDedupe(8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sub, err := client.Subscribe(
		"topic-retry",
		HandlerFunc(func(_ context.Context, _ *Message) error {
			return errors.New("transient handler failure")
		}),
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
		WithSubscriptionDeduplication(DeduplicationConfig{Enabled: true, TTL: time.Hour, Size: 16}),
		WithSubscriptionDedupeStore(spy),
		// MaxAttempts >= 2 so the first failure goes through onFailure
		// (Nack + Delete) instead of being escalated to permanent.
		WithSubscriptionRetry(RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond, Multiplier: 2}),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Stop(ctx)

	select {
	case got := <-ackCh:
		if got != "nack" {
			t.Fatalf("expected nack on retryable failure, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("handler/nack did not complete in 2s")
	}
	time.Sleep(50 * time.Millisecond)

	seen, extend, deleted, _, _ := spy.snapshot()
	if seen != 1 || extend != 0 || deleted != 1 {
		t.Fatalf("Seen=%d Extend=%d Delete=%d (want 1/0/1)", seen, extend, deleted)
	}
}

// TestDedupe_RaceBlocked_SecondInvocationSkipsHandler verifies the
// pre-handler SETNX still blocks a concurrent second processing of the same
// id (e.g. cross-pod redelivery). The first occupancy must be visible to
// the second Seen() call before the handler completes — we use a slow
// handler to widen that window.
func TestDedupe_RaceBlocked_SecondInvocationSkipsHandler(t *testing.T) {
	spy := newSpyDedupe(8)
	handlerStart := make(chan struct{})
	handlerRelease := make(chan struct{})
	var handlerCalls atomic.Int32

	// Two transports sharing the same dedupe spy — like two pods each
	// receiving the same redelivered message.
	makeTransport := func(id string, ackCh chan<- string) *mockTransport {
		return &mockTransport{subscribeFn: deliverOne(id, ackCh)}
	}
	ackA := make(chan string, 2)
	ackB := make(chan string, 2)
	transportA := makeTransport("dup-id", ackA)
	transportB := makeTransport("dup-id", ackB)
	loggerA := &recordingLogger{}
	loggerB := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clientA, _ := New(ctx, transportA, WithLogger(loggerA))
	clientB, _ := New(ctx, transportB, WithLogger(loggerB))

	slowHandler := HandlerFunc(func(c context.Context, _ *Message) error {
		handlerCalls.Add(1)
		close(handlerStart)
		select {
		case <-handlerRelease:
		case <-c.Done():
		}
		return nil
	})
	fastHandler := HandlerFunc(func(_ context.Context, _ *Message) error {
		handlerCalls.Add(1)
		return nil
	})

	subA, _ := clientA.Subscribe(
		"topic-race",
		slowHandler,
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
		WithSubscriptionDeduplication(DeduplicationConfig{Enabled: true, TTL: time.Hour, Size: 16}),
		WithSubscriptionDedupeStore(spy),
	)
	defer subA.Stop(ctx)

	// Wait for slowHandler to start so the dedupe key is occupied.
	select {
	case <-handlerStart:
	case <-time.After(2 * time.Second):
		t.Fatalf("slow handler never started")
	}

	subB, _ := clientB.Subscribe(
		"topic-race",
		fastHandler,
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
		WithSubscriptionDeduplication(DeduplicationConfig{Enabled: true, TTL: time.Hour, Size: 16}),
		WithSubscriptionDedupeStore(spy),
	)
	defer subB.Stop(ctx)

	// B should ack immediately without invoking its handler.
	select {
	case got := <-ackB:
		if got != "ack" {
			t.Fatalf("B should ack on dedupe drop, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("B did not ack within 2s — dedupe failed to block the race")
	}
	if got := handlerCalls.Load(); got != 1 {
		t.Fatalf("handler called %d times, want 1 (slow handler only — B must have skipped)", got)
	}

	close(handlerRelease)
	<-ackA
}

// the current Subscribe call on the configured cadence so the broker stream is
// torn down and re-established.
func TestStreamWatchdog_PeriodicRefresh(t *testing.T) {
	transport := &mockTransport{}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sub, err := client.Subscribe(
		"topic-a",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		WithSubscriptionStreamRefreshInterval(80*time.Millisecond),
		WithSubscriptionInactivityTimeout(-1), // disable the inactivity branch for this test
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 500ms / 80ms = 6.25 expected refreshes. Accept >=3 to absorb slow-CI
	// jitter without masking a real regression (the unpatched receiver would
	// produce exactly 1 call).
	got := transport.subscribeCalls.Load()
	if got < 3 {
		t.Fatalf("expected >=3 Subscribe calls within 500ms at 80ms refresh, got %d", got)
	}

	if !logger.has("info", "stream refreshed") {
		t.Fatalf("expected a 'stream refreshed' info log entry; got %+v", logger.entries)
	}

	if err := sub.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestStreamWatchdog_InactivityTriggersReconnect verifies that when no
// messages have flowed for longer than inactivityTimeout the watchdog cancels
// the stream even though the underlying Receive call is happy to stay blocked.
func TestStreamWatchdog_InactivityTriggersReconnect(t *testing.T) {
	transport := &mockTransport{}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sub, err := client.Subscribe(
		"topic-b",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		// Disable periodic refresh so only inactivity can trigger a reconnect.
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(150*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Wait long enough for the 30s inactivity polling to notice. The test
	// exploits the watchdog's use of inactivityCheckInterval which is a
	// package-level constant; we wait just past 2x inactivity window plus one
	// poll tick.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if transport.subscribeCalls.Load() >= 2 && logger.has("error", "stream inactive") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if got := transport.subscribeCalls.Load(); got < 2 {
		t.Fatalf("expected watchdog-triggered reconnect (>=2 Subscribe calls), got %d", got)
	}
	if !logger.has("error", "stream inactive") {
		t.Fatalf("expected an 'inactive, forcing reconnect' error log entry; entries=%+v", logger.entries)
	}

	if err := sub.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestStreamWatchdog_DoesNotKillFreshlyReconnectedStream is a regression test
// for the watchdog reconnect loop: LastActivity is subscription-scoped, so
// after an inactivity-triggered kill the next stream's first poll tick used
// to read a stale timestamp from the previous stream and trip immediately.
// With the streamStart floor the new stream gets its own grace window.
//
// Critical setup: the first stream MUST deliver a message so LastActivity is
// non-zero. Without that the buggy code path (`!last.IsZero()`) is never
// hit and the test cannot distinguish fixed vs. broken behavior.
func TestStreamWatchdog_DoesNotKillFreshlyReconnectedStream(t *testing.T) {
	var firstStream atomic.Bool
	transport := &mockTransport{
		subscribeFn: func(ctx context.Context, h TransportHandler) error {
			if firstStream.CompareAndSwap(false, true) {
				// Deliver one message immediately so LastActivity is set
				// to ~now. Any later kill+reconnect after the inactivity
				// window thus sees a stale LastActivity that exceeds the
				// window — which used to mean instant re-kill.
				_ = h(ctx, &TransportMessage{
					Envelope:   Envelope{ID: "msg-1", Data: []byte("hello")},
					ReceivedAt: time.Now(),
					Ack:        func() error { return nil },
					Nack:       func() error { return nil },
					Extend:     func(time.Duration) error { return nil },
					Done:       make(chan struct{}),
				})
			}
			<-ctx.Done()
			return ctx.Err()
		},
	}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const inactivity = 150 * time.Millisecond

	sub, err := client.Subscribe(
		"topic-reconnect-loop",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(inactivity),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Stop(ctx)

	// Wait for the watchdog's first kill (the message arrives ~immediately;
	// the stream then idles past the inactivity window and gets killed).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if transport.subscribeCalls.Load() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := transport.subscribeCalls.Load(); got < 2 {
		t.Fatalf("expected first reconnect after %v, got %d Subscribe calls", 2*inactivity, got)
	}

	// Critical assertion: within one inactivity window after the reconnect,
	// the new stream must survive — the previous bug would re-trip on the
	// very next poll tick (~inactivity/2 later) producing a 3rd Subscribe
	// call in well under one window. Sleep one full window minus slack and
	// verify Subscribe count is still 2.
	reconnectAt := time.Now()
	callsAtReconnect := transport.subscribeCalls.Load()
	time.Sleep(inactivity - 20*time.Millisecond)
	if got := transport.subscribeCalls.Load(); got > callsAtReconnect {
		t.Fatalf("watchdog killed freshly reconnected stream within its own grace window: %d Subscribe calls within %v of reconnect (want %d)",
			got, time.Since(reconnectAt), callsAtReconnect)
	}
}

// TestReceiveError_PermanentLogsAsError asserts NotFound / PermissionDenied are
// surfaced as ERROR logs so they are not buried in WARN-level reconnect noise.
func TestReceiveError_PermanentLogsAsError(t *testing.T) {
	transport := &mockTransport{
		subscribeFn: func(_ context.Context, _ TransportHandler) error {
			return status.Error(codes.NotFound, "subscription does not exist")
		},
	}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sub, err := client.Subscribe(
		"ghost-topic",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
		WithSubscriptionRetry(RetryPolicy{
			MaxAttempts: 1, InitialBackoff: time.Second, MaxBackoff: time.Second, Multiplier: 2,
		}),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	// Give the receiver enough time to emit at least one classified log entry.
	time.Sleep(150 * time.Millisecond)

	if !logger.has("error", "misconfigured") {
		t.Fatalf("expected permanent error to be logged at ERROR level; entries=%+v", logger.entries)
	}
	if logger.has("warn", "subscription reconnect") {
		t.Fatalf("permanent error should not log as WARN reconnect; entries=%+v", logger.entries)
	}

	if err := sub.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestReceive_NilErrorExits verifies that a transport returning nil (clean
// close, e.g. inmem shutdown) exits the receiver instead of spinning in an
// error loop.
func TestReceive_NilErrorExits(t *testing.T) {
	calls := make(chan struct{}, 8)
	transport := &mockTransport{
		subscribeFn: func(_ context.Context, _ TransportHandler) error {
			calls <- struct{}{}
			return nil // clean close
		},
	}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = client.Subscribe(
		"clean-close",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Wait for the first Subscribe call, then give the receiver a chance to
	// loop (which it shouldn't on a nil return).
	<-calls
	time.Sleep(100 * time.Millisecond)

	if got := transport.subscribeCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 Subscribe call after nil return, got %d", got)
	}
	if logger.has("warn", "subscription reconnect") {
		t.Fatalf("nil error must not log as reconnect; entries=%+v", logger.entries)
	}
}

// TestStreamWatchdog_NilReturnAfterRefreshReconnects guards against the
// silent-death regression caused by a watchdog refresh on a transport that
// returns nil-err on ctx cancel (e.g. the Google Cloud Pub/Sub transport,
// whose docs state "Receive returns nil when the receiver is shut down;
// that is, when the parent ctx is cancelled or the maximum stream time is
// reached"). Without the fix, the receiver took the "err == nil → return"
// fast path and the subscription went dead the first time the watchdog
// fired — exactly the failure mode the watchdog was introduced to prevent.
func TestStreamWatchdog_NilReturnAfterRefreshReconnects(t *testing.T) {
	transport := &mockTransport{
		subscribeFn: func(ctx context.Context, _ TransportHandler) error {
			// Block until the watchdog cancels receiveCtx, then return nil
			// like the GCP transport would.
			<-ctx.Done()
			return nil
		},
	}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sub, err := client.Subscribe(
		"nil-after-refresh",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		// Force a refresh as fast as the watchdog allows.
		WithSubscriptionStreamRefreshInterval(80*time.Millisecond),
		WithSubscriptionInactivityTimeout(-1),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Stop(ctx)

	// Without the fix, subscribeCalls stays at 1 forever after the first
	// refresh because the receiver exits silently. With the fix, the
	// receiver consults the reason channel, sees the periodic-refresh
	// reason, logs Info, and reopens the stream.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if transport.subscribeCalls.Load() >= 3 && logger.countMessage("stream refreshed") >= 2 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("watchdog refresh on nil-returning transport must reconnect: calls=%d, refreshed_logs=%d, entries=%+v",
		transport.subscribeCalls.Load(),
		logger.countMessage("stream refreshed"),
		logger.entries)
}

// TestReceive_SpuriousCanceledReconnects guards against the silent-death
// failure mode: if a transport returns context.Canceled while the parent
// context is still alive (e.g. transport bug), the receiver must log loudly
// and reconnect rather than exit.
func TestReceive_SpuriousCanceledReconnects(t *testing.T) {
	var calls atomic.Int32
	transport := &mockTransport{
		subscribeFn: func(ctx context.Context, _ TransportHandler) error {
			n := calls.Add(1)
			if n == 1 {
				// Return context.Canceled without the parent ctx being done.
				return context.Canceled
			}
			// Subsequent calls block until this receiveCtx is cancelled
			// (i.e. until Stop() tears us down).
			<-ctx.Done()
			return ctx.Err()
		},
	}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sub, err := client.Subscribe(
		"spurious-cancel",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Stop(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= 2 && logger.has("error", "cancelled without watchdog reason") {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("expected reconnect + ERROR log on spurious Canceled; calls=%d entries=%+v",
		calls.Load(), logger.entries)
}

// TestReceiveError_TransientLogsAsWarn asserts codes like Unavailable remain at
// WARN level so pagers do not fire on brief network blips.
func TestReceiveError_TransientLogsAsWarn(t *testing.T) {
	transport := &mockTransport{
		subscribeFn: func(_ context.Context, _ TransportHandler) error {
			return status.Error(codes.Unavailable, "temporary")
		},
	}
	logger := &recordingLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := New(ctx, transport, WithLogger(logger))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sub, err := client.Subscribe(
		"blippy-topic",
		HandlerFunc(func(_ context.Context, _ *Message) error { return nil }),
		WithSubscriptionStreamRefreshInterval(-1),
		WithSubscriptionInactivityTimeout(-1),
	)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	if !logger.has("warn", "subscription reconnect") {
		t.Fatalf("expected transient error to log at WARN; entries=%+v", logger.entries)
	}
	if logger.has("error", "misconfigured") {
		t.Fatalf("transient error must not log as ERROR misconfigured; entries=%+v", logger.entries)
	}

	if err := sub.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
