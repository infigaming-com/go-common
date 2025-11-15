package backoff

import (
	"math/rand"
	"sync"
	"time"
)

type Config struct {
	Initial    time.Duration
	Max        time.Duration
	Multiplier float64
	Jitter     float64
}

type Exponential struct {
	mu      sync.Mutex
	current time.Duration
	config  Config
}

func New(cfg Config) *Exponential {
	if cfg.Initial <= 0 {
		cfg.Initial = 200 * time.Millisecond
	}
	if cfg.Max <= 0 {
		cfg.Max = 30 * time.Second
	}
	if cfg.Multiplier <= 1 {
		cfg.Multiplier = 2
	}
	return &Exponential{config: cfg}
}

func (e *Exponential) Next() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current <= 0 {
		e.current = e.config.Initial
	} else {
		e.current = time.Duration(float64(e.current) * e.config.Multiplier)
		if e.current > e.config.Max {
			e.current = e.config.Max
		}
	}
	interval := e.current
	if e.config.Jitter > 0 {
		span := float64(interval) * e.config.Jitter
		interval = interval + time.Duration((rand.Float64()*2-1)*span)
		if interval < 0 {
			interval = e.config.Initial
		}
	}
	return interval
}

func (e *Exponential) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.current = 0
}
