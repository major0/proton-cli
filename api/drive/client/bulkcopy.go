package client

import (
	"context"
	"fmt"
)

// BulkCopy transfers multiple files concurrently using the shared
// reader/writer pipeline. Each CopyJob must be fully resolved before
// calling this function. Individual job errors are collected and
// returned as a joined error without stopping remaining transfers.
func (c *Client) BulkCopy(ctx context.Context, jobs []CopyJob, opts TransferOpts) error {
	if len(jobs) == 0 {
		return nil
	}

	store := NewBlockStore(c.Session, nil) // TODO: wire cache from share config
	pipe := &transferPipeline{
		workers: opts.workers(),
		store:   store,
		client:  c,
	}

	if err := pipe.run(ctx, jobs); err != nil {
		return fmt.Errorf("drive.BulkCopy: %w", err)
	}

	return nil
}
