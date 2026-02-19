package snowflake

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  int64
		wantErr bool
	}{
		{"valid min", 0, false},
		{"valid max", 1023, false},
		{"valid mid", 512, false},
		{"invalid negative", -1, true},
		{"invalid too large", 1024, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGenerator(tt.nodeID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidNodeID)
				assert.Nil(t, g)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, g)
				assert.Equal(t, tt.nodeID, g.NodeID())
			}
		})
	}
}

func TestNextID_Uniqueness(t *testing.T) {
	g, err := NewGenerator(1)
	require.NoError(t, err)

	const count = 100_000
	seen := make(map[int64]struct{}, count)
	for i := 0; i < count; i++ {
		id, err := g.NextID()
		require.NoError(t, err)
		assert.Positive(t, id)
		_, dup := seen[id]
		assert.False(t, dup, "duplicate ID at index %d: %d", i, id)
		seen[id] = struct{}{}
	}
}

func TestNextID_Monotonic(t *testing.T) {
	g, err := NewGenerator(1)
	require.NoError(t, err)

	var prev int64
	for i := 0; i < 10_000; i++ {
		id, err := g.NextID()
		require.NoError(t, err)
		assert.Greater(t, id, prev, "ID not monotonically increasing at index %d", i)
		prev = id
	}
}

func TestNextID_ConcurrentSafety(t *testing.T) {
	g, err := NewGenerator(1)
	require.NoError(t, err)

	const goroutines = 100
	const idsPerGoroutine = 1000

	var mu sync.Mutex
	seen := make(map[int64]struct{}, goroutines*idsPerGoroutine)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := g.NextID()
				require.NoError(t, err)
				mu.Lock()
				_, dup := seen[id]
				assert.False(t, dup, "concurrent duplicate ID: %d", id)
				seen[id] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Len(t, seen, goroutines*idsPerGoroutine)
}

func TestDecomposeID(t *testing.T) {
	g, err := NewGenerator(42)
	require.NoError(t, err)

	before := time.Now()
	id, err := g.NextID()
	require.NoError(t, err)
	after := time.Now()

	ts, nodeID, seq := DecomposeID(id)

	assert.Equal(t, int64(42), nodeID)
	assert.Equal(t, int64(0), seq)
	assert.True(t, !ts.Before(before.Truncate(time.Millisecond)),
		"timestamp %v should be >= %v", ts, before.Truncate(time.Millisecond))
	assert.True(t, !ts.After(after.Add(time.Millisecond)),
		"timestamp %v should be <= %v", ts, after.Add(time.Millisecond))
}

func TestNextID_SequenceOverflow(t *testing.T) {
	// Use a fixed time to force sequence overflow
	fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	advanced := false

	g, err := NewGenerator(1, WithNowFunc(func() time.Time {
		if advanced {
			return fixedTime.Add(time.Millisecond)
		}
		return fixedTime
	}))
	require.NoError(t, err)

	// Generate maxSequence+1 IDs to trigger overflow
	for i := 0; i <= maxSequence; i++ {
		id, err := g.NextID()
		require.NoError(t, err)
		assert.Positive(t, id)
	}

	// Next call should trigger sequence overflow, advance time
	advanced = true
	id, err := g.NextID()
	require.NoError(t, err)
	assert.Positive(t, id)
}

func TestNextID_ClockRollback_Small(t *testing.T) {
	currentTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	mu := sync.Mutex{}

	g, err := NewGenerator(1, WithMaxClockDrift(200*time.Millisecond), WithNowFunc(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return currentTime
	}))
	require.NoError(t, err)

	// Generate first ID
	_, err = g.NextID()
	require.NoError(t, err)

	// Roll back clock by 50ms (within drift tolerance)
	mu.Lock()
	currentTime = currentTime.Add(-50 * time.Millisecond)
	mu.Unlock()

	// Should eventually succeed (after sleeping)
	// Note: we advance time in the now func after a short delay to simulate clock correction
	go func() {
		time.Sleep(30 * time.Millisecond)
		mu.Lock()
		currentTime = currentTime.Add(100 * time.Millisecond)
		mu.Unlock()
	}()

	id, err := g.NextID()
	require.NoError(t, err)
	assert.Positive(t, id)
}

func TestNextID_ClockRollback_Large(t *testing.T) {
	currentTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	g, err := NewGenerator(1, WithMaxClockDrift(50*time.Millisecond), WithNowFunc(func() time.Time {
		return currentTime
	}))
	require.NoError(t, err)

	// Generate first ID
	_, err = g.NextID()
	require.NoError(t, err)

	// Roll back clock by 200ms (exceeds drift tolerance)
	currentTime = currentTime.Add(-200 * time.Millisecond)

	_, err = g.NextID()
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrClockRollback)
}

func TestBatchNextID(t *testing.T) {
	g, err := NewGenerator(5)
	require.NoError(t, err)

	t.Run("normal batch", func(t *testing.T) {
		ids, err := g.BatchNextID(100)
		require.NoError(t, err)
		assert.Len(t, ids, 100)

		// Verify uniqueness
		seen := make(map[int64]struct{}, len(ids))
		for _, id := range ids {
			_, dup := seen[id]
			assert.False(t, dup)
			seen[id] = struct{}{}
		}
	})

	t.Run("zero count", func(t *testing.T) {
		ids, err := g.BatchNextID(0)
		assert.NoError(t, err)
		assert.Nil(t, ids)
	})

	t.Run("negative count", func(t *testing.T) {
		ids, err := g.BatchNextID(-1)
		assert.NoError(t, err)
		assert.Nil(t, ids)
	})
}

func TestNextID_LeaseExpired(t *testing.T) {
	nl := &NodeLease{}
	nl.healthy.Store(false) // simulate expired lease

	g, err := NewGenerator(1, WithLeaseHealthCheck(nl))
	require.NoError(t, err)

	_, err = g.NextID()
	assert.ErrorIs(t, err, ErrLeaseExpired)
}

func TestDifferentNodes_UniqueIDs(t *testing.T) {
	g1, err := NewGenerator(1)
	require.NoError(t, err)
	g2, err := NewGenerator(2)
	require.NoError(t, err)

	seen := make(map[int64]struct{})
	for i := 0; i < 10_000; i++ {
		id1, err := g1.NextID()
		require.NoError(t, err)
		id2, err := g2.NextID()
		require.NoError(t, err)

		_, dup1 := seen[id1]
		assert.False(t, dup1, "duplicate from g1: %d", id1)
		_, dup2 := seen[id2]
		assert.False(t, dup2, "duplicate from g2: %d", id2)

		seen[id1] = struct{}{}
		seen[id2] = struct{}{}
	}
	assert.Len(t, seen, 20_000)
}

func BenchmarkNextID(b *testing.B) {
	g, _ := NewGenerator(1)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = g.NextID()
		}
	})
}

func BenchmarkNextID_SingleThread(b *testing.B) {
	g, _ := NewGenerator(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.NextID()
	}
}
