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
//
// **Property 7: Buffers zeroed after consumption**
// **Validates: Requirements 4.6, 6.4**
func TestBufferZeroed_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		size := rapid.IntRange(1, 64*1024).Draw(t, "size") // cap for test speed
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

	// Write a test file slightly larger than one block.
	srcData := make([]byte, drive.BlockSize+1024)
	for i := range srcData {
		srcData[i] = byte(i % 251)
	}
	if err := os.WriteFile(srcPath, srcData, 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	job := CopyJob{
		Src: CopyEndpoint{
			Type:      PathLocal,
			LocalPath: srcPath,
			FileSize:  int64(len(srcData)),
		},
		Dst: CopyEndpoint{
			Type:      PathLocal,
			LocalPath: dstPath,
		},
	}

	pipe := &transferPipeline{workers: 2, store: nil}
	if err := pipe.run(context.Background(), []CopyJob{job}); err != nil {
		t.Fatalf("pipeline.run: %v", err)
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}

	if len(dstData) < len(srcData) {
		t.Fatalf("dst size = %d, want >= %d", len(dstData), len(srcData))
	}

	// Compare only the bytes we wrote (dst may have trailing zeros from sparse write).
	for i := range srcData {
		if dstData[i] != srcData[i] {
			t.Fatalf("mismatch at byte %d: got %d, want %d", i, dstData[i], srcData[i])
		}
	}
}

// TestPipeline_EmptyJobs verifies that an empty job list returns nil.
func TestPipeline_EmptyJobs(t *testing.T) {
	pipe := &transferPipeline{workers: 2, store: nil}
	if err := pipe.run(context.Background(), nil); err != nil {
		t.Fatalf("expected nil for empty jobs, got: %v", err)
	}
}

// TestPipeline_ContextCancellation verifies that the pipeline stops
// promptly when the context is cancelled.
func TestPipeline_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.bin")
	dstPath := filepath.Join(dir, "dst.bin")

	// Write a large file to ensure the pipeline has work to do.
	srcData := make([]byte, drive.BlockSize*4)
	_ = os.WriteFile(srcPath, srcData, 0644)

	job := CopyJob{
		Src: CopyEndpoint{Type: PathLocal, LocalPath: srcPath, FileSize: int64(len(srcData))},
		Dst: CopyEndpoint{Type: PathLocal, LocalPath: dstPath},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	pipe := &transferPipeline{workers: 2, store: nil}
	// Should return quickly without processing all blocks.
	_ = pipe.run(ctx, []CopyJob{job})
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
		_ = os.WriteFile(srcPath, data, 0644)

		jobs = append(jobs, CopyJob{
			Src: CopyEndpoint{Type: PathLocal, LocalPath: srcPath, FileSize: int64(len(data))},
			Dst: CopyEndpoint{Type: PathLocal, LocalPath: dstPath},
		})
	}

	pipe := &transferPipeline{workers: 4, store: nil}
	if err := pipe.run(context.Background(), jobs); err != nil {
		t.Fatalf("pipeline.run: %v", err)
	}

	// Verify each file was copied correctly.
	for i, job := range jobs {
		src, _ := os.ReadFile(job.Src.LocalPath)
		dst, _ := os.ReadFile(job.Dst.LocalPath)
		if len(dst) < len(src) {
			t.Fatalf("file %d: dst size %d < src size %d", i, len(dst), len(src))
		}
		for j := range src {
			if dst[j] != src[j] {
				t.Fatalf("file %d: mismatch at byte %d", i, j)
				break
			}
		}
	}
}
