package client

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api"
)

// BlockStore fetches and stores encrypted blocks. Session-aware and
// cache-aware — implementations check the on-disk object cache before
// making HTTP requests.
type BlockStore interface {
	// GetBlock fetches a raw encrypted block by linkID and block index.
	// Checks on-disk cache first when enabled. Returns the full block bytes.
	GetBlock(ctx context.Context, linkID string, index int, bareURL, token string) ([]byte, error)
	// RequestUpload obtains upload URLs for a batch of blocks.
	RequestUpload(ctx context.Context, req proton.BlockUploadReq) ([]proton.BlockUploadLink, error)
	// UploadBlock uploads an encrypted block to the given URL.
	UploadBlock(ctx context.Context, linkID string, index int, bareURL, token string, data []byte) error
}

// blockReader wraps a []byte to satisfy resty.MultiPartStream.
type blockReader struct {
	r io.Reader
}

func (b *blockReader) GetMultipartReader() io.Reader { return b.r }

// httpBlockStore implements BlockStore using the session's HTTP transport
// and an optional on-disk block cache.
type httpBlockStore struct {
	session *api.Session
	cache   *blockCache // nil when caching disabled
}

// NewBlockStore creates a BlockStore backed by the session's HTTP transport.
// If cache is non-nil, blocks are checked/populated in the cache.
func NewBlockStore(session *api.Session, cache *blockCache) BlockStore {
	return &httpBlockStore{session: session, cache: cache}
}

// GetBlock fetches a raw encrypted block. Checks the cache first when
// available (by linkID + block index), falls through to HTTP on miss,
// and populates the cache on fetch.
func (s *httpBlockStore) GetBlock(ctx context.Context, linkID string, index int, bareURL, token string) ([]byte, error) {
	// Cache check.
	if s.cache != nil {
		if data, err := s.cache.getBlock(linkID, index); err == nil && data != nil {
			return data, nil
		}
	}

	// HTTP fetch.
	rc, err := s.session.Client.GetBlock(ctx, bareURL, token)
	if err != nil {
		return nil, fmt.Errorf("blockstore.GetBlock %s block %d: %w", linkID, index, err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("blockstore.GetBlock %s block %d: read: %w", linkID, index, err)
	}

	// Cache populate (best-effort).
	if s.cache != nil {
		_ = s.cache.putBlock(linkID, index, data)
	}

	return data, nil
}

// RequestUpload obtains upload URLs for a batch of blocks.
func (s *httpBlockStore) RequestUpload(ctx context.Context, req proton.BlockUploadReq) ([]proton.BlockUploadLink, error) {
	links, err := s.session.Client.RequestBlockUpload(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("blockstore.RequestUpload: %w", err)
	}
	return links, nil
}

// UploadBlock uploads an encrypted block to the given URL.
func (s *httpBlockStore) UploadBlock(ctx context.Context, linkID string, index int, bareURL, token string, data []byte) error {
	stream := &blockReader{r: bytes.NewReader(data)}
	if err := s.session.Client.UploadBlock(ctx, bareURL, token, stream); err != nil {
		return fmt.Errorf("blockstore.UploadBlock %s block %d: %w", linkID, index, err)
	}

	// Cache populate (best-effort).
	if s.cache != nil {
		_ = s.cache.putBlock(linkID, index, data)
	}

	return nil
}
