// Package shareCmd implements the share subcommands for proton-cli.
package shareCmd

import (
	"fmt"

	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Manage Proton Drive shares",
	Long:  "Manage Proton Drive shares, invitations, and members",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
}

// notImplemented returns a RunE that prints a not-yet-implemented message.
func notImplemented(name string) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("%s: not yet implemented (requires sharing API additions to go-proton-api)", name)
	}
}

var shareShowCmd = &cobra.Command{
	Use:   "show <share-name>",
	Short: "Show detailed share information",
	Long:  "Show detailed information about a share including members and invitations",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented("share show"),
}

var shareInviteCmd = &cobra.Command{
	Use:   "invite <share-name> <email> [permissions]",
	Short: "Invite a user to a share",
	Long:  "Invite a Proton user or external email to a share",
	Args:  cobra.MinimumNArgs(2),
	RunE:  notImplemented("share invite"),
}

var shareRevokeCmd = &cobra.Command{
	Use:   "revoke <share-name> <email-or-member-id>",
	Short: "Revoke access to a share",
	Long:  "Remove a member or cancel an invitation for a share",
	Args:  cobra.ExactArgs(2),
	RunE:  notImplemented("share revoke"),
}

func init() {
	cli.AddCommand(shareCmd)
	shareCmd.AddCommand(shareShowCmd)
	shareCmd.AddCommand(shareInviteCmd)
	shareCmd.AddCommand(shareRevokeCmd)
}
