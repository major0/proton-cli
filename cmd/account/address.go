package accountCmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var accountAddressCmd = &cobra.Command{
	Use:     "addresses",
	Aliases: []string{"address", "addr"},
	Short:   "report all email addresses associated with the account",
	Long:    `report all email addresses associated with the account`,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := cli.SessionRestore()
		if err != nil {
			return err
		}

		if session == nil {
			fmt.Println("Not logged in")
			return nil
		}

		addresses, err := session.Client.GetAddresses(context.Background())
		if err != nil {
			return err
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Address", "Type", "State"})
		for i := range addresses {
			addr := addresses[i].Email
			addrType := proton.AddressType(addresses[i].Type).String()
			addrStatus := proton.AddressStatus(addresses[i].Status).String()
			t.AppendRow(table.Row{addr, addrType, addrStatus})
		}
		t.Render()
		return nil
	},
}

func init() {
	accountCmd.AddCommand(accountAddressCmd)
}
