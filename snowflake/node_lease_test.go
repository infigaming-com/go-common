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

func TestNodeLease_SelfHeal_ReclaimAfterExpiry(t *testing.T) {
	mr, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(3*time.Second),
	)
	require.NoError(t, err)
	defer func() { _ = nl.Release(context.Background()) }()

	originalNodeID := nl.NodeID()

	// Simulate Redis outage: delete the lease key to mimic TTL expiry
	mr.Del("snowflake:node:" + strconv.FormatInt(originalNodeID, 10))

	// Wait for heartbeat to fire and self-heal (TTL/3 = 1s)
	time.Sleep(1500 * time.Millisecond)

	// Lease should have self-healed by reclaiming the same node ID
	assert.True(t, nl.IsHealthy(), "lease should be healthy after self-healing")
	assert.Equal(t, originalNodeID, nl.NodeID(), "should reclaim the same node ID")
}

func TestNodeLease_SelfHeal_ClaimNewNode(t *testing.T) {
	mr, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(3*time.Second),
	)
	require.NoError(t, err)
	defer func() { _ = nl.Release(context.Background()) }()

	originalNodeID := nl.NodeID()

	// Simulate: key expired AND another holder took our node
	leaseKey := "snowflake:node:" + strconv.FormatInt(originalNodeID, 10)
	mr.Set(leaseKey, "other-holder")
	mr.SetTTL(leaseKey, 30*time.Second)

	// Track node ID changes
	var newNodeIDFromCallback int64
	nl.setNodeIDUpdater(func(id int64) {
		newNodeIDFromCallback = id
	})

	// Wait for heartbeat to detect and self-heal
	time.Sleep(1500 * time.Millisecond)

	assert.True(t, nl.IsHealthy(), "lease should be healthy after claiming new node")
	assert.NotEqual(t, originalNodeID, nl.NodeID(), "should have a different node ID")
	assert.Equal(t, nl.NodeID(), newNodeIDFromCallback, "callback should be invoked with new node ID")
}

func TestNodeLease_SelfHeal_GeneratorNodeIDUpdate(t *testing.T) {
	mr, client := setupMiniredis(t)

	nl, err := AcquireNodeLease(context.Background(), client,
		WithServiceName("test-svc"),
		WithLeaseTTL(3*time.Second),
	)
	require.NoError(t, err)
	defer func() { _ = nl.Release(context.Background()) }()

	gen, err := NewGenerator(nl.NodeID(), WithLeaseHealthCheck(nl))
	require.NoError(t, err)

	originalNodeID := nl.NodeID()

	// Generate an ID before self-healing
	id1, err := gen.NextID()
	require.NoError(t, err)
	_, nodeFromID1, _ := DecomposeID(id1)
	assert.Equal(t, originalNodeID, nodeFromID1)

	// Simulate: another holder stole our node
	leaseKey := "snowflake:node:" + strconv.FormatInt(originalNodeID, 10)
	mr.Set(leaseKey, "other-holder")
	mr.SetTTL(leaseKey, 30*time.Second)

	// Wait for self-healing
	time.Sleep(1500 * time.Millisecond)

	assert.True(t, nl.IsHealthy())

	// Generator should use the new node ID
	id2, err := gen.NextID()
	require.NoError(t, err)
	_, nodeFromID2, _ := DecomposeID(id2)
	assert.Equal(t, nl.NodeID(), nodeFromID2, "generator should use the new node ID")
	assert.NotEqual(t, originalNodeID, nodeFromID2, "new ID should use different node")
}
