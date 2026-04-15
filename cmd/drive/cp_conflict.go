package driveCmd

import (
	"context"
	"os"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	driveClient "github.com/major0/proton-cli/api/drive/client"
)

// handleConflict handles destination file conflicts before copy.
// Default: truncate local, version Proton. Directories merge.
func handleConflict(ctx context.Context, dc *driveClient.Client, dst *resolvedEndpoint, removeDest, backup bool) error {
	if dst.isDir() {
		return nil
	}

	switch dst.pathType {
	case PathLocal:
		if dst.localInfo == nil {
			return nil
		}
		if backup {
			return os.Rename(dst.localPath, dst.localPath+"~")
		}
		if removeDest {
			return os.Remove(dst.localPath)
		}
		return os.Truncate(dst.localPath, 0)

	case PathProton:
		if dst.link == nil {
			return nil
		}
		if dst.link.Type() != proton.LinkTypeFolder && removeDest {
			return dc.Remove(ctx, dst.share, dst.link, drive.RemoveOpts{})
		}
	}
	return nil
}
