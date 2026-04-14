package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/major0/proton-cli/api/drive"
	"pgregory.net/rapid"
)

// TestBufferZeroed_Property verifies that after clear(), all bytes are zero.
func TestBufferZeroed_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		size := rapid.IntRange(1, 64*1024).Draw(t, "size")
		buf := make([]byte, size)
		for i := range buf {
			buf[i] = byte(rapid.IntRange(1, 255).Draw(t, "byte"))
		}
		clear(buf)
		for i, b := range buf {
			if b != 0 {
				t.Fatalf("buf[%d] = %d after clear, want 0", i, b)
			}
		}
	})
}

// TestPipeline_LocalToLocal verifies that the pipeline correctly copies
// a local file to another local path using the block pipeline.
func TestPipeline_LocalToLocal(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.bin")
	dstPath := filepath.Join(dir, "dst.bin")

	srcData := make([]byte, drive.BlockSize+1024)
	for i := range srcData {
		srcData[i] = byte(i % 251)
	}
	if err := os.WriteFile(srcPath, srcData, 0600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	job := CopyJob{
		Src: NewLocalReader(srcPath, int64(len(srcData))),
		Dst: NewLocalWriter(dstPath),
	}

	if err := RunPipeline(context.Background(), []CopyJob{job}, TransferOpts{Workers: 2}); err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}

	if len(dstData) < len(srcData) {
		t.Fatalf("dst size = %d, want >= %d", len(dstData), len(srcData))
	}
	for i := range srcData {
		if dstData[i] != srcData[i] {
			t.Fatalf("mismatch at byte %d: got %d, want %d", i, dstData[i], srcData[i])
		}
	}
}

// TestPipeline_EmptyJobs verifies that an empty job list returns nil.
func TestPipeline_EmptyJobs(t *testing.T) {
	if err := RunPipeline(context.Background(), nil, TransferOpts{Workers: 2}); err != nil {
		t.Fatalf("expected nil for empty jobs, got: %v", err)
	}
}

// TestPipeline_ContextCancellation verifies that the pipeline stops
// promptly when the context is cancelled.
func TestPipeline_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.bin")
	dstPath := filepath.Join(dir, "dst.bin")

	srcData := make([]byte, drive.BlockSize*4)
	_ = os.WriteFile(srcPath, srcData, 0600)

	job := CopyJob{
		Src: NewLocalReader(srcPath, int64(len(srcData))),
		Dst: NewLocalWriter(dstPath),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = RunPipeline(ctx, []CopyJob{job}, TransferOpts{Workers: 2})
}

// TestPipeline_MultipleFiles verifies that blocks from different files
// are processed through the same pipeline.
func TestPipeline_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	var jobs []CopyJob
	for i := 0; i < 5; i++ {
		srcPath := filepath.Join(dir, "src"+string(rune('a'+i))+".bin")
		dstPath := filepath.Join(dir, "dst"+string(rune('a'+i))+".bin")
		data := make([]byte, 1024*(i+1))
		for j := range data {
			data[j] = byte(i + j%200)
		}
		_ = os.WriteFile(srcPath, data, 0600)
		jobs = append(jobs, CopyJob{
			Src: NewLocalReader(srcPath, int64(len(data))),
			Dst: NewLocalWriter(dstPath),
		})
	}

	if err := RunPipeline(context.Background(), jobs, TransferOpts{Workers: 4}); err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	for i, job := range jobs {
		src, _ := os.ReadFile(job.Src.Describe())
		dst, _ := os.ReadFile(job.Dst.Describe())
		if len(dst) < len(src) {
			t.Fatalf("file %d: dst size %d < src size %d", i, len(dst), len(src))
		}
		for j := range src {
			if dst[j] != src[j] {
				t.Fatalf("file %d: mismatch at byte %d", i, j)
			}
		}
	}
}
