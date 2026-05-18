//go:build !windows

package driveCmd

import (
	"os"

	"github.com/major0/proton-utils/api/drive"
)

// setProtonWriterMode sets the Unix permission bits on a ProtonWriter
// when the source is a local file and --preserve=mode (or -a) is active.
// When preserve is not active, Mode stays 0 (omitted from XAttr JSON).
func setProtonWriterMode(pw *drive.ProtonWriter, src *resolvedEndpoint, opts cpOptions) {
	if src.pathType != PathLocal {
		return
	}
	preserve := parsePreserve(opts)
	if !preserve.mode {
		return
	}
	fi, err := os.Lstat(src.localPath)
	if err != nil {
		return
	}
	pw.SetMode(unixMode(fi.Mode()))
}

// unixMode converts a Go os.FileMode to a traditional Unix mode_t
// containing the lower 12 permission bits (setuid, setgid, sticky + rwx).
func unixMode(m os.FileMode) uint32 {
	mode := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		mode |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		mode |= 0o2000
	}
	if m&os.ModeSticky != 0 {
		mode |= 0o1000
	}
	return mode
}
