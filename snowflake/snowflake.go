package snowflake

import (
	"fmt"
	"sync"
	"time"
)

const (
	// Bit allocation: 1 sign + 41 timestamp + 10 node + 12 sequence = 64
	timestampBits = 41
	nodeBits      = 10
	sequenceBits  = 12

	maxNodeID   = (1 << nodeBits) - 1     // 1023
	maxSequence = (1 << sequenceBits) - 1 // 4095

	nodeShift      = sequenceBits            // 12
	timestampShift = sequenceBits + nodeBits // 22

	// Custom epoch: 2023-01-01 00:00:00 UTC (matches existing Sonyflake epoch)
	customEpochMs = 1672531200000
)

// Generator produces globally unique int64 snowflake IDs.
type Generator struct {
	mu            sync.Mutex
	epoch         int64 // custom epoch in ms
	nodeID        int64
	lastTime      int64 // last timestamp ms since epoch
	sequence      int64
	maxClockDrift time.Duration
	leaseCheck    *NodeLease
	metrics       MetricsHook
	now           func() time.Time
}

// NewGenerator creates a snowflake ID generator for the given node ID (0-1023).
func NewGenerator(nodeID int64, opts ...Option) (*Generator, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidNodeID, nodeID)
	}

	o := defaultGeneratorOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Generator{
		epoch:         customEpochMs,
		nodeID:        nodeID,
		maxClockDrift: o.maxClockDrift,
		leaseCheck:    o.leaseCheck,
		metrics:       o.metrics,
		now:           o.now,
	}, nil
}

// NextID generates a single unique int64 ID.
func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.leaseCheck != nil && !g.leaseCheck.IsHealthy() {
		return 0, ErrLeaseExpired
	}

	now := g.currentTimeMs()

	if now < g.lastTime {
		drift := time.Duration(g.lastTime-now) * time.Millisecond
		if drift > g.maxClockDrift {
			g.metrics.OnClockRollback()
			return 0, fmt.Errorf("%w: drift %v", ErrClockRollback, drift)
		}
		// Small drift: sleep and retry
		g.metrics.OnClockRollback()
		g.mu.Unlock()
		time.Sleep(drift)
		g.mu.Lock()
		now = g.currentTimeMs()
		if now < g.lastTime {
			return 0, fmt.Errorf("%w: drift persists after sleep", ErrClockRollback)
		}
	}

	if now == g.lastTime {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			// Sequence overflow: spin-wait for next millisecond
			g.metrics.OnSequenceOverflow()
			now = g.waitNextMs(now)
		}
	} else {
		g.sequence = 0
	}

	g.lastTime = now

	id := (now << timestampShift) | (g.nodeID << nodeShift) | g.sequence
	g.metrics.OnIDGenerated(1)
	return id, nil
}

// BatchNextID generates multiple unique int64 IDs.
func (g *Generator) BatchNextID(count int) ([]int64, error) {
	if count <= 0 {
		return nil, nil
	}

	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		id, err := g.NextID()
		if err != nil {
			return ids, fmt.Errorf("batch generate at index %d: %w", i, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// NodeID returns the node ID of this generator.
func (g *Generator) NodeID() int64 {
	return g.nodeID
}

// DecomposeID extracts the timestamp, node ID, and sequence from an ID.
func DecomposeID(id int64) (timestamp time.Time, nodeID int64, sequence int64) {
	ts := (id >> timestampShift) + customEpochMs
	nodeID = (id >> nodeShift) & maxNodeID
	sequence = id & maxSequence
	timestamp = time.UnixMilli(ts)
	return
}

// currentTimeMs returns current time in milliseconds since custom epoch.
func (g *Generator) currentTimeMs() int64 {
	return g.now().UnixMilli() - g.epoch
}

// waitNextMs spins until the clock advances past lastMs.
func (g *Generator) waitNextMs(lastMs int64) int64 {
	for {
		now := g.currentTimeMs()
		if now > lastMs {
			return now
		}
		// Yield to other goroutines briefly
		time.Sleep(100 * time.Microsecond)
	}
}
