package authCmd

import (
	"context"

	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "logout of ProtonDrive",
	Long:  `logout of ProtonDrive`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := cli.Client.AuthRevoke(ctx, cli.Config.UID)
		if err != nil {
			return err
		}
		return cli.PurgeConfig()
	},
}

func init() {
	authCmd.AddCommand(authLogoutCmd)
}
