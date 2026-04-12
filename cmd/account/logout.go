package accountCmd

import (
	"context"

	cli "github.com/major0/proton-cli/cmd"
	common "github.com/major0/proton-cli/proton"
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

		session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
		if err != nil && !authLogoutForce {
			return err
		}

		if session != nil {
			session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
			session.AddDeauthHandler(common.NewDeauthHandler())
		}

		return common.SessionRevoke(ctx, session, cli.SessionStoreVar, authLogoutForce)
	},
}

func init() {
	accountCmd.AddCommand(authLogoutCmd)
	authLogoutCmd.Flags().BoolVarP(&authLogoutForce, "force", "f", false, "Force logout of Proton")
}
