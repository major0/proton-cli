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
	LinkID     string
	Blocks     []proton.Block
	SessionKey *crypto.SessionKey
	FileSize   int64
	BlockSizes []int64
	Store      BlockStore
	nBlocks    int
}

// NewProtonReader creates a BlockReader for a Proton Drive file.
func NewProtonReader(linkID string, blocks []proton.Block, sessionKey *crypto.SessionKey, fileSize int64, blockSizes []int64, store BlockStore) *ProtonReader {
	n := len(blockSizes)
	if n == 0 {
		n = drive.BlockCount(fileSize)
	}
	return &ProtonReader{
		LinkID:     linkID,
		Blocks:     blocks,
		SessionKey: sessionKey,
		FileSize:   fileSize,
		BlockSizes: blockSizes,
		Store:      store,
		nBlocks:    n,
	}
}

// ReadBlock fetches block at index from the BlockStore.
func (r *ProtonReader) ReadBlock(ctx context.Context, index int, buf []byte) (int, error) {
	if index >= len(r.Blocks) {
		return 0, fmt.Errorf("block index %d out of range (have %d blocks)", index, len(r.Blocks))
	}
	pb := r.Blocks[index]

	encrypted, err := r.Store.GetBlock(ctx, r.LinkID, index+1, pb.BareURL, pb.Token)
	if err != nil {
		return 0, err
	}

	// TODO: decrypt encrypted block using SessionKey into buf.
	n := copy(buf, encrypted)
	return n, nil
}

// BlockCount returns the total number of blocks.
func (r *ProtonReader) BlockCount() int { return r.nBlocks }

// BlockSize returns the size of block at index.
func (r *ProtonReader) BlockSize(index int) int64 {
	if index < len(r.BlockSizes) {
		return r.BlockSizes[index]
	}
	offset := int64(index) * drive.BlockSize
	remaining := r.FileSize - offset
	if remaining <= 0 {
		return 0
	}
	if remaining > drive.BlockSize {
		return drive.BlockSize
	}
	return remaining
}

// Describe returns the link ID.
func (r *ProtonReader) Describe() string { return r.LinkID }

// Close is a no-op.
func (r *ProtonReader) Close() error { return nil }

// ProtonWriter writes blocks to a Proton Drive file via the BlockStore.
type ProtonWriter struct {
	LinkID     string
	RevisionID string
	SessionKey *crypto.SessionKey
	Store      BlockStore
}

// NewProtonWriter creates a BlockWriter for a Proton Drive file.
func NewProtonWriter(linkID, revisionID string, sessionKey *crypto.SessionKey, store BlockStore) *ProtonWriter {
	return &ProtonWriter{
		LinkID:     linkID,
		RevisionID: revisionID,
		SessionKey: sessionKey,
		Store:      store,
	}
}

// WriteBlock encrypts and uploads a block.
func (w *ProtonWriter) WriteBlock(_ context.Context, _ int, _ []byte) error {
	// TODO: encrypt data using SessionKey, then upload via Store.
	return nil
}

// Describe returns the link ID.
func (w *ProtonWriter) Describe() string { return w.LinkID }

// Close is a no-op.
func (w *ProtonWriter) Close() error { return nil }
