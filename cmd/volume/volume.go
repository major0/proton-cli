package volumeCmd

import (
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var volumeCmd = &cobra.Command{
	Use:     "volume",
	Aliases: []string{"vol", "vols"},
	Short:   "Manage ProtonDrive volumes",
	Long:    "Manage ProtonDrive volumes",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	cli.RootCmd.AddCommand(volumeCmd)
}
