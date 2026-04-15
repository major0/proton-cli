package client

import (
	"context"
	"fmt"
	"os"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
)

// DownloadFile downloads a Proton Drive file to a local path using the
// block pipeline. Retrieves the revision block list, builds a CopyJob
// with ProtonReader and LocalWriter, and feeds it through RunPipeline.
func (c *Client) DownloadFile(ctx context.Context, link *drive.Link, localPath string, opts TransferOpts) error {
	if link.Type() != proton.LinkTypeFile {
		return fmt.Errorf("drive.DownloadFile: %s: not a file", link.LinkID())
	}

	fh, err := c.OpenFile(ctx, link)
	if err != nil {
		return fmt.Errorf("drive.DownloadFile: %w", err)
	}

	// Pre-create the destination file.
	f, err := os.Create(localPath) //nolint:gosec // path from caller, pre-creation before pipeline
	if err != nil {
		return fmt.Errorf("drive.DownloadFile: create %s: %w", localPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("drive.DownloadFile: close %s: %w", localPath, err)
	}

	store := NewBlockStore(c.Session, nil)
	job := CopyJob{
		Src: NewProtonReader(link.LinkID(), fh.Blocks, fh.SessionKey, fh.FileSize, nil, store),
		Dst: NewLocalWriter(localPath),
	}

	if err := RunPipeline(ctx, []CopyJob{job}, opts); err != nil {
		return fmt.Errorf("drive.DownloadFile: %s: %w", link.LinkID(), err)
	}

	// Preserve mtime from FileHandle (populated by OpenFile from XAttr).
	if !fh.ModTime.IsZero() {
		_ = os.Chtimes(localPath, fh.ModTime, fh.ModTime)
	}

	return nil
}
