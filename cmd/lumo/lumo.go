package lumoCmd

import (
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var lumoCmd = &cobra.Command{
	Use:   "lumo",
	Short: "Proton Lumo AI assistant",
	Long:  "Proton Lumo AI assistant",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if p := cmd.Root(); p != nil && p.PersistentPreRunE != nil {
			if err := p.PersistentPreRunE(p, args); err != nil {
				return err
			}
		}
		cli.SetService("lumo")
		return nil
	},
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
