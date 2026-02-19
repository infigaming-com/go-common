package snowflake

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// NodeLease manages a leased node ID in Redis for snowflake ID generation.
type NodeLease struct {
	client    redis.Scripter
	nodeID    int64
	holder    string
	leaseKey  string
	keyPrefix string
	ttl       time.Duration
	healthy   atomic.Bool
	metrics   MetricsHook
	stopCh    chan struct{}
	doneCh    chan struct{}
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
	return nl.nodeID
}

// IsHealthy returns true if the lease is still considered valid.
func (nl *NodeLease) IsHealthy() bool {
	return nl.healthy.Load()
}

// Release gracefully releases the lease and stops the heartbeat.
func (nl *NodeLease) Release(ctx context.Context) error {
	// Signal heartbeat to stop
	close(nl.stopCh)
	// Wait for heartbeat goroutine to exit
	<-nl.doneCh

	result, err := redis.NewScript(releaseLeaseLua).Run(ctx, nl.client,
		[]string{nl.leaseKey},
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
			ctx, cancel := context.WithTimeout(context.Background(), interval/2)
			result, err := redis.NewScript(renewLeaseLua).Run(ctx, nl.client,
				[]string{nl.leaseKey},
				nl.holder, int(nl.ttl.Seconds()),
			).Int64()
			cancel()

			if err != nil || result == 0 {
				consecutiveFailures++
				nl.metrics.OnLeaseRenewFail()
				if consecutiveFailures >= maxConsecutiveFailures {
					nl.healthy.Store(false)
					nl.metrics.OnLeaseExpired()
				}
			} else {
				consecutiveFailures = 0
				nl.healthy.Store(true)
				nl.metrics.OnLeaseRenewed()
			}
		}
	}
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
