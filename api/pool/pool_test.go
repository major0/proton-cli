package pool_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/major0/proton-cli/api/pool"
)

// TestPool_GoBlocks verifies that the N+1th Go call blocks when N
// workers are busy.
func TestPool_GoBlocks(t *testing.T) {
	const n = 2
	ctx := context.Background()
	p := pool.New(ctx, n)

	// Block both workers.
	started := make(chan struct{}, n)
	release := make(chan struct{})
	for i := 0; i < n; i++ {
		p.Go(func(_ context.Context) error {
			started <- struct{}{}
			<-release
			return nil
		})
	}

	// Wait for both workers to be running.
	for i := 0; i < n; i++ {
		<-started
	}

	// The N+1th Go should block because all workers are busy.
	submitted := make(chan struct{})
	go func() {
		p.Go(func(_ context.Context) error {
			return nil
		})
		close(submitted)
	}()

	select {
	case <-submitted:
		t.Fatal("Go did not block when all workers busy")
	case <-time.After(50 * time.Millisecond):
		// Expected: Go is blocked.
	}

	// Release workers so pool can drain.
	close(release)

	// The blocked Go should now proceed.
	select {
	case <-submitted:
		// Good.
	case <-time.After(time.Second):
		t.Fatal("Go still blocked after workers released")
	}

	if err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

// TestPool_NoThrottle verifies that tasks dispatch without delay when
// no throttle is configured.
func TestPool_NoThrottle(t *testing.T) {
	const taskCount = 10
	ctx := context.Background()
	p := pool.New(ctx, 4)

	var count atomic.Int64
	for i := 0; i < taskCount; i++ {
		p.Go(func(_ context.Context) error {
			count.Add(1)
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if got := count.Load(); got != taskCount {
		t.Fatalf("executed %d tasks, want %d", got, taskCount)
	}
}

// TestPool_ZeroTasks verifies that Wait returns immediately when no
// tasks have been submitted.
func TestPool_ZeroTasks(t *testing.T) {
	ctx := context.Background()
	p := pool.New(ctx, 4)

	done := make(chan error, 1)
	go func() {
		done <- p.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not return for zero tasks")
	}

	snap := p.Stats()
	if snap.Submitted != 0 || snap.Completed != 0 || snap.Active != 0 {
		t.Fatalf("stats not zero: %+v", snap)
	}
}

// TestPool_Stats verifies that counters are accurate after a mixed batch
// of successful and failing tasks.
func TestPool_Stats(t *testing.T) {
	const (
		successes = 5
		failures  = 3
		total     = successes + failures
	)

	ctx := context.Background()
	p := pool.New(ctx, total) // enough workers for all tasks

	errBoom := errors.New("boom")

	var mu sync.Mutex
	var errs []error

	for i := 0; i < successes; i++ {
		p.Go(func(_ context.Context) error {
			return nil
		})
	}
	for i := 0; i < failures; i++ {
		p.Go(func(_ context.Context) error {
			mu.Lock()
			errs = append(errs, errBoom)
			mu.Unlock()
			return errBoom
		})
	}

	// errgroup returns the first error; we just need Wait to finish.
	_ = p.Wait()

	snap := p.Stats()
	if snap.Submitted != total {
		t.Fatalf("Submitted %d, want %d", snap.Submitted, total)
	}
	// errgroup cancels on first error, so not all tasks may complete.
	// But completed should be > 0 and <= total.
	if snap.Completed == 0 {
		t.Fatal("Completed is 0, expected at least 1")
	}
	if snap.Completed > total {
		t.Fatalf("Completed %d > total %d", snap.Completed, total)
	}
	if snap.Active != 0 {
		t.Fatalf("Active %d after Wait, want 0", snap.Active)
	}
}
