package client_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/major0/proton-cli/api/drive/client"
	"pgregory.net/rapid"
)

func TestBlockCache_GetPutBlock(t *testing.T) {
	cache, err := client.ExportNewBlockCache(t.TempDir())
	if err != nil {
		t.Fatalf("newBlockCache: %v", err)
	}

	linkID := "test-link-abc"
	data := []byte("encrypted-block-content")

	// Miss before put.
	got, err := cache.ExportGetBlock(linkID, 1)
	if err != nil {
		t.Fatalf("getBlock (miss): %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on miss, got %d bytes", len(got))
	}

	// Put.
	if err := cache.ExportPutBlock(linkID, 1, data); err != nil {
		t.Fatalf("putBlock: %v", err)
	}

	// Hit after put.
	got, err = cache.ExportGetBlock(linkID, 1)
	if err != nil {
		t.Fatalf("getBlock (hit): %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("cache hit mismatch")
	}
}

func TestBlockCache_GetPutLink(t *testing.T) {
	cache, err := client.ExportNewBlockCache(t.TempDir())
	if err != nil {
		t.Fatalf("newBlockCache: %v", err)
	}

	linkID := "test-link-xyz"
	data := []byte(`{"LinkID":"test-link-xyz","Type":1}`)

	// Miss.
	got, err := cache.ExportGetLink(linkID)
	if err != nil {
		t.Fatalf("getLink (miss): %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on miss")
	}

	// Put + hit.
	if err := cache.ExportPutLink(linkID, data); err != nil {
		t.Fatalf("putLink: %v", err)
	}
	got, err = cache.ExportGetLink(linkID)
	if err != nil {
		t.Fatalf("getLink (hit): %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("link cache mismatch")
	}
}

func TestBlockCache_Invalidate(t *testing.T) {
	cache, err := client.ExportNewBlockCache(t.TempDir())
	if err != nil {
		t.Fatalf("newBlockCache: %v", err)
	}

	linkID := "test-link-inv"
	_ = cache.ExportPutBlock(linkID, 1, []byte("block1"))
	_ = cache.ExportPutLink(linkID, []byte("link-meta"))

	if err := cache.ExportInvalidate(linkID); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	got, _ := cache.ExportGetBlock(linkID, 1)
	if got != nil {
		t.Fatal("block should be gone after invalidate")
	}
	got, _ = cache.ExportGetLink(linkID)
	if got != nil {
		t.Fatal("link should be gone after invalidate")
	}
}

// TestBlockCache_RoundTrip_Property verifies that for any linkID, block
// index, and data, putBlock followed by getBlock returns the exact same bytes.
//
// **Property 4: BlockStore returns unmodified encrypted bytes**
// **Validates: Requirements 2.4**
func TestBlockCache_RoundTrip_Property(t *testing.T) {
	cache, err := client.ExportNewBlockCache(t.TempDir())
	if err != nil {
		t.Fatalf("newBlockCache: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		linkID := rapid.StringMatching(`[a-zA-Z0-9_-]{8,32}`).Draw(t, "linkID")
		index := rapid.IntRange(1, 100).Draw(t, "index")
		size := rapid.IntRange(1, 64*1024).Draw(t, "size") // cap at 64KB for test speed
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("b%d", i)))
		}

		if err := cache.ExportPutBlock(linkID, index, data); err != nil {
			t.Fatalf("putBlock: %v", err)
		}

		got, err := cache.ExportGetBlock(linkID, index)
		if err != nil {
			t.Fatalf("getBlock: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("round-trip mismatch for %s block %d", linkID, index)
		}
	})
}

func TestBlockCache_MissReturnsNil(t *testing.T) {
	cache, err := client.ExportNewBlockCache(t.TempDir())
	if err != nil {
		t.Fatalf("newBlockCache: %v", err)
	}

	got, err := cache.ExportGetBlock("nonexistent", 1)
	if err != nil {
		t.Fatalf("getBlock: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on miss")
	}
}

func TestBlockCache_PrefixBucketing(t *testing.T) {
	dir := t.TempDir()
	cache, err := client.ExportNewBlockCache(dir)
	if err != nil {
		t.Fatalf("newBlockCache: %v", err)
	}

	_ = cache.ExportPutBlock("abcdef123", 1, []byte("data"))

	// Verify the prefix bucket directory was created.
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("expected prefix bucket directory")
	}
	if entries[0].Name() != "ab" {
		t.Fatalf("expected prefix bucket 'ab', got %q", entries[0].Name())
	}
}

func TestBlockCache_InvalidDir(t *testing.T) {
	_, err := client.ExportNewBlockCache("/dev/null/impossible")
	if err == nil {
		t.Fatal("expected error for invalid dir")
	}
}

// TestCachePathLayout_Property verifies that block and link paths use
// the {prefix}/{linkID}/ bucketing layout.
//
// **Property 4: Cache path layout uses prefix bucketing**
// **Validates: Requirements 2.6**
func TestCachePathLayout_Property(t *testing.T) {
	dir := t.TempDir()
	cache, err := client.ExportNewBlockCache(dir)
	if err != nil {
		t.Fatalf("newBlockCache: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate linkIDs with at least 2 chars.
		linkID := rapid.StringMatching(`[a-zA-Z0-9_-]{2,32}`).Draw(t, "linkID")
		index := rapid.IntRange(1, 100).Draw(t, "index")
		data := []byte("test-block")

		if err := cache.ExportPutBlock(linkID, index, data); err != nil {
			t.Fatalf("putBlock: %v", err)
		}

		// Verify the prefix bucket directory exists.
		prefix := linkID[:2]
		prefixDir := filepath.Join(dir, prefix)
		if _, err := os.Stat(prefixDir); err != nil {
			t.Fatalf("prefix dir %q missing: %v", prefixDir, err)
		}

		// Verify the linkID directory exists under the prefix.
		linkDir := filepath.Join(prefixDir, linkID)
		if _, err := os.Stat(linkDir); err != nil {
			t.Fatalf("link dir %q missing: %v", linkDir, err)
		}

		// Verify the block file exists.
		blockFile := filepath.Join(linkDir, fmt.Sprintf("block.%d", index))
		if _, err := os.Stat(blockFile); err != nil {
			t.Fatalf("block file %q missing: %v", blockFile, err)
		}
	})
}
