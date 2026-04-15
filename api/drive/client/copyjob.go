package client

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/major0/proton-cli/api/drive"
)

// DefaultWorkers is the default number of concurrent block workers.
const DefaultWorkers = 8

// BlockReader reads blocks from a source. Implementations carry their
// own state (file path, link, session key, etc.).
type BlockReader interface {
	// ReadBlock reads block at index (0-based) into buf. Returns bytes read.
	ReadBlock(ctx context.Context, index int, buf []byte) (int, error)
	// BlockCount returns the total number of blocks.
	BlockCount() int
	// BlockSize returns the size of block at index (0-based).
	BlockSize(index int) int64
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

// CopyJob is a fully resolved source/destination pair.
type CopyJob struct {
	Src BlockReader
	Dst BlockWriter
}

// TransferOpts configures bulk transfer behavior.
type TransferOpts struct {
	Workers  int // reader/writer count; default DefaultWorkers (8)
	Progress func(completed, total int, bytes int64, rate float64)
	Verbose  func(src, dst string)
}

// workers returns the configured worker count, defaulting to DefaultWorkers.
func (o TransferOpts) workers() int {
	if o.Workers <= 0 {
		return DefaultWorkers
	}
	return o.Workers
}

// LocalReader reads blocks from a local file. Each ReadBlock call
// opens its own file descriptor so concurrent workers don't interfere.
type LocalReader struct {
	Path    string
	Size    int64
	nBlocks int
}

// NewLocalReader creates a BlockReader for a local file.
func NewLocalReader(path string, size int64) *LocalReader {
	return &LocalReader{
		Path:    path,
		Size:    size,
		nBlocks: drive.BlockCount(size),
	}
}

// ReadBlock opens the file, reads block at index into buf, and closes.
func (r *LocalReader) ReadBlock(_ context.Context, index int, buf []byte) (int, error) {
	f, err := os.Open(r.Path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	offset := int64(index) * drive.BlockSize
	sz := r.BlockSize(index)
	n, err := f.ReadAt(buf[:sz], offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	return n, nil
}

// BlockCount returns the total number of blocks.
func (r *LocalReader) BlockCount() int { return r.nBlocks }

// BlockSize returns the size of block at index.
func (r *LocalReader) BlockSize(index int) int64 {
	offset := int64(index) * drive.BlockSize
	remaining := r.Size - offset
	if remaining <= 0 {
		return 0
	}
	if remaining > drive.BlockSize {
		return drive.BlockSize
	}
	return remaining
}

// Describe returns the file path.
func (r *LocalReader) Describe() string { return r.Path }

// Close is a no-op — FDs are per-call.
func (r *LocalReader) Close() error { return nil }

// LocalWriter writes blocks to a local file. Each WriteBlock call
// opens its own file descriptor so concurrent workers don't interfere.
type LocalWriter struct {
	Path string
}

// NewLocalWriter creates a BlockWriter for a local file.
func NewLocalWriter(path string) *LocalWriter {
	return &LocalWriter{Path: path}
}

// WriteBlock opens the file, writes data at the correct offset, and closes.
func (w *LocalWriter) WriteBlock(_ context.Context, index int, data []byte) error {
	f, err := os.OpenFile(w.Path, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	offset := int64(index) * drive.BlockSize
	_, err = f.WriteAt(data, offset)
	return err
}

// Describe returns the file path.
func (w *LocalWriter) Describe() string { return w.Path }

// Close is a no-op — FDs are per-call.
func (w *LocalWriter) Close() error { return nil }

// blockMap tracks block assignment for a single CopyJob. Workers claim
// blocks sequentially via an advancing counter — no bitmap needed since
// blocks are never released or reordered.
type blockMap struct {
	job   *CopyJob
	total int
	next  int // next block to claim; caller holds pipeline mutex
}

// newBlockMap creates a blockMap for a CopyJob.
func newBlockMap(job *CopyJob) *blockMap {
	return &blockMap{job: job, total: job.Src.BlockCount()}
}

// claim returns the index of the next unclaimed block, or -1 if all
// blocks have been claimed. Caller must hold the pipeline mutex.
func (m *blockMap) claim() int {
	if m.next >= m.total {
		return -1
	}
	idx := m.next
	m.next++
	return idx
}
