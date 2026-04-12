package accountCmd

import (
	"context"
	"errors"

	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var authLogoutForce = false
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout of Proton",
	Long:  `Logout of Proton`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		session, err := cli.RestoreSession(ctx)
		if err != nil && !errors.Is(err, common.ErrNotLoggedIn) && !authLogoutForce {
			return err
		}

		return common.SessionRevoke(ctx, session, cli.SessionStoreVar, authLogoutForce)
	},
}

func init() {
	accountCmd.AddCommand(authLogoutCmd)
	cli.BoolFlagP(authLogoutCmd.Flags(), &authLogoutForce, "force", "f", false, "Force logout of Proton")
}
