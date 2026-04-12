package driveCmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	cli "github.com/major0/proton-cli/cmd"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var mkdirFlags struct {
	parents bool
	verbose bool
}

var driveMkdirCmd = &cobra.Command{
	Use:   "mkdir [options] <path> [<path> ...]",
	Short: "Create directories in Proton Drive",
	Long:  "Create directories in Proton Drive",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runMkdir,
}

func init() {
	driveCmd.AddCommand(driveMkdirCmd)
	driveMkdirCmd.Flags().BoolVarP(&mkdirFlags.parents, "parents", "p", false, "Create parent directories as needed")
	driveMkdirCmd.Flags().BoolVarP(&mkdirFlags.verbose, "verbose", "v", false, "Print each directory as it is created")
}

func runMkdir(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
	if err != nil {
		return err
	}

	session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
	session.AddDeauthHandler(common.NewDeauthHandler())

	for _, arg := range args {
		if err := mkdirOne(ctx, session, arg); err != nil {
			return err
		}
	}

	return nil
}

func mkdirOne(ctx context.Context, session *common.Session, rawPath string) error {
	if !strings.HasPrefix(rawPath, "proton://") {
		return fmt.Errorf("invalid path: %s (must start with proton://)", rawPath)
	}

	path := parsePath(rawPath)
	if path == "" {
		return fmt.Errorf("mkdir: missing directory name")
	}

	parts := strings.SplitN(path, "/", 2)
	shareName := parts[0]
	relPath := ""
	if len(parts) > 1 {
		relPath = parts[1]
	}

	share, err := session.ResolveShare(ctx, shareName, true)
	if err != nil {
		return fmt.Errorf("mkdir: %s: %w", shareName, err)
	}

	if relPath == "" {
		return fmt.Errorf("mkdir: cannot create share root")
	}

	if mkdirFlags.parents {
		return mkdirAllCmd(ctx, session, share, relPath)
	}

	return mkdirSingle(ctx, session, share, relPath)
}

func mkdirSingle(ctx context.Context, session *common.Session, share *common.Share, relPath string) error {
	relPath = strings.TrimSuffix(relPath, "/")
	dir := ""
	name := relPath
	if idx := strings.LastIndex(relPath, "/"); idx >= 0 {
		dir = relPath[:idx]
		name = relPath[idx+1:]
	}

	var parent *common.Link
	var err error
	if dir == "" {
		parent = share.Link
	} else {
		parent, err = share.Link.ResolvePath(ctx, dir, true)
		if err != nil {
			return fmt.Errorf("mkdir: %s: %w", dir, err)
		}
	}

	if parent.Type() != proton.LinkTypeFolder {
		return fmt.Errorf("mkdir: %s: not a directory", dir)
	}

	newDir, err := session.MkDir(ctx, share, parent, name)
	if err != nil {
		return err
	}

	if mkdirFlags.verbose {
		shareName, _ := share.GetName(ctx)
		fmt.Printf("mkdir: created directory '%s/%s'\n", shareName, relPath)
	}

	_ = newDir
	return nil
}

func mkdirAllCmd(ctx context.Context, session *common.Session, share *common.Share, relPath string) error {
	relPath = strings.TrimSuffix(relPath, "/")
	parts := strings.Split(relPath, "/")

	current := share.Link
	builtPath, _ := share.GetName(ctx)

	for _, name := range parts {
		if name == "" || name == "." {
			continue
		}

		child, err := current.Lookup(ctx, name)
		if err != nil {
			return err
		}

		if child != nil {
			if child.Type() != proton.LinkTypeFolder {
				return fmt.Errorf("mkdir: %s/%s: not a directory", builtPath, name)
			}
			current = child
			builtPath += "/" + name
			continue
		}

		newDir, err := session.MkDir(ctx, share, current, name)
		if err != nil {
			return err
		}

		if mkdirFlags.verbose {
			fmt.Printf("mkdir: created directory '%s/%s'\n", builtPath, name)
		}

		current = newDir
		builtPath += "/" + name
	}

	return nil
}
