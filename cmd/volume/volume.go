// Package volumeCmd implements the volume subcommands for proton-cli.
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
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
}

func init() {
	cli.AddCommand(volumeCmd)
}
