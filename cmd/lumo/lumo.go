package lumoCmd

import (
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var lumoCmd = &cobra.Command{
	Use:   "lumo",
	Short: "Proton Lumo AI assistant",
	Long:  "Proton Lumo AI assistant",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
}

func init() {
	cli.AddCommand(lumoCmd)
}

// AddCommand registers a subcommand under the lumo command group.
func AddCommand(cmd *cobra.Command) {
	lumoCmd.AddCommand(cmd)
}
