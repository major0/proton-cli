package accountCmd

import (
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var authLogoutForce = false
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "logout of ProtonDrive",
	Long:  `logout of ProtonDrive`,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := cli.SessionRestore()
		if err != nil && !authLogoutForce {
			return err
		}

		return cli.SessionRevoke(session, authLogoutForce)
	},
}

func init() {
	accountCmd.AddCommand(authLogoutCmd)
	authLogoutCmd.Flags().BoolVarP(&authLogoutForce, "force", "f", false, "Force logout of ProtonDrive")
}
