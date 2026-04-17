package client

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

// TestLocalReadWrite_RoundTrip_Property writes random data to a file,
// reads it back via LocalReader, and verifies the data matches.
//
// **Validates: Requirements 2.3**
func TestLocalReadWrite_RoundTrip_Property(t *testing.T) {
	dir := t.TempDir()
	rapid.Check(t, func(t *rapid.T) {
		// Use moderate sizes to keep test fast while still exercising
		// multi-block reads (BlockSize boundary + partial last block).
		size := int64(rapid.IntRange(1, drive.BlockSize+4096).Draw(t, "size"))
		data := make([]byte, size)
		// Fill with a deterministic pattern seeded from size — avoids
		// drawing O(size) random bytes which dominates runtime.
		for i := range data {
			data[i] = byte((i * 251) + int(size%127)) //nolint:gosec // deterministic test pattern
		}

		srcPath := filepath.Join(dir, rapid.StringMatching(`[a-z]{8}`).Draw(t, "name")+".bin")
		if err := os.WriteFile(srcPath, data, 0600); err != nil {
			t.Fatalf("write: %v", err)
		}

		r := NewLocalReader(srcPath, size)
		if r.BlockCount() != drive.BlockCount(size) {
			t.Fatalf("BlockCount = %d, want %d", r.BlockCount(), drive.BlockCount(size))
		}

		var reassembled []byte
		for i := 0; i < r.BlockCount(); i++ {
			bs := r.BlockSize(i)
			buf := make([]byte, bs)
			n, err := r.ReadBlock(context.Background(), i, buf)
			if err != nil {
				t.Fatalf("ReadBlock(%d): %v", i, err)
			}
			reassembled = append(reassembled, buf[:n]...)
		}

		if !bytes.Equal(reassembled, data) {
			t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(reassembled), len(data))
		}
	})
}

// TestTransferOpts_Workers_TableDriven extends worker count tests with
// additional edge cases.
func TestTransferOpts_Workers_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		workers int
		want    int
	}{
		{"zero defaults", 0, DefaultWorkers},
		{"negative defaults", -1, DefaultWorkers},
		{"negative large", -100, DefaultWorkers},
		{"one", 1, 1},
		{"custom", 16, 16},
		{"large", 1000, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TransferOpts{Workers: tt.workers}
			if got := opts.workers(); got != tt.want {
				t.Fatalf("workers() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestBlockMap tests the blockMap claim logic.
func TestBlockMap(t *testing.T) {
	tests := []struct {
		name       string
		blockCount int
		claims     int
		wantLast   int // expected last claimed index, or -1
	}{
		{"single block", 1, 1, 0},
		{"single block exhausted", 1, 2, -1},
		{"multi block", 5, 3, 2},
		{"all claimed", 3, 4, -1},
		{"zero blocks", 0, 1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a minimal CopyJob with a stub reader that reports blockCount.
			r := NewLocalReader("/dev/null", int64(tt.blockCount)*drive.BlockSize)
			// Override nBlocks for the zero case.
			if tt.blockCount == 0 {
				r = NewLocalReader("/dev/null", 0)
			}
			job := &CopyJob{Src: r}
			bm := newBlockMap(job)

			var last int
			for i := 0; i < tt.claims; i++ {
				last = bm.claim()
			}
			if last != tt.wantLast {
				t.Fatalf("after %d claims: last = %d, want %d", tt.claims, last, tt.wantLast)
			}
		})
	}
}

// TestLocalReader_Describe returns the path.
func TestLocalReader_Describe(t *testing.T) {
	r := NewLocalReader("/some/path.bin", 100)
	if got := r.Describe(); got != "/some/path.bin" {
		t.Fatalf("Describe() = %q, want %q", got, "/some/path.bin")
	}
}

// TestLocalWriter_Describe returns the path.
func TestLocalWriter_Describe(t *testing.T) {
	w := NewLocalWriter("/some/output.bin")
	if got := w.Describe(); got != "/some/output.bin" {
		t.Fatalf("Describe() = %q, want %q", got, "/some/output.bin")
	}
}

// TestLocalReader_Close is a no-op.
func TestLocalReader_Close(t *testing.T) {
	r := NewLocalReader("/dev/null", 0)
	if err := r.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
}

// TestLocalWriter_Close is a no-op.
func TestLocalWriter_Close(t *testing.T) {
	w := NewLocalWriter("/dev/null")
	if err := w.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
}

// TestLocalReader_ReadBlock_FileNotFound verifies ReadBlock returns an
// error when the file doesn't exist.
func TestLocalReader_ReadBlock_FileNotFound(t *testing.T) {
	r := NewLocalReader("/nonexistent/path.bin", 1024)
	buf := make([]byte, drive.BlockSize)
	_, err := r.ReadBlock(context.Background(), 0, buf)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestLocalWriter_WriteBlock_FileNotFound verifies WriteBlock returns an
// error when the file doesn't exist.
func TestLocalWriter_WriteBlock_FileNotFound(t *testing.T) {
	w := NewLocalWriter("/nonexistent/path.bin")
	err := w.WriteBlock(context.Background(), 0, []byte("data"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestLocalReader_BlockSize_BeyondEnd verifies BlockSize returns 0 for
// indices beyond the file.
func TestLocalReader_BlockSize_BeyondEnd(t *testing.T) {
	r := NewLocalReader("/dev/null", 100)
	if got := r.BlockSize(1); got != 0 {
		t.Fatalf("BlockSize(1) = %d, want 0 for 100-byte file", got)
	}
}
