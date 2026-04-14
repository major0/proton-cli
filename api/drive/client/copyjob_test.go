package client

import (
	"testing"

	"github.com/major0/proton-cli/api/drive"
	"pgregory.net/rapid"
)

// TestLocalReader_BlockCount_Property verifies that BlockCount and
// BlockSize produce consistent results for arbitrary file sizes.
func TestLocalReader_BlockCount_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		size := int64(rapid.IntRange(0, drive.BlockSize*20).Draw(t, "size"))
		r := NewLocalReader("/dev/null", size)

		wantBlocks := drive.BlockCount(size)
		if r.BlockCount() != wantBlocks {
			t.Fatalf("BlockCount() = %d, want %d for size %d", r.BlockCount(), wantBlocks, size)
		}

		var totalSize int64
		for i := 0; i < r.BlockCount(); i++ {
			bs := r.BlockSize(i)
			if bs <= 0 || bs > drive.BlockSize {
				t.Fatalf("BlockSize(%d) = %d, out of range", i, bs)
			}
			totalSize += bs
		}
		if totalSize != size {
			t.Fatalf("sum of BlockSize = %d, want %d", totalSize, size)
		}
	})
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
