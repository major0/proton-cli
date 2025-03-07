package accountCmd

import (
	"context"
	"fmt"

	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var accountInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "report account information",
	Long:  `report information about currently logged in user`,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := cli.SessionRestore()
		if err != nil {
			return err
		}

		if session == nil {
			fmt.Println("Not logged in")
			return nil
		}

		user, err := session.Client.GetUser(context.Background())
		if err != nil {
			return err
		}

		fmt.Println("ID: " + user.ID)
		fmt.Println("Display Name: " + user.DisplayName)
		fmt.Println("Username: " + user.Name)
		fmt.Println("Email: " + user.Email)

		return nil
	},
}

func init() {
	accountCmd.AddCommand(accountInfoCmd)
}
