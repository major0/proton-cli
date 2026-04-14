package client

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/major0/proton-cli/api/drive"
)

// UploadFile uploads a local file to Proton Drive using the block
// pipeline. Creates a file draft, builds a CopyJob with LocalReader
// and ProtonWriter, and feeds it through RunPipeline.
func (c *Client) UploadFile(ctx context.Context, share *drive.Share, parentLink *drive.Link, localPath string, opts TransferOpts) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: stat %s: %w", localPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("drive.UploadFile: %s: is a directory", localPath)
	}

	fileName := filepath.Base(localPath)

	fh, err := c.CreateFile(ctx, share, parentLink, fileName)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: %w", err)
	}

	store := NewBlockStore(c.Session, nil)
	job := CopyJob{
		Src: NewLocalReader(localPath, info.Size()),
		Dst: NewProtonWriter(fh.Link.LinkID(), fh.RevisionID, fh.SessionKey, store),
	}

	if err := RunPipeline(ctx, []CopyJob{job}, opts); err != nil {
		return fmt.Errorf("drive.UploadFile: %s: %w", fileName, err)
	}

	// TODO: after pipeline completes:
	// 1. Compute SHA-256 per encrypted block, SHA-1 of file content
	// 2. Sign block hash manifest with address keyring
	// 3. Commit revision via UpdateRevision with encrypted XAttr

	return nil
}

// detectMIMEType returns the MIME type for a file name based on extension.
// Returns "application/octet-stream" for unknown extensions.
func detectMIMEType(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return "application/octet-stream"
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}
