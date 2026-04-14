package client

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// blockCache stores raw encrypted block bytes on disk. Only encrypted
// content is persisted — no decrypted data touches the cache.
//
// Default cache directory: ${XDG_RUNTIME_DIR}/proton/drive/
// Layout: {linkID-prefix}/{linkID}/block.{index} for block data,
// {linkID-prefix}/{linkID}/link.json for the raw encrypted proton.Link
// metadata. The two-character prefix bucket (first 2 chars of linkID)
// avoids a single directory with millions of entries.
//
// The cache is cross-share — keyed by linkID, not share membership.
// Per-share config controls whether caching is enabled for operations
// on that share, but the cache directory is shared across all shares.
// Caching is PROHIBITED for the root (main) and photos shares — users
// must make a conscious choice about which shared folders are safe to
// cache locally. Only user-created shared folders may have caching
// enabled.
//
// Volume and share metadata are never cached — the upstream API is
// always the authority for the entry-point linkID. Once a root linkID
// is resolved from the API, subtree traversal can hit the cache:
// cached link metadata contains child linkIDs, and those children's
// metadata and blocks are also cached. The API is only consulted on
// cache miss.
//
// Cache I/O errors are logged and treated as misses (best-effort).
type blockCache struct {
	dir string // cache root, defaults to ${XDG_RUNTIME_DIR}/proton/drive/
}

// newBlockCache creates a block cache rooted at dir. Creates the
// directory if it doesn't exist.
func newBlockCache(dir string) (*blockCache, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("blockcache: mkdir %s: %w", dir, err)
	}
	return &blockCache{dir: dir}, nil
}

// linkDir returns the directory for a link's cached data.
// Uses a two-character prefix bucket to avoid flat directory bloat.
func (c *blockCache) linkDir(linkID string) string {
	prefix := linkID
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return filepath.Join(c.dir, prefix, linkID)
}

// blockPath returns the filesystem path for a cached block.
func (c *blockCache) blockPath(linkID string, index int) string {
	return filepath.Join(c.linkDir(linkID), fmt.Sprintf("block.%d", index))
}

// linkPath returns the filesystem path for cached link metadata.
func (c *blockCache) linkPath(linkID string) string {
	return filepath.Join(c.linkDir(linkID), "link.json")
}

// getBlock returns cached block bytes, or nil if not cached.
func (c *blockCache) getBlock(linkID string, index int) ([]byte, error) {
	data, err := os.ReadFile(c.blockPath(linkID, index))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		slog.Warn("blockcache.getBlock", "linkID", linkID, "index", index, "error", err)
		return nil, nil
	}
	return data, nil
}

// putBlock stores block bytes in the cache. Best-effort — errors are logged.
func (c *blockCache) putBlock(linkID string, index int, data []byte) error {
	dir := c.linkDir(linkID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		slog.Warn("blockcache.putBlock: mkdir", "linkID", linkID, "error", err)
		return err
	}
	if err := os.WriteFile(c.blockPath(linkID, index), data, 0600); err != nil {
		slog.Warn("blockcache.putBlock", "linkID", linkID, "index", index, "error", err)
		return err
	}
	return nil
}

// getLink returns cached encrypted link metadata, or nil if not cached.
func (c *blockCache) getLink(linkID string) ([]byte, error) {
	data, err := os.ReadFile(c.linkPath(linkID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		slog.Warn("blockcache.getLink", "linkID", linkID, "error", err)
		return nil, nil
	}
	return data, nil
}

// putLink stores encrypted link metadata in the cache.
func (c *blockCache) putLink(linkID string, data []byte) error {
	dir := c.linkDir(linkID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		slog.Warn("blockcache.putLink: mkdir", "linkID", linkID, "error", err)
		return err
	}
	if err := os.WriteFile(c.linkPath(linkID), data, 0600); err != nil {
		slog.Warn("blockcache.putLink", "linkID", linkID, "error", err)
		return err
	}
	return nil
}

// invalidate removes all cached data for a link (metadata + blocks).
func (c *blockCache) invalidate(linkID string) error {
	return os.RemoveAll(c.linkDir(linkID))
}
