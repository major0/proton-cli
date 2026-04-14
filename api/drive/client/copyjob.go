package client

import (
	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api/drive"
)

// DefaultWorkers is the default number of reader/writer goroutines.
const DefaultWorkers = 8

// PathType distinguishes local filesystem paths from Proton Drive paths.
type PathType int

const (
	// PathLocal is a local filesystem path.
	PathLocal PathType = iota
	// PathProton is a Proton Drive path (proton:// URI).
	PathProton
)

// CopyEndpoint describes one side of a copy operation.
type CopyEndpoint struct {
	Type       PathType
	LocalPath  string             // set when Type == PathLocal
	Link       *drive.Link        // set when Type == PathProton
	Share      *drive.Share       // set when Type == PathProton
	RevisionID string             // Proton source: from GetRevisionAllBlocks; Proton dest: from CreateFile/CreateRevision
	Blocks     []proton.Block     // Proton source: block list from revision
	SessionKey *crypto.SessionKey // Proton source: for decrypt; Proton dest: for encrypt
	FileSize   int64              // total file size in bytes
	BlockSizes []int64            // per-block sizes from XAttr or computed from FileSize
}

// CopyJob is a fully resolved source/destination pair. All context
// needed for block transfer is resolved upfront — workers do no
// additional lookups during transfer.
type CopyJob struct {
	Src CopyEndpoint
	Dst CopyEndpoint
}

// BlockJob is a single block work item flowing through the
// reader→writer channel. Buf is a slice into the worker's owned
// buffer — not a separate allocation.
type BlockJob struct {
	Job    *CopyJob
	Index  int   // 1-based block index
	Offset int64 // byte offset in the file
	Size   int64 // block size in bytes
	Buf    []byte
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

// expandBlocks generates BlockJob items from a CopyJob. For Proton
// sources, block sizes come from the source endpoint's BlockSizes.
// For local sources, blocks are computed from FileSize / BlockSize.
// Indices are 1-based, offsets are cumulative.
func expandBlocks(job *CopyJob) []BlockJob {
	sizes := job.Src.BlockSizes
	if len(sizes) == 0 && job.Src.FileSize > 0 {
		// Compute block sizes from file size.
		n := drive.BlockCount(job.Src.FileSize)
		sizes = make([]int64, n)
		remaining := job.Src.FileSize
		for i := range sizes {
			if remaining >= drive.BlockSize {
				sizes[i] = drive.BlockSize
			} else {
				sizes[i] = remaining
			}
			remaining -= sizes[i]
		}
	}

	jobs := make([]BlockJob, len(sizes))
	var offset int64
	for i, sz := range sizes {
		jobs[i] = BlockJob{
			Job:    job,
			Index:  i + 1, // 1-based
			Offset: offset,
			Size:   sz,
		}
		offset += sz
	}
	return jobs
}
