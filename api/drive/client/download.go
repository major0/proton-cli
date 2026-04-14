package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
)

// DownloadFile downloads a Proton Drive file to a local path using the
// block pipeline. Retrieves the revision block list, builds a CopyJob,
// and feeds it through the transferPipeline.
func (c *Client) DownloadFile(ctx context.Context, link *drive.Link, localPath string, opts TransferOpts) error {
	if link.Type() != proton.LinkTypeFile {
		return fmt.Errorf("drive.DownloadFile: %s: not a file", link.LinkID())
	}

	pLink := link.ProtonLink()
	if pLink.FileProperties == nil {
		return fmt.Errorf("drive.DownloadFile: %s: no file properties", link.LinkID())
	}

	shareID := link.Share().ProtonShare().ShareID
	revisionID := pLink.FileProperties.ActiveRevision.ID

	// Fetch the full revision with block list.
	revision, err := c.Session.Client.GetRevisionAllBlocks(ctx, shareID, link.LinkID(), revisionID)
	if err != nil {
		return fmt.Errorf("drive.DownloadFile: %s: get revision: %w", link.LinkID(), err)
	}

	// Derive the session key for decryption.
	nodeKR, err := link.KeyRing()
	if err != nil {
		return fmt.Errorf("drive.DownloadFile: %s: keyring: %w", link.LinkID(), err)
	}

	sessionKey, err := pLink.GetSessionKey(nodeKR)
	if err != nil {
		return fmt.Errorf("drive.DownloadFile: %s: session key: %w", link.LinkID(), err)
	}

	// Compute block sizes from revision XAttr if available.
	var blockSizes []int64
	addrKR, err := c.addrKRForLink(link)
	if err == nil {
		xattr, err := revision.GetDecXAttrString(addrKR, nodeKR)
		if err == nil && xattr != nil {
			blockSizes = xattr.BlockSizes
		}
	}

	// Pre-create the destination file.
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("drive.DownloadFile: create %s: %w", localPath, err)
	}
	f.Close()

	// Build the CopyJob.
	job := CopyJob{
		Src: CopyEndpoint{
			Type:       PathProton,
			Link:       link,
			Share:      link.Share(),
			RevisionID: revisionID,
			Blocks:     revision.Blocks,
			SessionKey: sessionKey,
			FileSize:   pLink.FileProperties.ActiveRevision.Size,
			BlockSizes: blockSizes,
		},
		Dst: CopyEndpoint{
			Type:      PathLocal,
			LocalPath: localPath,
		},
	}

	// If we don't have block sizes from XAttr, compute from file size.
	if len(job.Src.BlockSizes) == 0 {
		n := drive.BlockCount(job.Src.FileSize)
		job.Src.BlockSizes = make([]int64, n)
		remaining := job.Src.FileSize
		for i := range job.Src.BlockSizes {
			if remaining >= drive.BlockSize {
				job.Src.BlockSizes[i] = drive.BlockSize
			} else {
				job.Src.BlockSizes[i] = remaining
			}
			remaining -= job.Src.BlockSizes[i]
		}
	}

	store := NewBlockStore(c.Session, nil) // TODO: wire cache from share config
	pipe := &transferPipeline{
		workers: opts.workers(),
		store:   store,
		client:  c,
	}

	if err := pipe.run(ctx, []CopyJob{job}); err != nil {
		return fmt.Errorf("drive.DownloadFile: %s: %w", link.LinkID(), err)
	}

	// Preserve mtime from XAttr if available.
	if addrKR != nil {
		xattr, err := revision.GetDecXAttrString(addrKR, nodeKR)
		if err == nil && xattr != nil && xattr.ModificationTime != "" {
			if mt, err := time.Parse(time.RFC3339, xattr.ModificationTime); err == nil {
				_ = os.Chtimes(localPath, mt, mt)
			}
		}
	}

	return nil
}
