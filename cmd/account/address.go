package accountCmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	cli "github.com/major0/proton-cli/cmd"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var accountAddressCmd = &cobra.Command{
	Use:     "addresses",
	Aliases: []string{"address", "addr"},
	Short:   "report all email addresses associated with the account",
	Long:    `report all email addresses associated with the account`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
		if err != nil {
			return err
		}

		if session == nil {
			fmt.Println("Not logged in")
			return nil
		}

		session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
		session.AddDeauthHandler(common.NewDeauthHandler())

		addresses, err := session.Client.GetAddresses(ctx)
		if err != nil {
			return err
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Address", "Type", "State"})
		for i := range addresses {
			addr := addresses[i].Email
			addrType := common.AddressType(addresses[i].Type).String()
			addrStatus := common.AddressStatus(addresses[i].Status).String()
			t.AppendRow(table.Row{addr, addrType, addrStatus})
		}
		t.Render()
		return nil
	},
}

func init() {
	accountCmd.AddCommand(accountAddressCmd)
}
