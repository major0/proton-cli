package shareCmd

import (
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
	Use:     "share",
	Aliases: []string{"shares"},
	Short:   "Manage ProtonDrive shares",
	Long:    "Manage ProtonDrive shares",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	cli.AddCommand(shareCmd)
}
