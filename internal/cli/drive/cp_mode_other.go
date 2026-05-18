//go:build windows

package driveCmd

import "github.com/major0/proton-utils/api/drive"

// setProtonWriterMode is a no-op on Windows where Unix permission bits
// are not available from the filesystem.
func setProtonWriterMode(_ *drive.ProtonWriter, _ *resolvedEndpoint, _ cpOptions) {}
