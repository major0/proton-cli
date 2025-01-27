package accountCmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ProtonMail/go-proton-api"
	"github.com/jedib0t/go-pretty/v6/table"
	cli "github.com/major0/proton-cli/cmd"
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
			addrType := getAddrType(addresses[i].Type)
			addrStatus := getAddrStatus(addresses[i].Status)
			t.AppendRow(table.Row{addr, addrType, addrStatus})
		}
		t.Render()
		return nil
	},
}

func getAddrType(addrType proton.AddressType) string {
	switch proton.AddressType(addrType) {
	case proton.AddressTypeOriginal:
		return "original"
	case proton.AddressTypeAlias:
		return "alias"
	case proton.AddressTypeCustom:
		return "custom"
	case proton.AddressTypePremium:
		return "premium"
	case proton.AddressTypeExternal:
		return "external"
	default:
		return fmt.Sprintf("Unknown (%d)", addrType)
	}
}

func getAddrStatus(addrStatus proton.AddressStatus) string {
	switch proton.AddressStatus(addrStatus) {
	case proton.AddressStatusDisabled:
		return "disabled"
	case proton.AddressStatusEnabled:
		return "enabled"
	case proton.AddressStatusDeleting:
		return "deleting"
	default:
		return fmt.Sprintf("Unknown (%d)", addrStatus)
	}
}

func init() {
	accountCmd.AddCommand(accountAddressCmd)
}
