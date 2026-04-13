package shareCmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	driveClient "github.com/major0/proton-cli/api/drive/client"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	colorReset    = "\033[0m"
	colorBoldBlue = "\033[1;34m"
)

var shareListFlags struct {
	color    string
	classify bool
	long     bool
	inode    bool
}

var shareListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List shares",
	Long:    "List all Proton Drive shares visible to this account",
	RunE:    runShareList,
}

func init() {
	shareCmd.AddCommand(shareListCmd)
	shareListCmd.Flags().StringVar(&shareListFlags.color, "color", "auto", "Colorize output: auto, always, never")
	cli.BoolFlagP(shareListCmd.Flags(), &shareListFlags.classify, "classify", "F", false, "Append indicator (/ for directories) to entries")
	cli.BoolFlagP(shareListCmd.Flags(), &shareListFlags.long, "long", "l", false, "Use long listing format")
	cli.BoolFlagP(shareListCmd.Flags(), &shareListFlags.inode, "inode", "i", false, "Show share ID")
}

func runShareList(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := cli.RestoreSession(ctx)
	if err != nil {
		return err
	}

	dc, err := driveClient.NewClient(ctx, session)
	if err != nil {
		return err
	}

	shares, err := dc.ListShares(ctx, true)
	if err != nil {
		return err
	}

	useColor := resolveColor(shareListFlags.color)

	for i := range shares {
		name, _ := shares[i].GetName(ctx)
		meta := shares[i].Metadata()
		linkType := shares[i].Link.Type()

		display := formatName(name, linkType, useColor, shareListFlags.classify)

		if shareListFlags.inode && shareListFlags.long {
			fmt.Printf("%-20s %-8s %-30s %-12s %s\n",
				meta.ShareID,
				drive.FormatShareType(meta.Type),
				meta.Creator,
				fmtTime(meta.CreationTime),
				display,
			)
		} else if shareListFlags.inode {
			fmt.Printf("%s %s\n", meta.ShareID, display)
		} else if shareListFlags.long {
			fmt.Printf("%-8s %-30s %-12s %s\n",
				drive.FormatShareType(meta.Type),
				meta.Creator,
				fmtTime(meta.CreationTime),
				display,
			)
		} else {
			fmt.Println(display)
		}
	}

	return nil
}

func resolveColor(flag string) bool {
	switch flag {
	case "always":
		return true
	case "never":
		return false
	default:
		return term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec
	}
}

func formatName(name string, lt proton.LinkType, useColor, classify bool) string {
	suffix := ""
	if classify && lt == proton.LinkTypeFolder {
		suffix = "/"
	}

	if useColor && lt == proton.LinkTypeFolder {
		return colorBoldBlue + name + colorReset + suffix
	}
	return name + suffix
}

func fmtTime(epoch int64) string {
	if epoch == 0 {
		return "-"
	}
	return time.Unix(epoch, 0).Format("2006-01-02")
}
