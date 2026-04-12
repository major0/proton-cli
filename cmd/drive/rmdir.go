package driveCmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var rmdirFlags struct {
	verbose   bool
	permanent bool
}

var driveRmdirCmd = &cobra.Command{
	Use:   "rmdir [options] <path> [<path> ...]",
	Short: "Remove empty directories from Proton Drive",
	Long:  "Remove empty directories from Proton Drive (moves to trash by default)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRmdir,
}

func init() {
	driveCmd.AddCommand(driveRmdirCmd)
	cli.BoolFlagP(driveRmdirCmd.Flags(), &rmdirFlags.verbose, "verbose", "v", false, "Print each directory as it is removed")
	cli.BoolFlag(driveRmdirCmd.Flags(), &rmdirFlags.permanent, "permanent", false, "Permanently delete instead of moving to trash")
}

func runRmdir(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
	if err != nil {
		return err
	}

	session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
	session.AddDeauthHandler(common.NewDeauthHandler())

	for _, arg := range args {
		if err := rmdirOne(ctx, session, arg); err != nil {
			return err
		}
	}

	return nil
}

func rmdirOne(ctx context.Context, session *common.Session, rawPath string) error {
	if !strings.HasPrefix(rawPath, "proton://") {
		return fmt.Errorf("invalid path: %s (must start with proton://)", rawPath)
	}

	path := parsePath(rawPath)
	if path == "" {
		return fmt.Errorf("rmdir: missing directory name")
	}

	parts := strings.SplitN(path, "/", 2)
	shareName := parts[0]
	relPath := ""
	if len(parts) > 1 {
		relPath = parts[1]
	}

	share, err := session.ResolveShare(ctx, shareName, true)
	if err != nil {
		return fmt.Errorf("rmdir: %s: %w", shareName, err)
	}

	if relPath == "" {
		return fmt.Errorf("rmdir: cannot remove share root")
	}

	relPath = strings.TrimSuffix(relPath, "/")
	link, err := share.Link.ResolvePath(ctx, relPath, true)
	if err != nil {
		return fmt.Errorf("rmdir: %s: %w", relPath, err)
	}

	if link.Type() != proton.LinkTypeFolder {
		return fmt.Errorf("rmdir: %s: not a directory", relPath)
	}

	if rmdirFlags.permanent {
		err = session.RmDirPermanent(ctx, share, link, false)
	} else {
		err = session.RmDir(ctx, share, link, false)
	}

	if err != nil {
		return err
	}

	if rmdirFlags.verbose {
		action := "trashed"
		if rmdirFlags.permanent {
			action = "deleted"
		}
		name, _ := link.Name()
		fmt.Printf("rmdir: %s '%s'\n", action, name)
	}

	return nil
}
