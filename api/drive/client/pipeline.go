package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/major0/proton-cli/api/drive"
)

// transferPipeline manages the three-stage pipeline:
// Job Worker → Reader Pool → Writer Pool.
type transferPipeline struct {
	workers int
	store   BlockStore
	client  *Client
}

// run executes the pipeline for the given jobs. Returns a joined error
// containing all per-job failures. Non-failing jobs complete even when
// others fail.
func (p *transferPipeline) run(ctx context.Context, jobs []CopyJob) error {
	if len(jobs) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channels connecting the three stages.
	jobQueue := make(chan *BlockJob, p.workers*2)
	blockCh := make(chan *BlockJob, p.workers*2)

	// Error collection.
	var errMu sync.Mutex
	var errs []error
	addErr := func(err error) {
		errMu.Lock()
		errs = append(errs, err)
		errMu.Unlock()
	}

	var wg sync.WaitGroup

	// Stage 3: Writer Pool (N goroutines).
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.writerLoop(ctx, blockCh, addErr)
		}()
	}

	// Stage 2: Reader Pool (N goroutines).
	var readerWg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		readerWg.Add(1)
		go func() {
			defer readerWg.Done()
			p.readerLoop(ctx, jobQueue, blockCh, addErr)
		}()
	}

	// Close blockCh when all readers are done.
	go func() {
		readerWg.Wait()
		close(blockCh)
	}()

	// Stage 1: Job Worker (single goroutine).
	// Expands CopyJobs into BlockJobs and feeds the jobQueue.
	go func() {
		defer close(jobQueue)
		for i := range jobs {
			blocks := expandBlocks(&jobs[i])
			for j := range blocks {
				select {
				case jobQueue <- &blocks[j]:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Wait for all writers to finish.
	wg.Wait()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// readerLoop is the main loop for a reader worker. It pulls BlockJob
// items from the job queue, reads the source block, and pushes the
// filled job into the block channel.
func (p *transferPipeline) readerLoop(ctx context.Context, jobQueue <-chan *BlockJob, blockCh chan<- *BlockJob, addErr func(error)) {
	buf := make([]byte, drive.BlockSize)

	var localFile *os.File
	var localPath string
	defer func() {
		if localFile != nil {
			localFile.Close()
		}
	}()

	for bj := range jobQueue {
		if ctx.Err() != nil {
			return
		}

		var n int
		var err error

		switch bj.Job.Src.Type {
		case PathLocal:
			n, err = p.readLocalBlock(bj, buf, &localFile, &localPath)
		case PathProton:
			n, err = p.readProtonBlock(ctx, bj, buf)
		}

		if err != nil {
			addErr(fmt.Errorf("read %s block %d: %w", srcDesc(bj.Job), bj.Index, err))
			continue
		}

		// Copy into the BlockJob's own buffer — the reader's buf will be
		// reused for the next block before the writer consumes this one.
		bj.Buf = make([]byte, n)
		copy(bj.Buf, buf[:n])

		select {
		case blockCh <- bj:
		case <-ctx.Done():
			return
		}
	}
}

// readLocalBlock reads a block from a local file at the correct offset.
// Reuses the file descriptor across blocks from the same file.
func (p *transferPipeline) readLocalBlock(bj *BlockJob, buf []byte, fp **os.File, currentPath *string) (int, error) {
	path := bj.Job.Src.LocalPath
	if *fp == nil || *currentPath != path {
		if *fp != nil {
			(*fp).Close()
		}
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		*fp = f
		*currentPath = path
	}

	n, err := (*fp).ReadAt(buf[:bj.Size], bj.Offset)
	if err != nil && err != io.EOF {
		return 0, err
	}
	return n, nil
}

// readProtonBlock fetches and decrypts a block from Proton Drive.
func (p *transferPipeline) readProtonBlock(ctx context.Context, bj *BlockJob, buf []byte) (int, error) {
	src := &bj.Job.Src
	if bj.Index-1 >= len(src.Blocks) {
		return 0, fmt.Errorf("block index %d out of range (have %d blocks)", bj.Index, len(src.Blocks))
	}
	pb := src.Blocks[bj.Index-1]

	linkID := ""
	if src.Link != nil {
		linkID = src.Link.LinkID()
	}

	encrypted, err := p.store.GetBlock(ctx, linkID, bj.Index, pb.BareURL, pb.Token)
	if err != nil {
		return 0, err
	}

	// TODO: decrypt encrypted block using src.SessionKey into buf.
	// For now, copy raw bytes (works for testing without real crypto).
	n := copy(buf, encrypted)
	return n, nil
}

// writerLoop is the main loop for a writer worker. It pulls filled
// BlockJob items from the block channel and writes to the destination.
func (p *transferPipeline) writerLoop(ctx context.Context, blockCh <-chan *BlockJob, addErr func(error)) {
	buf := make([]byte, drive.BlockSize)

	var localFile *os.File
	var localPath string
	defer func() {
		if localFile != nil {
			localFile.Close()
		}
	}()

	for bj := range blockCh {
		if ctx.Err() != nil {
			return
		}

		var err error

		switch bj.Job.Dst.Type {
		case PathLocal:
			err = p.writeLocalBlock(bj, &localFile, &localPath)
		case PathProton:
			err = p.writeProtonBlock(ctx, bj, buf)
		}

		// Zero the block data after consumption.
		clear(bj.Buf)

		if err != nil {
			addErr(fmt.Errorf("write %s block %d: %w", dstDesc(bj.Job), bj.Index, err))
		}
	}

	// Zero the writer's encrypt buffer.
	clear(buf)
}

// writeLocalBlock writes a decrypted block to a local file at the
// correct offset. Opens the file without O_TRUNC — workers write
// blocks out of order into distinct file regions.
func (p *transferPipeline) writeLocalBlock(bj *BlockJob, fp **os.File, currentPath *string) error {
	path := bj.Job.Dst.LocalPath
	if *fp == nil || *currentPath != path {
		if *fp != nil {
			(*fp).Close()
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		*fp = f
		*currentPath = path
	}

	_, err := (*fp).WriteAt(bj.Buf, bj.Offset)
	return err
}

// writeProtonBlock encrypts and uploads a block to Proton Drive.
func (p *transferPipeline) writeProtonBlock(ctx context.Context, bj *BlockJob, buf []byte) error {
	// TODO: encrypt bj.Buf using dst.SessionKey into buf.
	// For now, copy raw bytes (works for testing without real crypto).
	n := copy(buf, bj.Buf)
	encrypted := buf[:n]

	linkID := ""
	if bj.Job.Dst.Link != nil {
		linkID = bj.Job.Dst.Link.LinkID()
	}

	// TODO: need upload URL from RequestUpload — this will be wired
	// in UploadFile which calls RequestUpload before starting the pipeline.
	_ = linkID
	_ = encrypted
	return nil
}

// srcDesc returns a human-readable description of the source for error messages.
func srcDesc(job *CopyJob) string {
	if job.Src.Type == PathLocal {
		return job.Src.LocalPath
	}
	if job.Src.Link != nil {
		return job.Src.Link.LinkID()
	}
	return "<proton>"
}

// dstDesc returns a human-readable description of the destination for error messages.
func dstDesc(job *CopyJob) string {
	if job.Dst.Type == PathLocal {
		return job.Dst.LocalPath
	}
	if job.Dst.Link != nil {
		return job.Dst.Link.LinkID()
	}
	return "<proton>"
}
