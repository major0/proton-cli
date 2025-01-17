package authCmd

import (
	"github.com/major0/protondrive-cli/cmd"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth [options] <command>",
	Short: "authenticate with ProtonDrive",
	Long:  `authenticate with ProtonDrive`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	pdcli.RootCmd.AddCommand(authCmd)
	authCmd.Flags().BoolP("help", "h", false, "Help for pdcli")
	authCmd.Flags().Lookup("help").Hidden = true
}
