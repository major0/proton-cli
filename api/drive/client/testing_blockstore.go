package client

// ExportNewBlockCache exposes newBlockCache for testing.
func ExportNewBlockCache(dir string) (*BlockCacheExport, error) {
	c, err := newBlockCache(dir)
	if err != nil {
		return nil, err
	}
	return &BlockCacheExport{c: c}, nil
}

// BlockCacheExport wraps blockCache for external test access.
type BlockCacheExport struct {
	c *blockCache
}

// ExportGetBlock wraps blockCache.getBlock for testing.
func (e *BlockCacheExport) ExportGetBlock(linkID string, index int) ([]byte, error) {
	return e.c.getBlock(linkID, index)
}

// ExportPutBlock wraps blockCache.putBlock for testing.
func (e *BlockCacheExport) ExportPutBlock(linkID string, index int, data []byte) error {
	return e.c.putBlock(linkID, index, data)
}

// ExportGetLink wraps blockCache.getLink for testing.
func (e *BlockCacheExport) ExportGetLink(linkID string) ([]byte, error) {
	return e.c.getLink(linkID)
}

// ExportPutLink wraps blockCache.putLink for testing.
func (e *BlockCacheExport) ExportPutLink(linkID string, data []byte) error {
	return e.c.putLink(linkID, data)
}

// ExportInvalidate wraps blockCache.invalidate for testing.
func (e *BlockCacheExport) ExportInvalidate(linkID string) error {
	return e.c.invalidate(linkID)
}
