package shareCmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	common "github.com/major0/proton-cli/api"
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
	shareListCmd.Flags().BoolVarP(&shareListFlags.classify, "classify", "F", false, "Append indicator (/ for directories) to entries")
}

func runShareList(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
	if err != nil {
		return err
	}

	session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
	session.AddDeauthHandler(common.NewDeauthHandler())

	shares, err := session.ListShares(ctx, true)
	if err != nil {
		return err
	}

	useColor := resolveColor(shareListFlags.color)

	for i := range shares {
		name, _ := shares[i].GetName(ctx)
		meta := shares[i].Metadata()
		linkType := shares[i].Link.Type()

		display := formatName(name, linkType, useColor, shareListFlags.classify)

		fmt.Printf("%-8s %-30s %-12s %s\n",
			fmtShareType(meta.Type),
			meta.Creator,
			fmtTime(meta.CreationTime),
			display,
		)
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

func fmtShareType(st proton.ShareType) string {
	switch st {
	case proton.ShareTypeMain:
		return "main"
	case proton.ShareTypeStandard:
		return "shared"
	case proton.ShareTypeDevice:
		return "device"
	case 4:
		return "photos"
	default:
		return fmt.Sprintf("?(%d)", st)
	}
}

func fmtShareState(state proton.ShareState) string {
	switch state {
	case proton.ShareStateActive:
		return "active"
	case proton.ShareStateDeleted:
		return "deleted"
	default:
		return fmt.Sprintf("?(%d)", state)
	}
}

func fmtShareFlags(flags proton.ShareFlags) string {
	switch flags {
	case proton.NoFlags:
		return "-"
	case proton.PrimaryShare:
		return "primary"
	default:
		return fmt.Sprintf("?(%d)", flags)
	}
}

func fmtTime(epoch int64) string {
	if epoch == 0 {
		return "-"
	}
	return time.Unix(epoch, 0).Format("2006-01-02")
}
