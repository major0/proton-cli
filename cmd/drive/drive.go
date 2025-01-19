package driveCmd

import (
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var driveCmd = &cobra.Command{
	Use:   "drive",
	Short: "Manage files/directories stored in ProtonDrive",
	Long:  "Manage files/directories stored in ProtonDrive",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	cli.AddCommand(driveCmd)
}
