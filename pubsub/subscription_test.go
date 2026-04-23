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

// TestStreamWatchdog_PeriodicRefresh verifies the preventive refresh cancels
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

	got := transport.subscribeCalls.Load()
	if got < 4 {
		t.Fatalf("expected >=4 Subscribe calls within 500ms at 80ms refresh, got %d", got)
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
