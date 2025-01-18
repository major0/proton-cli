package volumeCmd

import (
	"github.com/major0/protondrive-cli/cmd"
	"github.com/spf13/cobra"
)

var volumeCmd = &cobra.Command{
	Use:   "volume",
  Aliases: []string{"vol", "vols"},
	Short: "Manage ProtonDrive volumes",
	Long:  "Manage ProtonDrive volumes",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	pdcli.RootCmd.AddCommand(volumeCmd)
}
