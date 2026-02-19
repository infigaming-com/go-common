package snowflake

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return mr, client
}

func TestAcquireNodeLease(t *testing.T) {
	_, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(10*time.Second),
	)
	require.NoError(t, err)
	require.NotNil(t, nl)

	assert.GreaterOrEqual(t, nl.NodeID(), int64(0))
	assert.LessOrEqual(t, nl.NodeID(), int64(1023))
	assert.True(t, nl.IsHealthy())

	// Clean up
	err = nl.Release(context.Background())
	assert.NoError(t, err)
}

func TestAcquireNodeLease_MultipleNodes(t *testing.T) {
	_, client := setupMiniredis(t)

	leases := make([]*NodeLease, 0, 5)
	nodeIDs := make(map[int64]struct{})

	for i := 0; i < 5; i++ {
		nl, err := AcquireNodeLease(context.Background(), client,
			WithServiceName("test-svc"),
			WithLeaseTTL(10*time.Second),
		)
		require.NoError(t, err)
		_, dup := nodeIDs[nl.NodeID()]
		assert.False(t, dup, "duplicate node ID: %d", nl.NodeID())
		nodeIDs[nl.NodeID()] = struct{}{}
		leases = append(leases, nl)
	}

	assert.Len(t, nodeIDs, 5)

	// Release all
	for _, nl := range leases {
		err := nl.Release(context.Background())
		assert.NoError(t, err)
	}
}

func TestNodeLease_Release(t *testing.T) {
	_, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(10*time.Second),
	)
	require.NoError(t, err)

	nodeID := nl.NodeID()

	err = nl.Release(context.Background())
	require.NoError(t, err)
	assert.False(t, nl.IsHealthy())

	// Should be able to acquire the same node ID again
	nl2, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc-2"),
		WithLeaseTTL(10*time.Second),
	)
	require.NoError(t, err)
	assert.Equal(t, nodeID, nl2.NodeID())

	err = nl2.Release(context.Background())
	assert.NoError(t, err)
}

func TestNodeLease_TTLExpiry(t *testing.T) {
	mr, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(5*time.Second),
	)
	require.NoError(t, err)

	nodeID := nl.NodeID()

	// Stop heartbeat goroutine
	close(nl.stopCh)
	<-nl.doneCh

	// Fast-forward time to expire the key
	mr.FastForward(6 * time.Second)

	// Key should be expired, another process can claim it
	nl2, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc-2"),
		WithLeaseTTL(5*time.Second),
	)
	require.NoError(t, err)
	assert.Equal(t, nodeID, nl2.NodeID())

	err = nl2.Release(context.Background())
	assert.NoError(t, err)
}

func TestNodeLease_HeartbeatKeepsAlive(t *testing.T) {
	mr, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(3*time.Second),
	)
	require.NoError(t, err)
	defer func() { _ = nl.Release(context.Background()) }()

	// Heartbeat fires at TTL/3 = 1s. Wait for it, then advance within TTL.
	time.Sleep(1500 * time.Millisecond)
	mr.FastForward(2 * time.Second)

	assert.True(t, nl.IsHealthy())
}

func TestNodeLease_AllSlotsOccupied(t *testing.T) {
	mr, client := setupMiniredis(t)

	// Pre-fill all 1024 slots
	for i := 0; i < 1024; i++ {
		mr.Set("snowflake:node:"+strconv.Itoa(i), "other-holder")
		mr.SetTTL("snowflake:node:"+strconv.Itoa(i), 30*time.Second)
	}

	_, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(10*time.Second),
	)
	assert.ErrorIs(t, err, ErrNoAvailableNode)
}

func TestNodeLease_CustomKeyPrefix(t *testing.T) {
	mr, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseKeyPrefix("custom:prefix:"),
		WithLeaseTTL(10*time.Second),
	)
	require.NoError(t, err)

	val := mr.Exists("custom:prefix:0")
	assert.True(t, val)

	err = nl.Release(context.Background())
	assert.NoError(t, err)
}

func TestNodeLease_HolderIdentity(t *testing.T) {
	holder := buildHolder("my-service")
	assert.Contains(t, holder, "my-service:")
}
