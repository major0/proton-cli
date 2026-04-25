package lumoCmd

import (
	"fmt"

	lumoClient "github.com/major0/proton-cli/api/lumo/client"
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

// restoreClient restores the session and creates a Lumo client.
func restoreClient(cmd *cobra.Command) (*lumoClient.Client, error) {
	session, err := cli.RestoreSession(cmd.Context())
	if err != nil {
		return nil, fmt.Errorf("no active session (run 'proton account login' first): %w", err)
	}
	return lumoClient.NewClient(session), nil
}

// AddCommand registers a subcommand under the lumo command group.
func AddCommand(cmd *cobra.Command) {
	lumoCmd.AddCommand(cmd)
}
