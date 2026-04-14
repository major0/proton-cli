package drive

import (
	"encoding/base64"

	"github.com/ProtonMail/go-proton-api"
)

// BlockSize is the standard Proton Drive block size (~4 MB).
const BlockSize = 4 * 1024 * 1024

// Block is a raw API block descriptor. No session or cache awareness.
type Block struct {
	Index   int
	BareURL string
	Token   string
	Hash    []byte
	Size    int64
}

// BlockFromProton converts a proton.Block to a drive.Block, decoding
// the base64 hash. Size is not available from proton.Block — set it
// from the revision XAttr BlockSizes if needed.
func BlockFromProton(pb proton.Block) (Block, error) {
	hash, err := base64.StdEncoding.DecodeString(pb.Hash)
	if err != nil {
		return Block{}, err
	}
	return Block{
		Index:   pb.Index,
		BareURL: pb.BareURL,
		Token:   pb.Token,
		Hash:    hash,
	}, nil
}

// BlockCount returns the number of blocks needed for a file of the given size.
func BlockCount(size int64) int {
	if size <= 0 {
		return 0
	}
	return int((size + BlockSize - 1) / BlockSize)
}
