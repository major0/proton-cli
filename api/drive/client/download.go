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
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("drive.DownloadFile: create %s: %w", localPath, err)
	}
	f.Close()

	store := NewBlockStore(c.Session, nil)
	job := CopyJob{
		Src: NewProtonReader(link.LinkID(), fh.Blocks, fh.SessionKey, fh.FileSize, fh.BlockSizes, store),
		Dst: NewLocalWriter(localPath),
	}

	if err := RunPipeline(ctx, []CopyJob{job}, opts); err != nil {
		return fmt.Errorf("drive.DownloadFile: %s: %w", link.LinkID(), err)
	}

	// Preserve mtime from XAttr if available.
	addrKR, aErr := c.addrKRForLink(link)
	if aErr == nil {
		pLink := link.ProtonLink()
		shareID := link.Share().ProtonShare().ShareID
		revisionID := pLink.FileProperties.ActiveRevision.ID
		revision, rErr := c.Session.Client.GetRevisionAllBlocks(ctx, shareID, link.LinkID(), revisionID)
		if rErr == nil {
			nodeKR, kErr := link.KeyRing()
			if kErr == nil {
				xattr, xErr := revision.GetDecXAttrString(addrKR, nodeKR)
				if xErr == nil && xattr != nil && xattr.ModificationTime != "" {
					if mt, tErr := time.Parse(time.RFC3339, xattr.ModificationTime); tErr == nil {
						_ = os.Chtimes(localPath, mt, mt)
					}
				}
			}
		}
	}

	return nil
}
