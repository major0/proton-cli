package client

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api/drive"
)

// ProtonReader reads blocks from a Proton Drive file via the BlockStore.
type ProtonReader struct {
	linkID     string
	blocks     []proton.Block
	sessionKey *crypto.SessionKey
	fileSize   int64
	blockSizes []int64
	store      BlockStore
	nBlocks    int
}

// NewProtonReader creates a BlockReader for a Proton Drive file.
func NewProtonReader(linkID string, blocks []proton.Block, sessionKey *crypto.SessionKey, fileSize int64, blockSizes []int64, store BlockStore) *ProtonReader {
	n := len(blockSizes)
	if n == 0 {
		n = drive.BlockCount(fileSize)
	}
	return &ProtonReader{
		linkID:     linkID,
		blocks:     blocks,
		sessionKey: sessionKey,
		fileSize:   fileSize,
		blockSizes: blockSizes,
		store:      store,
		nBlocks:    n,
	}
}

// ReadBlock fetches block at index from the BlockStore.
func (r *ProtonReader) ReadBlock(ctx context.Context, index int, buf []byte) (int, error) {
	if index >= len(r.blocks) {
		return 0, fmt.Errorf("block index %d out of range (have %d blocks)", index, len(r.blocks))
	}
	pb := r.blocks[index]

	encrypted, err := r.store.GetBlock(ctx, r.linkID, index+1, pb.BareURL, pb.Token)
	if err != nil {
		return 0, err
	}

	// TODO: decrypt encrypted block using sessionKey into buf.
	n := copy(buf, encrypted)
	return n, nil
}

// BlockCount returns the total number of blocks.
func (r *ProtonReader) BlockCount() int { return r.nBlocks }

// BlockSize returns the size of block at index.
func (r *ProtonReader) BlockSize(index int) int64 {
	if index < len(r.blockSizes) {
		return r.blockSizes[index]
	}
	offset := int64(index) * drive.BlockSize
	remaining := r.fileSize - offset
	if remaining <= 0 {
		return 0
	}
	if remaining > drive.BlockSize {
		return drive.BlockSize
	}
	return remaining
}

// Describe returns the link ID.
func (r *ProtonReader) Describe() string { return r.linkID }

// Close is a no-op.
func (r *ProtonReader) Close() error { return nil }

// ProtonWriter writes blocks to a Proton Drive file via the BlockStore.
type ProtonWriter struct {
	linkID     string
	revisionID string
	sessionKey *crypto.SessionKey
	store      BlockStore
}

// NewProtonWriter creates a BlockWriter for a Proton Drive file.
func NewProtonWriter(linkID, revisionID string, sessionKey *crypto.SessionKey, store BlockStore) *ProtonWriter {
	return &ProtonWriter{
		linkID:     linkID,
		revisionID: revisionID,
		sessionKey: sessionKey,
		store:      store,
	}
}

// WriteBlock encrypts and uploads a block.
func (w *ProtonWriter) WriteBlock(_ context.Context, _ int, _ []byte) error {
	// TODO: encrypt data using sessionKey, then upload via store.
	return nil
}

// Describe returns the link ID.
func (w *ProtonWriter) Describe() string { return w.linkID }

// Close is a no-op.
func (w *ProtonWriter) Close() error { return nil }
