package authCmd

import (
	"context"

	"github.com/major0/protondrive-cli/cmd"
	"github.com/spf13/cobra"
)

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "logout of ProtonDrive",
	Long:  `logout of ProtonDrive`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := pdcli.Client.AuthRevoke(ctx, pdcli.Config.UID)
		if err != nil {
			return err
		}
		return pdcli.PurgeConfig()
	},
}

func init() {
	authCmd.AddCommand(authLogoutCmd)
}
