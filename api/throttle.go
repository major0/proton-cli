package api

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Throttle coordinates rate limiting across concurrent workers. When any
// worker receives a 429, it signals the throttle which pauses all workers
// for the specified duration. Workers call Wait() before each API call.
type Throttle struct {
	mu       sync.Mutex
	until    time.Time
	backoff  time.Duration
	maxDelay time.Duration
}

// NewThrottle creates a throttle with the given initial backoff and max delay.
func NewThrottle(initialBackoff, maxDelay time.Duration) *Throttle {
	return &Throttle{
		backoff:  initialBackoff,
		maxDelay: maxDelay,
	}
}

// Signal notifies the throttle that a 429 was received. If retryAfter > 0,
// use that duration; otherwise use exponential backoff. All workers will
// pause until the delay expires.
func (t *Throttle) Signal(retryAfter time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delay := retryAfter
	if delay <= 0 {
		delay = t.backoff
		t.backoff = min(t.backoff*2, t.maxDelay)
	}

	until := time.Now().Add(delay)
	if until.After(t.until) {
		t.until = until
		slog.Debug("throttle", "delay", delay)
	}
}

// Reset clears the backoff after a successful request.
func (t *Throttle) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Only reset backoff if we're past the throttle window.
	if time.Now().After(t.until) {
		t.backoff /= 2
		if t.backoff < time.Second {
			t.backoff = time.Second
		}
	}
}

// Wait blocks until the throttle window expires or the context is cancelled.
// Returns ctx.Err() if cancelled.
func (t *Throttle) Wait(ctx context.Context) error {
	t.mu.Lock()
	until := t.until
	t.mu.Unlock()

	delay := time.Until(until)
	if delay <= 0 {
		return nil
	}

	slog.Debug("throttle.wait", "delay", delay)

	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
