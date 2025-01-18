package authCmd

import (
	"github.com/major0/protondrive-cli/cmd"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage user authentication with ProtonDrive",
	Long:  "Manage user authentication with ProtonDrive",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	pdcli.RootCmd.AddCommand(authCmd)
}
