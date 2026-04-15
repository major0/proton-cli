package driveCmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	driveClient "github.com/major0/proton-cli/api/drive/client"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var cpFlags struct {
	recursive   bool   // -r, -R, --recursive
	archive     bool   // -a (implies -r -d --preserve=mode,timestamps)
	dereference bool   // -L, --dereference (follow symlinks)
	noDeref     bool   // -d (skip symlinks; implied by -a)
	verbose     bool   // -v, --verbose
	progress    bool   // --progress
	preserve    string // --preserve=mode,timestamps
	workers     int    // --workers (override default 8)
	targetDir   string // -t, --target-directory
	removeDest  bool   // --remove-destination (trash Proton / remove local before copy)
	backup      bool   // --backup (local: rename to <name>~; Proton: no-op)
}

var driveCpCmd = &cobra.Command{
	Use:   "cp [options] <source> [<source> ...] <dest>",
	Short: "Copy files and directories",
	Long: `Copy files and directories between local filesystem and Proton Drive,
within Proton Drive, or locally. Supports all four directions:
local→local, local→remote, remote→local, remote→remote.

Proton Drive files are versioned by default — copying over an existing
file creates a new revision preserving the old content.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCp,
}

func init() {
	driveCmd.AddCommand(driveCpCmd)
	f := driveCpCmd.Flags()

	cli.BoolFlagP(f, &cpFlags.recursive, "recursive", "r", false, "Copy directories recursively")
	f.BoolVarP(&cpFlags.recursive, "Recursive", "R", false, "Copy directories recursively (alias for -r)")
	cli.BoolFlagP(f, &cpFlags.archive, "archive", "a", false, "Archive mode: -r -d --preserve=mode,timestamps")
	cli.BoolFlagP(f, &cpFlags.dereference, "dereference", "L", false, "Follow symbolic links")
	cli.BoolFlagP(f, &cpFlags.noDeref, "no-dereference", "d", false, "Skip symbolic links (default; explicit for -a)")
	cli.BoolFlagP(f, &cpFlags.verbose, "verbose", "v", false, "Print each file as it completes")
	cli.BoolFlag(f, &cpFlags.progress, "progress", false, "Show aggregate transfer progress")
	f.StringVar(&cpFlags.preserve, "preserve", "", "Preserve attributes: mode,timestamps")
	f.IntVar(&cpFlags.workers, "workers", 0, "Number of concurrent workers (default 8)")
	f.StringVarP(&cpFlags.targetDir, "target-directory", "t", "", "Copy all sources into this directory")
	cli.BoolFlag(f, &cpFlags.removeDest, "remove-destination", false, "Trash/remove destination before copy (disables versioning)")
	cli.BoolFlag(f, &cpFlags.backup, "backup", false, "Backup existing local files as <name>~")
}

// pathArg is a parsed command argument with its classified type.
type pathArg struct {
	raw      string
	pathType driveClient.PathType
}

// classifyPath returns PathProton if arg starts with "proton://", PathLocal otherwise.
func classifyPath(arg string) driveClient.PathType {
	if strings.HasPrefix(arg, "proton://") {
		return driveClient.PathProton
	}
	return driveClient.PathLocal
}

// resolvedEndpoint holds the result of resolving a source or destination path.
// Exactly one variant is populated based on pathType.
type resolvedEndpoint struct {
	pathType driveClient.PathType
	raw      string // original argument string

	// Local path resolution (pathType == PathLocal)
	localPath string      // cleaned absolute path
	localInfo os.FileInfo // from os.Stat

	// Proton path resolution (pathType == PathProton)
	link  *drive.Link
	share *drive.Share
}

// isDir returns true if the resolved endpoint is a directory.
func (r *resolvedEndpoint) isDir() bool {
	if r.pathType == driveClient.PathLocal {
		return r.localInfo != nil && r.localInfo.IsDir()
	}
	return r.link != nil && r.link.Type() == proton.LinkTypeFolder
}

// basename returns the name of the resolved endpoint.
func (r *resolvedEndpoint) basename() string {
	if r.pathType == driveClient.PathLocal {
		return filepath.Base(r.localPath)
	}
	if r.link != nil {
		name, err := r.link.Name()
		if err != nil {
			return filepath.Base(r.raw)
		}
		return name
	}
	return filepath.Base(r.raw)
}

// resolveDest resolves the destination path with coreutils cp semantics.
// For existing paths, returns the resolved endpoint directly. For
// non-existent paths, verifies the parent exists and returns an endpoint
// with the parent info (localPath set but localInfo nil for local;
// link pointing to parent for Proton).
func resolveDest(ctx context.Context, dc *driveClient.Client, arg pathArg, multiSource bool) (*resolvedEndpoint, error) {
	ep := &resolvedEndpoint{pathType: arg.pathType, raw: arg.raw}

	switch arg.pathType {
	case driveClient.PathLocal:
		info, err := os.Stat(arg.raw)
		if err == nil {
			// Dest exists.
			ep.localPath = arg.raw
			ep.localInfo = info
			if multiSource && !info.IsDir() {
				return nil, fmt.Errorf("cp: %s: not a directory", arg.raw)
			}
			return ep, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("cp: %s: %w", arg.raw, err)
		}
		// Dest doesn't exist — parent must exist.
		if multiSource {
			return nil, fmt.Errorf("cp: %s: no such file or directory", arg.raw)
		}
		parent := filepath.Dir(arg.raw)
		pInfo, pErr := os.Stat(parent)
		if pErr != nil {
			return nil, fmt.Errorf("cp: %s: %w", arg.raw, pErr)
		}
		if !pInfo.IsDir() {
			return nil, fmt.Errorf("cp: %s: not a directory", parent)
		}
		// localPath set but localInfo nil signals "create new file at this path".
		ep.localPath = arg.raw
		return ep, nil

	case driveClient.PathProton:
		link, share, err := ResolveProtonPath(ctx, dc, arg.raw)
		if err == nil {
			// Dest exists.
			ep.link = link
			ep.share = share
			if multiSource && link.Type() != proton.LinkTypeFolder {
				return nil, fmt.Errorf("cp: %s: not a directory", arg.raw)
			}
			return ep, nil
		}
		// Dest doesn't exist — resolve parent.
		if multiSource {
			return nil, fmt.Errorf("cp: %s: no such file or directory", arg.raw)
		}
		parsed := parsePath(arg.raw)
		parentPath := filepath.Dir(parsed)
		parentURI := "proton:///" + parentPath
		parentLink, parentShare, pErr := ResolveProtonPath(ctx, dc, parentURI)
		if pErr != nil {
			return nil, fmt.Errorf("cp: %s: %w", arg.raw, pErr)
		}
		if parentLink.Type() != proton.LinkTypeFolder {
			return nil, fmt.Errorf("cp: %s: not a directory", parentPath)
		}
		ep.link = parentLink
		ep.share = parentShare
		return ep, nil
	}

	return ep, nil
}

// errSkipSymlink signals that a symlink source should be skipped.
var errSkipSymlink = fmt.Errorf("skipping symbolic link")

// resolveSource resolves a source path argument to a resolvedEndpoint.
// For local paths, uses os.Lstat to detect symlinks. With -L, follows
// symlinks via os.Stat. Without -L, returns errSkipSymlink.
func resolveSource(ctx context.Context, dc *driveClient.Client, arg pathArg) (*resolvedEndpoint, error) {
	ep := &resolvedEndpoint{pathType: arg.pathType, raw: arg.raw}
	switch arg.pathType {
	case driveClient.PathProton:
		link, share, err := ResolveProtonPath(ctx, dc, arg.raw)
		if err != nil {
			return nil, fmt.Errorf("cp: %s: %w", arg.raw, err)
		}
		ep.link = link
		ep.share = share
	case driveClient.PathLocal:
		info, err := os.Lstat(arg.raw)
		if err != nil {
			return nil, fmt.Errorf("cp: %s: %w", arg.raw, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if !cpFlags.dereference {
				return nil, fmt.Errorf("cp: %s: %w", arg.raw, errSkipSymlink)
			}
			// -L: follow the symlink.
			info, err = os.Stat(arg.raw)
			if err != nil {
				return nil, fmt.Errorf("cp: %s: %w", arg.raw, err)
			}
		}
		ep.localPath = arg.raw
		ep.localInfo = info
	}
	return ep, nil
}

// handleConflict handles destination file conflicts before copy.
//
// Default: local files are truncated (overwrite in place); Proton files
// get a new revision (versioning — handled by CreateFile, no action here).
// Directories are never removed — contents merge naturally.
//
// --remove-destination: local files are removed; Proton files are trashed
// (disables versioning). Directories are skipped.
//
// --backup: local files are renamed to <name>~; Proton files are a no-op
// (versioning is the default backup mechanism).
func handleConflict(ctx context.Context, dc *driveClient.Client, dst *resolvedEndpoint) error {
	// Directories: never remove, merge contents.
	if dst.isDir() {
		return nil
	}

	switch dst.pathType {
	case driveClient.PathLocal:
		// No existing file → nothing to do.
		if dst.localInfo == nil {
			return nil
		}
		if cpFlags.backup {
			return os.Rename(dst.localPath, dst.localPath+"~")
		}
		if cpFlags.removeDest {
			return os.Remove(dst.localPath)
		}
		// Default: truncate existing file so overwrite is clean.
		return os.Truncate(dst.localPath, 0)

	case driveClient.PathProton:
		// Non-existent Proton dest (link points to parent) → nothing to do.
		if dst.link == nil {
			return nil
		}
		// Only act on files, not directories.
		if dst.link.Type() != proton.LinkTypeFolder {
			if cpFlags.removeDest {
				return dc.Remove(ctx, dst.share, dst.link, drive.RemoveOpts{})
			}
			// Default and --backup: Proton versioning handles it.
		}
	}
	return nil
}

// expandRecursive walks a source directory and returns CopyJobs for all
// files. Destination subdirectories are created as encountered (breadth-
// first for Proton sources, natural walk order for local). Directories
// never become CopyJobs — only files with block data do.
func expandRecursive(ctx context.Context, dc *driveClient.Client, src, dstBase *resolvedEndpoint) ([]driveClient.CopyJob, []preserveEntry, error) {
	switch src.pathType {
	case driveClient.PathLocal:
		return expandLocalRecursive(ctx, dc, src, dstBase)
	case driveClient.PathProton:
		return expandProtonRecursive(ctx, dc, src, dstBase)
	}
	return nil, nil, nil
}

// expandLocalRecursive walks a local source directory tree.
func expandLocalRecursive(ctx context.Context, dc *driveClient.Client, src, dstBase *resolvedEndpoint) ([]driveClient.CopyJob, []preserveEntry, error) {
	var jobs []driveClient.CopyJob
	var preserves []preserveEntry
	srcRoot := src.localPath

	// Create the top-level dest directory.
	switch dstBase.pathType {
	case driveClient.PathLocal:
		if err := os.MkdirAll(dstBase.localPath, 0700); err != nil {
			return nil, nil, fmt.Errorf("cp: mkdir %s: %w", dstBase.localPath, err)
		}
	case driveClient.PathProton:
		if _, err := dc.MkDirAll(ctx, dstBase.share, dstBase.link, filepath.Base(dstBase.raw)); err != nil {
			return nil, nil, fmt.Errorf("cp: mkdir %s: %w", dstBase.raw, err)
		}
	}

	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			fmt.Fprintf(os.Stderr, "cp: %s: %v\n", path, walkErr)
			return nil // continue on error
		}

		// Compute relative path from source root.
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cp: %s: %v\n", path, err)
			return nil
		}
		if rel == "." {
			return nil // skip the root itself
		}

		// Symlink handling.
		if d.Type()&os.ModeSymlink != 0 {
			if !cpFlags.dereference {
				fmt.Fprintf(os.Stderr, "cp: %s: skipping symbolic link\n", path)
				return nil
			}
			// -L: follow the symlink — re-stat to get the target info.
			// WalkDir won't descend into symlinked dirs, so we only
			// handle symlinked files here.
		}

		if d.IsDir() {
			// Create dest subdirectory.
			switch dstBase.pathType {
			case driveClient.PathLocal:
				dstDir := filepath.Join(dstBase.localPath, rel)
				if err := os.MkdirAll(dstDir, 0700); err != nil {
					fmt.Fprintf(os.Stderr, "cp: mkdir %s: %v\n", dstDir, err)
				}
			case driveClient.PathProton:
				if _, err := dc.MkDirAll(ctx, dstBase.share, dstBase.link, rel); err != nil {
					fmt.Fprintf(os.Stderr, "cp: mkdir %s: %v\n", rel, err)
				}
			}
			return nil
		}

		// Regular file — build a CopyJob.
		info, err := d.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cp: %s: %v\n", path, err)
			return nil
		}

		fileSrc := &resolvedEndpoint{
			pathType:  driveClient.PathLocal,
			raw:       path,
			localPath: path,
			localInfo: info,
		}

		var fileDst *resolvedEndpoint
		switch dstBase.pathType {
		case driveClient.PathLocal:
			fileDst = &resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				raw:       filepath.Join(dstBase.localPath, rel),
				localPath: filepath.Join(dstBase.localPath, rel),
			}
		case driveClient.PathProton:
			fileDst = &resolvedEndpoint{
				pathType: driveClient.PathProton,
				raw:      rel,
				link:     dstBase.link,
				share:    dstBase.share,
			}
		}

		if err := handleConflict(ctx, dc, fileDst); err != nil {
			fmt.Fprintf(os.Stderr, "cp: %s: %v\n", path, err)
			return nil
		}

		job, err := buildCopyJob(ctx, dc, fileSrc, fileDst)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cp: %s: %v\n", path, err)
			return nil
		}
		jobs = append(jobs, *job)

		// Collect preservation metadata for local→local.
		if fileDst.pathType == driveClient.PathLocal {
			preserves = append(preserves, preserveEntry{
				dstPath: fileDst.localPath,
				mode:    info.Mode().Perm(),
				mtime:   info.ModTime(),
			})
		}

		return nil
	})

	if err != nil {
		return jobs, preserves, err
	}
	return jobs, preserves, nil
}

// expandProtonRecursive walks a Proton source directory tree using
// breadth-first TreeWalk.
func expandProtonRecursive(ctx context.Context, dc *driveClient.Client, src, dstBase *resolvedEndpoint) ([]driveClient.CopyJob, []preserveEntry, error) {
	var jobs []driveClient.CopyJob

	results := make(chan driveClient.WalkEntry, 64)
	var walkErr error
	go func() {
		defer close(results)
		walkErr = dc.TreeWalk(ctx, src.link, "", drive.BreadthFirst, results)
	}()

	for entry := range results {
		if ctx.Err() != nil {
			return jobs, nil, ctx.Err()
		}

		// Skip the root itself.
		if entry.Depth == 0 {
			continue
		}

		if entry.Link.Type() == proton.LinkTypeFolder {
			// Create dest subdirectory.
			switch dstBase.pathType {
			case driveClient.PathLocal:
				dstDir := filepath.Join(dstBase.localPath, entry.Path)
				if err := os.MkdirAll(dstDir, 0700); err != nil {
					fmt.Fprintf(os.Stderr, "cp: mkdir %s: %v\n", dstDir, err)
				}
			case driveClient.PathProton:
				if _, err := dc.MkDirAll(ctx, dstBase.share, dstBase.link, entry.Path); err != nil {
					fmt.Fprintf(os.Stderr, "cp: mkdir %s: %v\n", entry.Path, err)
				}
			}
			continue
		}

		// Regular file — build a CopyJob.
		fileSrc := &resolvedEndpoint{
			pathType: driveClient.PathProton,
			raw:      entry.Path,
			link:     entry.Link,
			share:    src.share,
		}

		var fileDst *resolvedEndpoint
		switch dstBase.pathType {
		case driveClient.PathLocal:
			fileDst = &resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				raw:       filepath.Join(dstBase.localPath, entry.Path),
				localPath: filepath.Join(dstBase.localPath, entry.Path),
			}
		case driveClient.PathProton:
			fileDst = &resolvedEndpoint{
				pathType: driveClient.PathProton,
				raw:      entry.Path,
				link:     dstBase.link,
				share:    dstBase.share,
			}
		}

		if err := handleConflict(ctx, dc, fileDst); err != nil {
			fmt.Fprintf(os.Stderr, "cp: %s: %v\n", entry.Path, err)
			continue
		}

		job, err := buildCopyJob(ctx, dc, fileSrc, fileDst)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cp: %s: %v\n", entry.Path, err)
			continue
		}
		jobs = append(jobs, *job)
	}

	if walkErr != nil {
		return jobs, nil, walkErr
	}
	return jobs, nil, nil
}

// buildCopyJob constructs a CopyJob from resolved source and destination
// endpoints. For Proton endpoints, uses CreateFile/OpenFile to get the
// FileHandle with revision, session key, and block info.
func buildCopyJob(ctx context.Context, dc *driveClient.Client, src, dst *resolvedEndpoint) (*driveClient.CopyJob, error) {
	var job driveClient.CopyJob

	// Build source reader.
	switch src.pathType {
	case driveClient.PathLocal:
		job.Src = driveClient.NewLocalReader(src.localPath, src.localInfo.Size())
	case driveClient.PathProton:
		fh, err := dc.OpenFile(ctx, src.link)
		if err != nil {
			return nil, fmt.Errorf("cp: %s: %w", src.raw, err)
		}
		store := driveClient.NewBlockStore(dc.Session, nil)
		job.Src = driveClient.NewProtonReader(fh.Link.LinkID(), fh.Blocks, fh.SessionKey, fh.FileSize, fh.BlockSizes, store)
	}

	// Build destination writer. Pre-create local files so workers can
	// write blocks at arbitrary offsets into an existing file.
	switch dst.pathType {
	case driveClient.PathLocal:
		f, err := os.Create(dst.localPath)
		if err != nil {
			return nil, fmt.Errorf("cp: %s: %w", dst.localPath, err)
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("cp: %s: %w", dst.localPath, err)
		}
		job.Dst = driveClient.NewLocalWriter(dst.localPath)
	case driveClient.PathProton:
		name := filepath.Base(dst.raw)
		if src.pathType == driveClient.PathLocal {
			name = filepath.Base(src.localPath)
		}
		fh, err := dc.CreateFile(ctx, dst.share, dst.link, name)
		if err != nil {
			return nil, fmt.Errorf("cp: %s: %w", dst.raw, err)
		}
		store := driveClient.NewBlockStore(dc.Session, nil)
		job.Dst = driveClient.NewProtonWriter(fh.Link.LinkID(), fh.RevisionID, fh.SessionKey, store)
	}

	return &job, nil
}

// transferOpts builds TransferOpts from the current flag values.
func transferOpts() driveClient.TransferOpts {
	opts := driveClient.TransferOpts{}
	if cpFlags.workers > 0 {
		opts.Workers = cpFlags.workers
	}
	// Progress and verbose callbacks wired in Task 11.
	return opts
}

func runCp(_ *cobra.Command, args []string) error {
	// Validate mutually exclusive flags.
	if cpFlags.removeDest && cpFlags.backup {
		return fmt.Errorf("cp: --remove-destination and --backup are mutually exclusive")
	}

	// Expand -a into its component flags.
	if cpFlags.archive {
		cpFlags.recursive = true
		cpFlags.noDeref = true
		if cpFlags.preserve == "" {
			cpFlags.preserve = "mode,timestamps"
		}
	}

	// Validate argument count.
	if cpFlags.targetDir == "" && len(args) < 2 {
		return fmt.Errorf("cp: missing destination operand after %q", args[0])
	}
	if cpFlags.targetDir != "" && len(args) < 1 {
		return fmt.Errorf("cp: missing source operand")
	}

	// Split args into sources and dest.
	var sources []pathArg
	var dest pathArg

	if cpFlags.targetDir != "" {
		// -t mode: all positional args are sources, -t value is dest.
		dest = pathArg{raw: cpFlags.targetDir, pathType: classifyPath(cpFlags.targetDir)}
		for _, a := range args {
			sources = append(sources, pathArg{raw: a, pathType: classifyPath(a)})
		}
	} else {
		// Default: last arg is dest, rest are sources.
		dest = pathArg{raw: args[len(args)-1], pathType: classifyPath(args[len(args)-1])}
		for _, a := range args[:len(args)-1] {
			sources = append(sources, pathArg{raw: a, pathType: classifyPath(a)})
		}
	}

	// Determine if any path is a Proton path — session setup is only
	// needed when at least one endpoint is remote.
	needSession := dest.pathType == driveClient.PathProton
	if !needSession {
		for _, s := range sources {
			if s.pathType == driveClient.PathProton {
				needSession = true
				break
			}
		}
	}

	// Create context — timeout applies to the entire operation.
	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	var dc *driveClient.Client
	if needSession {
		session, err := cli.RestoreSession(ctx)
		if err != nil {
			return err
		}

		dc, err = driveClient.NewClient(ctx, session)
		if err != nil {
			return err
		}
	}

	// Resolve destination.
	dstEp, err := resolveDest(ctx, dc, dest, len(sources) > 1)
	if err != nil {
		return err
	}

	// Build CopyJobs for all source/dest pairs.
	var jobs []driveClient.CopyJob
	var preserves []preserveEntry
	for _, src := range sources {
		srcEp, err := resolveSource(ctx, dc, src)
		if err != nil {
			if errors.Is(err, errSkipSymlink) {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				continue
			}
			return err
		}

		// Compute the effective destination for this source.
		fileDst := dstEp
		if dstEp.isDir() {
			fileDst = &resolvedEndpoint{
				pathType:  dstEp.pathType,
				raw:       dstEp.raw,
				localPath: filepath.Join(dstEp.localPath, srcEp.basename()),
				localInfo: nil,
				link:      dstEp.link,
				share:     dstEp.share,
			}
		}

		// Directory sources: expand recursively or skip.
		if srcEp.isDir() {
			if !cpFlags.recursive {
				fmt.Fprintf(os.Stderr, "cp: %s: is a directory (use -r to copy recursively)\n", srcEp.raw)
				continue
			}
			expanded, preserveExpanded, err := expandRecursive(ctx, dc, srcEp, fileDst)
			if err != nil {
				return err
			}
			jobs = append(jobs, expanded...)
			preserves = append(preserves, preserveExpanded...)
			continue
		}

		if err := handleConflict(ctx, dc, fileDst); err != nil {
			return err
		}

		job, err := buildCopyJob(ctx, dc, srcEp, fileDst)
		if err != nil {
			return err
		}
		jobs = append(jobs, *job)

		// Collect preservation metadata for local destinations.
		if fileDst.pathType == driveClient.PathLocal && srcEp.pathType == driveClient.PathLocal && srcEp.localInfo != nil {
			preserves = append(preserves, preserveEntry{
				dstPath: fileDst.localPath,
				mode:    srcEp.localInfo.Mode().Perm(),
				mtime:   srcEp.localInfo.ModTime(),
			})
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	if err := driveClient.RunPipeline(ctx, jobs, transferOpts()); err != nil {
		return err
	}

	// Apply preserved attributes after all blocks are written.
	return applyPreserve(preserves)
}

// preserveEntry tracks metadata to apply after copy completes.
type preserveEntry struct {
	dstPath string
	mode    os.FileMode
	mtime   time.Time
}

// applyPreserve applies preserved mode and mtime to destination files.
func applyPreserve(entries []preserveEntry) error {
	preserve := parsePreserve()
	if !preserve.mode && !preserve.timestamps {
		return nil
	}
	for _, e := range entries {
		if preserve.mode {
			if err := os.Chmod(e.dstPath, e.mode); err != nil {
				fmt.Fprintf(os.Stderr, "cp: preserve mode %s: %v\n", e.dstPath, err)
			}
		}
		if preserve.timestamps {
			if err := os.Chtimes(e.dstPath, e.mtime, e.mtime); err != nil {
				fmt.Fprintf(os.Stderr, "cp: preserve timestamps %s: %v\n", e.dstPath, err)
			}
		}
	}
	return nil
}

// preserveFlags holds parsed --preserve flag values.
type preserveFlags struct {
	mode       bool
	timestamps bool
}

// parsePreserve parses the --preserve flag value.
func parsePreserve() preserveFlags {
	var pf preserveFlags
	for _, s := range strings.Split(cpFlags.preserve, ",") {
		switch strings.TrimSpace(s) {
		case "mode":
			pf.mode = true
		case "timestamps":
			pf.timestamps = true
		}
	}
	return pf
}
