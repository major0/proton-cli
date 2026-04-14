package client

import (
	"context"
	"fmt"
)

// BulkCopy transfers multiple files concurrently using the block
// pipeline. Each CopyJob must have fully initialized BlockReader and
// BlockWriter endpoints. Individual job errors are collected and
// returned as a joined error without stopping remaining transfers.
func (c *Client) BulkCopy(ctx context.Context, jobs []CopyJob, opts TransferOpts) error {
	if err := RunPipeline(ctx, jobs, opts); err != nil {
		return fmt.Errorf("drive.BulkCopy: %w", err)
	}
	return nil
}
