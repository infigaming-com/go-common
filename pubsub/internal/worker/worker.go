package worker

import (
	"context"
	"errors"
	"sync"
)

var ErrClosed = errors.New("worker: pool closed")

type Pool struct {
	size   int
	ch     chan job
	once   sync.Once
	wg     sync.WaitGroup
	mu     sync.RWMutex
	closed bool
}

type job struct {
	ctx context.Context
	fn  func(context.Context)
}

func New(size int, queue int) *Pool {
	if size <= 0 {
		size = 1
	}
	if queue <= 0 {
		queue = size
	}
	p := &Pool{
		size: size,
		ch:   make(chan job, queue),
	}
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for j := range p.ch {
				j.fn(j.ctx)
			}
		}()
	}
	return p
}

func (p *Pool) Submit(ctx context.Context, fn func(context.Context)) error {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	select {
	case p.ch <- job{ctx: ctx, fn: fn}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pool) Close() {
	p.once.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		close(p.ch)
	})
}

func (p *Pool) Wait() {
	p.wg.Wait()
}
