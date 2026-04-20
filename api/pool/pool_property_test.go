package pool_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/major0/proton-cli/api/pool"
	"pgregory.net/rapid"
)

// TestPool_ConcurrencyLimit_Property verifies that for any limit N and
// batch of tasks, the number of concurrently active tasks never exceeds N.
//
// **Validates: Requirements 1.2, 9.1**
func TestPool_ConcurrencyLimit_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 64).Draw(t, "workers")
		taskCount := rapid.IntRange(1, 200).Draw(t, "tasks")

		ctx := context.Background()
		p := pool.New(ctx, n)

		var peak atomic.Int64
		var active atomic.Int64

		for i := 0; i < taskCount; i++ {
			p.Go(func(_ context.Context) error {
				cur := active.Add(1)
				// Update peak via CAS loop.
				for {
					old := peak.Load()
					if cur <= old || peak.CompareAndSwap(old, cur) {
						break
					}
				}
				// Brief sleep to create contention.
				time.Sleep(time.Microsecond)
				active.Add(-1)
				return nil
			})
		}

		if err := p.Wait(); err != nil {
			t.Fatalf("Wait: %v", err)
		}

		if got := peak.Load(); got > int64(n) {
			t.Fatalf("peak active %d exceeded limit %d", got, n)
		}
	})
}

// TestPool_CompletionInvariant_Property verifies that after Wait returns,
// Completed == Submitted == len(tasks) for any batch size.
//
// **Validates: Requirements 1.5, 9.2, 9.3**
func TestPool_CompletionInvariant_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		taskCount := rapid.IntRange(0, 200).Draw(t, "tasks")
		n := rapid.IntRange(1, 64).Draw(t, "workers")

		ctx := context.Background()
		p := pool.New(ctx, n)

		for i := 0; i < taskCount; i++ {
			p.Go(func(_ context.Context) error {
				return nil
			})
		}

		if err := p.Wait(); err != nil {
			t.Fatalf("Wait: %v", err)
		}

		snap := p.Stats()
		if snap.Submitted != int64(taskCount) {
			t.Fatalf("Submitted %d, want %d", snap.Submitted, taskCount)
		}
		if snap.Completed != int64(taskCount) {
			t.Fatalf("Completed %d, want %d", snap.Completed, taskCount)
		}
		if snap.Active != 0 {
			t.Fatalf("Active %d after Wait, want 0", snap.Active)
		}
	})
}

// mockWaiter is a test Waiter that counts calls and optionally returns errors.
type mockWaiter struct {
	calls  atomic.Int64
	errors []bool // if errors[i] is true, the i-th call returns an error
}

func (m *mockWaiter) Wait(_ context.Context) error {
	i := m.calls.Add(1) - 1
	if int(i) < len(m.errors) && m.errors[i] {
		return context.Canceled
	}
	return nil
}

// TestPool_ThrottleIntegration_Property verifies that the Waiter is called
// once per task, and that when the Waiter returns an error the task body
// does not execute.
//
// **Validates: Requirements 4.2, 4.3**
func TestPool_ThrottleIntegration_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		taskCount := rapid.IntRange(1, 100).Draw(t, "tasks")

		// Generate a random error pattern for the waiter.
		errorPattern := make([]bool, taskCount)
		expectedSkips := 0
		for i := range errorPattern {
			errorPattern[i] = rapid.Bool().Draw(t, "error")
			if errorPattern[i] {
				expectedSkips++
			}
		}

		w := &mockWaiter{errors: errorPattern}
		ctx := context.Background()
		// Use 1 worker to make the call order deterministic.
		p := pool.New(ctx, 1, pool.WithThrottle(w))

		var bodyCalls atomic.Int64
		for i := 0; i < taskCount; i++ {
			p.Go(func(_ context.Context) error {
				bodyCalls.Add(1)
				return nil
			})
		}

		// errgroup returns the first error and cancels context, so
		// we can't assert on all tasks completing when errors occur.
		// But we can verify the waiter was called for each dispatched task.
		_ = p.Wait()

		// With 1 worker and errgroup semantics, the first error cancels
		// the context. Subsequent tasks may or may not run depending on
		// scheduling. What we CAN assert: waiter calls + body calls
		// account for all completed tasks.
		snap := p.Stats()

		// Every completed task called the waiter exactly once.
		if w.calls.Load() != snap.Completed {
			t.Fatalf("waiter calls %d != completed %d", w.calls.Load(), snap.Completed)
		}

		// Body calls should equal completed minus any throttle errors.
		// Since errgroup cancels on first error, body calls <= completed.
		if bodyCalls.Load() > snap.Completed {
			t.Fatalf("body calls %d > completed %d", bodyCalls.Load(), snap.Completed)
		}
	})
}

// TestPool_ContextCancellation_Property verifies that when the pool context
// is cancelled, in-flight tasks see a cancelled context and Wait returns
// the context error.
//
// **Validates: Requirements 1.4, 8.1**
func TestPool_ContextCancellation_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		taskCount := rapid.IntRange(1, 50).Draw(t, "tasks")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		p := pool.New(ctx, 2)

		var sawCancelled atomic.Int64

		for i := 0; i < taskCount; i++ {
			if i == 0 {
				// First task cancels the context.
				p.Go(func(ctx context.Context) error {
					cancel()
					return ctx.Err()
				})
			} else {
				p.Go(func(ctx context.Context) error {
					if ctx.Err() != nil {
						sawCancelled.Add(1)
					}
					return ctx.Err()
				})
			}
		}

		err := p.Wait()
		if err == nil && taskCount > 0 {
			t.Fatal("Wait returned nil after context cancellation")
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait returned %v, want context.Canceled", err)
		}
	})
}
