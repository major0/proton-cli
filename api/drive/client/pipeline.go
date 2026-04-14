package client

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/major0/proton-cli/api/drive"
)

// RunPipeline transfers files using a pool of workers. Each worker
// claims a block from the current job, reads it from Src, writes it
// to Dst, then claims the next. When the current job has no unclaimed
// blocks, the worker advances to the next job in the queue.
//
// Multiple jobs may have in-flight blocks simultaneously at job
// boundaries, but new blocks are always claimed from the frontmost
// incomplete job. This gives natural breadth-first serialization with
// concurrent overlap during transitions.
func RunPipeline(ctx context.Context, jobs []CopyJob, opts TransferOpts) error {
	if len(jobs) == 0 {
		return nil
	}

	nWorkers := opts.workers()

	// Build block maps for all jobs upfront.
	maps := make([]*blockMap, len(jobs))
	for i := range jobs {
		maps[i] = newBlockMap(&jobs[i])
	}

	// Shared state: current job index and its block map.
	var mu sync.Mutex
	jobIdx := 0

	// Error collection.
	var errMu sync.Mutex
	var errs []error
	addErr := func(err error) {
		errMu.Lock()
		errs = append(errs, err)
		errMu.Unlock()
	}

	// claim returns the next block to process: the CopyJob, block index,
	// and block size. Returns nil job when all jobs are exhausted.
	claim := func() (*CopyJob, int, int64) {
		mu.Lock()
		defer mu.Unlock()
		for jobIdx < len(maps) {
			idx := maps[jobIdx].claim()
			if idx >= 0 {
				job := maps[jobIdx].job
				return job, idx, job.Src.BlockSize(idx)
			}
			// Current job exhausted — advance.
			jobIdx++
		}
		return nil, 0, 0
	}

	var wg sync.WaitGroup
	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, drive.BlockSize)
			for {
				if ctx.Err() != nil {
					return
				}
				job, idx, sz := claim()
				if job == nil {
					return
				}
				n, err := job.Src.ReadBlock(ctx, idx, buf[:sz])
				if err != nil {
					addErr(fmt.Errorf("read %s block %d: %w", job.Src.Describe(), idx, err))
					continue
				}
				if err := job.Dst.WriteBlock(ctx, idx, buf[:n]); err != nil {
					addErr(fmt.Errorf("write %s block %d: %w", job.Dst.Describe(), idx, err))
				}
				clear(buf[:n])
			}
		}()
	}

	wg.Wait()

	// Close all readers and writers.
	for i := range jobs {
		if err := jobs[i].Src.Close(); err != nil {
			addErr(fmt.Errorf("close reader %s: %w", jobs[i].Src.Describe(), err))
		}
		if err := jobs[i].Dst.Close(); err != nil {
			addErr(fmt.Errorf("close writer %s: %w", jobs[i].Dst.Describe(), err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
