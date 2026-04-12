package accountCmd

import (
	"context"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	common "github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/api/account"
	cli "github.com/major0/proton-cli/cmd"
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

		session, err := cli.RestoreSession(ctx)
		if err != nil {
			return err
		}

		acct := account.NewClient(session)
		addresses, err := acct.GetAddresses(ctx)
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
