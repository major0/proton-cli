package shareCmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ProtonMail/go-proton-api"
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var shareListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List shares",
	Long:    "List all Proton Drive shares visible to this account",
	RunE:    runShareList,
}

func init() {
	shareCmd.AddCommand(shareListCmd)
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

	fmt.Printf("%-20s %-8s %-8s %-8s %s %-12s\n",
		"Name", "Type", "State", "Flags", "Creator", "Created")

	for i := range shares {
		name, _ := shares[i].GetName(ctx)
		meta := shares[i].Metadata()

		fmt.Printf("%-20s %-8s %-8s %-8s %s %-12s\n",
			name,
			fmtShareType(meta.Type),
			fmtShareState(meta.State),
			fmtShareFlags(meta.Flags),
			meta.Creator,
			fmtTime(meta.CreationTime),
		)
	}

	return nil
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
