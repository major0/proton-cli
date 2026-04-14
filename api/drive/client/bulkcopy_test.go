package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestBulkCopy_ErrorCollection_Property verifies that when a subset of
// jobs fail, non-failing jobs complete and all errors are collected.
func TestBulkCopy_ErrorCollection_Property(t *testing.T) {
	dir := t.TempDir()

	rapid.Check(t, func(t *rapid.T) {
		nGood := rapid.IntRange(1, 5).Draw(t, "nGood")
		nBad := rapid.IntRange(1, 5).Draw(t, "nBad")

		iterDir := filepath.Join(dir, rapid.StringMatching(`[a-z]{8}`).Draw(t, "iter"))
		_ = os.MkdirAll(iterDir, 0700)

		var jobs []CopyJob

		for i := 0; i < nGood; i++ {
			srcPath := filepath.Join(iterDir, "good"+string(rune('a'+i))+".bin")
			dstPath := filepath.Join(iterDir, "dst-good"+string(rune('a'+i))+".bin")
			data := []byte("good-data")
			_ = os.WriteFile(srcPath, data, 0600)
			_ = os.WriteFile(dstPath, nil, 0600) // pre-create dest
			jobs = append(jobs, CopyJob{
				Src: NewLocalReader(srcPath, int64(len(data))),
				Dst: NewLocalWriter(dstPath),
			})
		}

		for i := 0; i < nBad; i++ {
			srcPath := filepath.Join(iterDir, "nonexistent"+string(rune('a'+i))+".bin")
			dstPath := filepath.Join(iterDir, "dst-bad"+string(rune('a'+i))+".bin")
			_ = os.WriteFile(dstPath, nil, 0600) // pre-create dest
			jobs = append(jobs, CopyJob{
				Src: NewLocalReader(srcPath, 1024),
				Dst: NewLocalWriter(dstPath),
			})
		}

		err := RunPipeline(context.Background(), jobs, TransferOpts{Workers: 2})

		if err == nil {
			t.Fatal("expected errors from bad jobs, got nil")
		}

		for i := 0; i < nGood; i++ {
			dstPath := filepath.Join(iterDir, "dst-good"+string(rune('a'+i))+".bin")
			if _, err := os.Stat(dstPath); err != nil {
				t.Fatalf("good job %d: dst file missing: %v", i, err)
			}
		}

		errStr := err.Error()
		for i := 0; i < nBad; i++ {
			needle := "nonexistent" + string(rune('a'+i))
			if !strings.Contains(errStr, needle) {
				t.Fatalf("error should mention %q: %s", needle, errStr)
			}
		}
	})
}

// TestBulkCopy_Empty verifies that an empty job list returns nil.
func TestBulkCopy_Empty(t *testing.T) {
	if err := RunPipeline(context.Background(), nil, TransferOpts{Workers: 2}); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

// TestBulkCopy_AllSuccess verifies that all-success jobs return nil error.
func TestBulkCopy_AllSuccess(t *testing.T) {
	dir := t.TempDir()
	var jobs []CopyJob
	for i := 0; i < 3; i++ {
		srcPath := filepath.Join(dir, "src"+string(rune('a'+i))+".bin")
		dstPath := filepath.Join(dir, "dst"+string(rune('a'+i))+".bin")
		_ = os.WriteFile(srcPath, []byte("data"), 0600)
		_ = os.WriteFile(dstPath, nil, 0600) // pre-create dest
		jobs = append(jobs, CopyJob{
			Src: NewLocalReader(srcPath, 4),
			Dst: NewLocalWriter(dstPath),
		})
	}

	if err := RunPipeline(context.Background(), jobs, TransferOpts{Workers: 2}); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

// TestBulkCopy_AllFail verifies that all-failure jobs return errors for each.
func TestBulkCopy_AllFail(t *testing.T) {
	dir := t.TempDir()
	var jobs []CopyJob
	for i := 0; i < 3; i++ {
		srcPath := filepath.Join(dir, "missing"+string(rune('a'+i))+".bin")
		dstPath := filepath.Join(dir, "dst"+string(rune('a'+i))+".bin")
		_ = os.WriteFile(dstPath, nil, 0600) // pre-create dest
		jobs = append(jobs, CopyJob{
			Src: NewLocalReader(srcPath, 1024),
			Dst: NewLocalWriter(dstPath),
		})
	}

	err := RunPipeline(context.Background(), jobs, TransferOpts{Workers: 2})
	if err == nil {
		t.Fatal("expected errors, got nil")
	}
}
