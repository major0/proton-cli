package shareCmd

import (
	"fmt"
	"os"

	"github.com/ProtonMail/go-proton-api"
	"github.com/jedib0t/go-pretty/v6/table"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var shareListCmd = &cobra.Command{
	Use:   "list",
	Short: "List shares",
	Long:  "List shares",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := cli.SessionRestore()
		if err != nil {
			return err
		}

		shares, err := session.Client.ListShares(cli.Ctx, true)
		if err != nil {
			return err
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Share ID", "Creator", "Type", "State", "Flags"})
		for _, s := range shares {
			t.AppendRow(table.Row{
				s.ShareID,
				s.Creator,
				getShareType(s.Type),
				getShareState(s.State),
				getShareFlags(s.Flags),
			})
		}
		t.Render()

		return nil
	},
}

func getShareType(shareType proton.ShareType) string {
	switch proton.ShareType(shareType) {
	case proton.ShareTypeMain:
		return "main"
	case proton.ShareTypeStandard:
		return "standard"
	case proton.ShareTypeDevice:
		return "device"
	default:
		return fmt.Sprintf("Unknown (%d)", shareType)
	}
}

func getShareState(state proton.ShareState) string {
	switch proton.ShareState(state) {
	case proton.ShareStateActive:
		return "active"
	case proton.ShareStateDeleted:
		return "deleted"
	default:
		return fmt.Sprintf("Unknown (%d)", state)
	}
}

func getShareFlags(flags proton.ShareFlags) string {
	switch proton.ShareFlags(flags) {
	case proton.NoFlags:
		return "none"
	case proton.PrimaryShare:
		return "primary"
	default:
		return fmt.Sprintf("Unknown (%d)", flags)
	}
}

func init() {
	shareCmd.AddCommand(shareListCmd)
}
