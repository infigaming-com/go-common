package snowflake

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// NodeLease manages a leased node ID in Redis for snowflake ID generation.
type NodeLease struct {
	client    redis.Scripter
	holder    string
	keyPrefix string
	ttl       time.Duration
	healthy   atomic.Bool
	metrics   MetricsHook
	stopCh    chan struct{}
	doneCh    chan struct{}

	// mu protects nodeID, leaseKey, and nodeIDUpdater which may be
	// modified by the heartbeat goroutine during self-healing.
	mu            sync.RWMutex
	nodeID        int64
	leaseKey      string
	nodeIDUpdater func(int64)
}

// AcquireNodeLease claims an available node ID (0-1023) from Redis.
// It starts a background heartbeat goroutine to keep the lease alive.
func AcquireNodeLease(ctx context.Context, client redis.Scripter, opts ...LeaseOption) (*NodeLease, error) {
	o := defaultLeaseOptions()
	for _, opt := range opts {
		opt(o)
	}

	holder := buildHolder(o.serviceName)

	// Atomically claim first available node
	result, err := redis.NewScript(claimNodeLua).Run(ctx, client,
		nil, // no KEYS
		o.keyPrefix, holder, int(o.ttl.Seconds()),
	).Int64()
	if err != nil {
		return nil, fmt.Errorf("snowflake: claim node lease: %w", err)
	}
	if result < 0 {
		return nil, ErrNoAvailableNode
	}

	nl := &NodeLease{
		client:    client,
		nodeID:    result,
		holder:    holder,
		leaseKey:  o.keyPrefix + strconv.FormatInt(result, 10),
		keyPrefix: o.keyPrefix,
		ttl:       o.ttl,
		metrics:   o.metrics,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	nl.healthy.Store(true)
	nl.metrics.OnLeaseAcquired(result)

	// Start background heartbeat
	go nl.heartbeatLoop()

	return nl, nil
}

// NodeID returns the leased node ID.
func (nl *NodeLease) NodeID() int64 {
	nl.mu.RLock()
	defer nl.mu.RUnlock()
	return nl.nodeID
}

// IsHealthy returns true if the lease is still considered valid.
func (nl *NodeLease) IsHealthy() bool {
	return nl.healthy.Load()
}

// setNodeIDUpdater registers a callback invoked when the node ID changes
// during self-healing (e.g., when the original node was taken by another
// holder and a new node had to be claimed).
func (nl *NodeLease) setNodeIDUpdater(fn func(int64)) {
	nl.mu.Lock()
	nl.nodeIDUpdater = fn
	nl.mu.Unlock()
}

// Release gracefully releases the lease and stops the heartbeat.
func (nl *NodeLease) Release(ctx context.Context) error {
	// Signal heartbeat to stop
	close(nl.stopCh)
	// Wait for heartbeat goroutine to exit
	<-nl.doneCh

	nl.mu.RLock()
	leaseKey := nl.leaseKey
	nl.mu.RUnlock()

	result, err := redis.NewScript(releaseLeaseLua).Run(ctx, nl.client,
		[]string{leaseKey},
		nl.holder,
	).Int64()
	if err != nil {
		return fmt.Errorf("snowflake: release lease: %w", err)
	}
	if result == 0 {
		return ErrLeaseNotHeld
	}

	nl.healthy.Store(false)
	nl.metrics.OnLeaseReleased()
	return nil
}

// heartbeatLoop renews the lease at TTL/3 intervals.
// If the lease key expires in Redis (e.g., after a transient outage),
// it automatically reclaims the same node ID or acquires a new one.
func (nl *NodeLease) heartbeatLoop() {
	defer close(nl.doneCh)

	interval := nl.ttl / 3
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutiveFailures := 0
	const maxConsecutiveFailures = 3

	for {
		select {
		case <-nl.stopCh:
			return
		case <-ticker.C:
			ok := nl.tryRenewOrReclaim(interval)
			if ok {
				consecutiveFailures = 0
				nl.healthy.Store(true)
			} else {
				consecutiveFailures++
				nl.metrics.OnLeaseRenewFail()
				if consecutiveFailures >= maxConsecutiveFailures {
					nl.healthy.Store(false)
					nl.metrics.OnLeaseExpired()
				}
			}
		}
	}
}

// tryRenewOrReclaim attempts to keep the lease alive. It tries, in order:
//  1. Renew the existing key (holder matches) or reclaim it (key expired, SET NX)
//  2. If another holder owns our node, claim any available node and update state
//
// Returns true if the lease is healthy after this attempt.
func (nl *NodeLease) tryRenewOrReclaim(timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout/2)
	defer cancel()

	nl.mu.RLock()
	leaseKey := nl.leaseKey
	nl.mu.RUnlock()

	result, err := redis.NewScript(renewOrReclaimLua).Run(ctx, nl.client,
		[]string{leaseKey},
		nl.holder, int(nl.ttl.Seconds()),
	).Int64()
	if err != nil {
		return false
	}

	switch result {
	case 1:
		// Renewed successfully (holder matched)
		nl.metrics.OnLeaseRenewed()
		return true
	case 2:
		// Reclaimed same node ID (key had expired)
		nl.metrics.OnLeaseReclaimed(nl.NodeID())
		return true
	default:
		// result == 0: different holder owns our node.
		// Try to claim any available node.
		return nl.tryClaimNewNode(ctx)
	}
}

// tryClaimNewNode claims a new node ID when the original one was taken.
// It updates the lease state and notifies the Generator of the change.
func (nl *NodeLease) tryClaimNewNode(ctx context.Context) bool {
	newNodeID, err := redis.NewScript(claimNodeLua).Run(ctx, nl.client,
		nil,
		nl.keyPrefix, nl.holder, int(nl.ttl.Seconds()),
	).Int64()
	if err != nil || newNodeID < 0 {
		return false
	}

	nl.mu.Lock()
	oldNodeID := nl.nodeID
	nl.nodeID = newNodeID
	nl.leaseKey = nl.keyPrefix + strconv.FormatInt(newNodeID, 10)
	updater := nl.nodeIDUpdater
	nl.mu.Unlock()

	// Notify Generator to update its node ID
	if updater != nil && newNodeID != oldNodeID {
		updater(newNodeID)
	}

	nl.metrics.OnLeaseReclaimed(newNodeID)
	return true
}

// buildHolder constructs a holder identity: "{service}:{hostname}:{pid}".
func buildHolder(serviceName string) string {
	hostname, _ := os.Hostname()
	// POD_NAME env var is available in K8s pods
	if podName := os.Getenv("POD_NAME"); podName != "" {
		hostname = podName
	}
	return fmt.Sprintf("%s:%s:%d", serviceName, hostname, os.Getpid())
}
