package driveCmd

import (
	"context"
	"fmt"
	"strings"

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

	var dc *driveClient.Client
	if needSession {
		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		session, err := cli.RestoreSession(ctx)
		if err != nil {
			return err
		}

		dc, err = driveClient.NewClient(ctx, session)
		if err != nil {
			return err
		}
	}

	// Suppress unused variable warning until Task 3 wires up dispatch.
	_ = dc
	_ = sources
	_ = dest

	// TODO: dispatch to cpSingle/cpMultiple (Task 3)
	return fmt.Errorf("cp: not yet implemented")
}
