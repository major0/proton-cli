// Package accountCmd implements the account subcommands for proton-cli.
package accountCmd

import (
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage user authentication with Proton",
	Long:  "Manage user authentication with Proton",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
}

func init() {
	cli.AddCommand(accountCmd)
}
