// Package pool provides a bounded worker pool.
//
// Pool wraps golang.org/x/sync/errgroup with throttle integration and
// atomic stats counters. It enforces a concurrency limit and coordinates
// graceful shutdown via context cancellation.
package pool

import (
	"context"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// Waiter blocks until ready or the context is cancelled.
// *api.Throttle satisfies this interface without changes.
type Waiter interface {
	Wait(ctx context.Context) error
}

// Option configures a Pool at construction time.
type Option func(*Pool)

// WithThrottle attaches a Waiter that gates each task dispatch.
// When set, Waiter.Wait is called before every task body executes.
func WithThrottle(w Waiter) Option {
	return func(p *Pool) { p.throttle = w }
}

// Stats holds atomic counters for pool utilization.
type Stats struct {
	active    atomic.Int64
	submitted atomic.Int64
	completed atomic.Int64
}

// StatsSnapshot is a point-in-time copy of pool counters.
type StatsSnapshot struct {
	Active    int64
	Submitted int64
	Completed int64
}

// Snapshot returns a point-in-time copy of the counters.
func (s *Stats) Snapshot() StatsSnapshot {
	return StatsSnapshot{
		Active:    s.active.Load(),
		Submitted: s.submitted.Load(),
		Completed: s.completed.Load(),
	}
}

// Pool wraps errgroup.Group with throttle integration and stats.
type Pool struct {
	g        *errgroup.Group
	ctx      context.Context
	throttle Waiter
	stats    Stats
}

// New creates a pool with n concurrent workers. The context governs
// lifetime via errgroup.WithContext.
func New(ctx context.Context, n int, opts ...Option) *Pool {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(n)
	p := &Pool{g: g, ctx: ctx}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Go submits a task. Blocks if all workers are busy (errgroup
// backpressure via SetLimit). The task receives the pool's context.
func (p *Pool) Go(task func(context.Context) error) {
	p.stats.submitted.Add(1)
	p.g.Go(func() error {
		if p.throttle != nil {
			if err := p.throttle.Wait(p.ctx); err != nil {
				p.stats.completed.Add(1)
				return err
			}
		}
		p.stats.active.Add(1)
		defer p.stats.active.Add(-1)
		defer p.stats.completed.Add(1)
		return task(p.ctx)
	})
}

// Wait blocks until all tasks complete. Returns the first non-nil
// error (errgroup semantics).
func (p *Pool) Wait() error {
	return p.g.Wait()
}

// Stats returns a point-in-time snapshot of pool counters.
func (p *Pool) Stats() StatsSnapshot {
	return p.stats.Snapshot()
}
