package client

import (
	"testing"

	"github.com/major0/proton-cli/api/drive"
	"pgregory.net/rapid"
)

// TestExpandBlocks_Property verifies that expandBlocks produces BlockJob
// items with 1-based indices and cumulative byte offsets matching the
// input block sizes.
//
// **Property 5: BlockJob indices and offsets from block sizes**
// **Validates: Requirements 3.1, 3.2**
func TestExpandBlocks_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(t, "nBlocks")
		sizes := make([]int64, n)
		var totalSize int64
		for i := range sizes {
			sizes[i] = int64(rapid.IntRange(1, drive.BlockSize).Draw(t, "blockSize"))
			totalSize += sizes[i]
		}

		job := &CopyJob{
			Src: CopyEndpoint{
				Type:       PathLocal,
				FileSize:   totalSize,
				BlockSizes: sizes,
			},
		}

		blocks := expandBlocks(job)

		if len(blocks) != n {
			t.Fatalf("expected %d blocks, got %d", n, len(blocks))
		}

		var offset int64
		for i, bj := range blocks {
			// 1-based index.
			if bj.Index != i+1 {
				t.Fatalf("block %d: Index = %d, want %d", i, bj.Index, i+1)
			}
			// Cumulative offset.
			if bj.Offset != offset {
				t.Fatalf("block %d: Offset = %d, want %d", i, bj.Offset, offset)
			}
			// Size matches input.
			if bj.Size != sizes[i] {
				t.Fatalf("block %d: Size = %d, want %d", i, bj.Size, sizes[i])
			}
			// Job reference.
			if bj.Job != job {
				t.Fatalf("block %d: Job pointer mismatch", i)
			}
			offset += sizes[i]
		}
	})
}

// TestExpandBlocks_FromFileSize verifies that when BlockSizes is nil,
// expandBlocks computes blocks from FileSize.
func TestExpandBlocks_FromFileSize(t *testing.T) {
	tests := []struct {
		fileSize   int64
		wantBlocks int
		wantLast   int64
	}{
		{0, 0, 0},
		{1, 1, 1},
		{drive.BlockSize, 1, drive.BlockSize},
		{drive.BlockSize + 1, 2, 1},
		{drive.BlockSize * 3, 3, drive.BlockSize},
	}

	for _, tt := range tests {
		job := &CopyJob{Src: CopyEndpoint{Type: PathLocal, FileSize: tt.fileSize}}
		blocks := expandBlocks(job)

		if len(blocks) != tt.wantBlocks {
			t.Errorf("FileSize=%d: got %d blocks, want %d", tt.fileSize, len(blocks), tt.wantBlocks)
			continue
		}
		if tt.wantBlocks > 0 && blocks[len(blocks)-1].Size != tt.wantLast {
			t.Errorf("FileSize=%d: last block size = %d, want %d", tt.fileSize, blocks[len(blocks)-1].Size, tt.wantLast)
		}
	}
}

// TestTransferOpts_DefaultWorkers verifies the default worker count.
func TestTransferOpts_DefaultWorkers(t *testing.T) {
	opts := TransferOpts{}
	if got := opts.workers(); got != DefaultWorkers {
		t.Fatalf("default workers = %d, want %d", got, DefaultWorkers)
	}

	opts.Workers = 16
	if got := opts.workers(); got != 16 {
		t.Fatalf("custom workers = %d, want 16", got)
	}

	opts.Workers = -1
	if got := opts.workers(); got != DefaultWorkers {
		t.Fatalf("negative workers = %d, want %d", got, DefaultWorkers)
	}
}
