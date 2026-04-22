package client

import "context"

// BlockReader reads blocks from a source. Implementations carry their
// own state (file path, link, session key, etc.).
type BlockReader interface {
	// ReadBlock reads block at index (0-based) into buf. Returns bytes read.
	ReadBlock(ctx context.Context, index int, buf []byte) (int, error)
	// BlockCount returns the total number of blocks.
	BlockCount() int
	// BlockSize returns the size of block at index (0-based).
	BlockSize(index int) int64
	// TotalSize returns the total file size in bytes.
	TotalSize() int64
	// Describe returns a human-readable name for error messages.
	Describe() string
	// Close releases resources.
	Close() error
}

// BlockWriter writes blocks to a destination. Implementations carry
// their own state (file path, link, session key, etc.).
type BlockWriter interface {
	// WriteBlock writes data as block at index (0-based).
	WriteBlock(ctx context.Context, index int, data []byte) error
	// Describe returns a human-readable name for error messages.
	Describe() string
	// Close releases resources.
	Close() error
}
