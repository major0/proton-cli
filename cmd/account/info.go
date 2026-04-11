package accountCmd

import (
	"context"
	"fmt"

	cli "github.com/major0/proton-cli/cmd"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var accountInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "report account information",
	Long:  `report information about currently logged in user`,
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

		user, err := session.Client.GetUser(ctx)
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
