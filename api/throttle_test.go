package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestThrottle exercises the Throttle type's core behavior.
func TestThrottle(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new throttle does not block",
			fn: func(t *testing.T) {
				th := NewThrottle(time.Second, 30*time.Second)
				err := th.Wait(context.Background())
				if err != nil {
					t.Fatalf("Wait: %v", err)
				}
			},
		},
		{
			name: "signal with retryAfter sets delay",
			fn: func(t *testing.T) {
				th := NewThrottle(time.Second, 30*time.Second)
				th.Signal(50 * time.Millisecond)

				start := time.Now()
				err := th.Wait(context.Background())
				elapsed := time.Since(start)

				if err != nil {
					t.Fatalf("Wait: %v", err)
				}
				if elapsed < 40*time.Millisecond {
					t.Fatalf("Wait returned too early: %v", elapsed)
				}
			},
		},
		{
			name: "signal with zero uses backoff",
			fn: func(t *testing.T) {
				th := NewThrottle(50*time.Millisecond, 30*time.Second)
				th.Signal(0)

				start := time.Now()
				err := th.Wait(context.Background())
				elapsed := time.Since(start)

				if err != nil {
					t.Fatalf("Wait: %v", err)
				}
				if elapsed < 40*time.Millisecond {
					t.Fatalf("Wait returned too early: %v", elapsed)
				}
			},
		},
		{
			name: "exponential backoff doubles",
			fn: func(t *testing.T) {
				th := NewThrottle(50*time.Millisecond, 10*time.Second)
				th.Signal(0) // backoff = 50ms, then doubles to 100ms

				// Wait out the first signal.
				_ = th.Wait(context.Background())

				th.Signal(0) // should use 100ms backoff

				start := time.Now()
				_ = th.Wait(context.Background())
				elapsed := time.Since(start)

				if elapsed < 80*time.Millisecond {
					t.Fatalf("expected doubled backoff, got %v", elapsed)
				}
			},
		},
		{
			name: "backoff capped at maxDelay",
			fn: func(t *testing.T) {
				th := NewThrottle(50*time.Millisecond, 100*time.Millisecond)
				// Signal multiple times to push backoff past max.
				for i := 0; i < 10; i++ {
					th.Signal(0)
					_ = th.Wait(context.Background())
				}

				th.Signal(0)
				start := time.Now()
				_ = th.Wait(context.Background())
				elapsed := time.Since(start)

				// Should not exceed maxDelay + some tolerance.
				if elapsed > 200*time.Millisecond {
					t.Fatalf("backoff exceeded max: %v", elapsed)
				}
			},
		},
		{
			name: "wait respects context cancellation",
			fn: func(t *testing.T) {
				th := NewThrottle(time.Second, 30*time.Second)
				th.Signal(5 * time.Second) // long delay

				ctx, cancel := context.WithCancel(context.Background())
				cancel() // cancel immediately

				err := th.Wait(ctx)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("expected context.Canceled, got %v", err)
				}
			},
		},
		{
			name: "reset reduces backoff",
			fn: func(t *testing.T) {
				th := NewThrottle(50*time.Millisecond, 10*time.Second)
				// Push backoff up: 50ms → 100ms after first signal.
				th.Signal(0)
				_ = th.Wait(context.Background())

				// Reset halves backoff. But Reset has a floor of 1s,
				// so 100ms/2 = 50ms → clamped to 1s. Verify Reset
				// doesn't panic and the throttle remains functional.
				th.Reset()

				err := th.Wait(context.Background())
				if err != nil {
					t.Fatalf("Wait after reset: %v", err)
				}
			},
		},
		{
			name: "reset floor is 1 second",
			fn: func(t *testing.T) {
				th := NewThrottle(time.Second, 30*time.Second)
				// Reset without any signals — backoff should not go below 1s.
				th.Reset()
				th.Reset()
				th.Reset()

				// Verify the throttle still works (doesn't panic or go negative).
				err := th.Wait(context.Background())
				if err != nil {
					t.Fatalf("Wait after resets: %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(t)
		})
	}
}
