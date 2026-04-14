package driveCmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// resolveSource resolves a source path argument to a resolvedEndpoint.
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
		info, err := os.Stat(arg.raw)
		if err != nil {
			return nil, fmt.Errorf("cp: %s: %w", arg.raw, err)
		}
		ep.localPath = arg.raw
		ep.localInfo = info
	}
	return ep, nil
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

	// Build destination writer.
	switch dst.pathType {
	case driveClient.PathLocal:
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

	// Dispatch.
	if len(sources) == 1 {
		return cpSingle(ctx, dc, sources[0], dstEp)
	}
	return cpMultiple(ctx, dc, sources, dstEp)
}

// cpSingle copies a single source to the destination.
func cpSingle(ctx context.Context, dc *driveClient.Client, src pathArg, dst *resolvedEndpoint) error {
	srcEp, err := resolveSource(ctx, dc, src)
	if err != nil {
		return err
	}

	// If dest is a directory, copy source into it (preserve basename).
	if dst.isDir() {
		dst = &resolvedEndpoint{
			pathType:  dst.pathType,
			raw:       dst.raw,
			localPath: filepath.Join(dst.localPath, srcEp.basename()),
			localInfo: nil, // new file
			link:      dst.link,
			share:     dst.share,
		}
	}

	// Directory sources are handled by expandRecursive (Task 7).
	if srcEp.isDir() {
		return fmt.Errorf("cp: %s: is a directory (use -r to copy recursively)", srcEp.raw)
	}

	job, err := buildCopyJob(ctx, dc, srcEp, dst)
	if err != nil {
		return err
	}

	// All directions flow through the same pipeline.
	return driveClient.RunPipeline(ctx, []driveClient.CopyJob{*job}, transferOpts())
}

// cpMultiple copies multiple sources into a directory destination.
func cpMultiple(ctx context.Context, dc *driveClient.Client, srcs []pathArg, dst *resolvedEndpoint) error {
	for _, src := range srcs {
		if err := cpSingle(ctx, dc, src, dst); err != nil {
			return err
		}
	}
	return nil
}
