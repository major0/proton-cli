package shareCmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	driveClient "github.com/major0/proton-cli/api/drive/client"
	"github.com/major0/proton-cli/api/share"
	shareClient "github.com/major0/proton-cli/api/share/client"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

func init() {
	shareRevokeCmd.RunE = runShareRevoke
}

// revokeTarget identifies a matched entity for revocation.
type revokeTarget struct {
	kind string // "member", "invitation", "external-invitation"
	id   string // MemberID, InvitationID, or ExternalInvitationID
	desc string // human-readable description for ambiguity errors
}

// findRevokeTarget searches members, invitations, and external invitations
// for a match by email or ID. Returns the single match, or an error if
// zero or multiple matches are found.
func findRevokeTarget(arg string, members []share.Member, invs []share.Invitation, exts []share.ExternalInvitation) (revokeTarget, error) {
	var matches []revokeTarget

	for _, m := range members {
		if m.Email == arg || m.MemberID == arg {
			matches = append(matches, revokeTarget{
				kind: "member",
				id:   m.MemberID,
				desc: fmt.Sprintf("member %s (%s)", m.Email, m.MemberID),
			})
		}
	}

	for _, inv := range invs {
		if inv.InviteeEmail == arg || inv.InvitationID == arg {
			matches = append(matches, revokeTarget{
				kind: "invitation",
				id:   inv.InvitationID,
				desc: fmt.Sprintf("invitation %s (%s)", inv.InviteeEmail, inv.InvitationID),
			})
		}
	}

	for _, ext := range exts {
		if ext.InviteeEmail == arg || ext.ExternalInvitationID == arg {
			matches = append(matches, revokeTarget{
				kind: "external-invitation",
				id:   ext.ExternalInvitationID,
				desc: fmt.Sprintf("external invitation %s (%s)", ext.InviteeEmail, ext.ExternalInvitationID),
			})
		}
	}

	switch len(matches) {
	case 0:
		return revokeTarget{}, fmt.Errorf("no matching member or invitation found")
	case 1:
		return matches[0], nil
	default:
		var descs []string
		for _, m := range matches {
			descs = append(descs, m.desc)
		}
		return revokeTarget{}, fmt.Errorf("ambiguous match — use a specific ID instead:\n  %s", strings.Join(descs, "\n  "))
	}
}

func runShareRevoke(_ *cobra.Command, args []string) error {
	shareName := args[0]
	target := args[1]

	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := cli.RestoreSession(ctx)
	if err != nil {
		return err
	}

	dc, err := driveClient.NewClient(ctx, session)
	if err != nil {
		return err
	}

	resolved, err := dc.ResolveShare(ctx, shareName, true)
	if err != nil {
		return fmt.Errorf("share revoke: %s: share not found", shareName)
	}

	// Main and photos shares don't support member management.
	meta := resolved.Metadata()
	if meta.Type == proton.ShareTypeMain || meta.Type == drive.ShareTypePhotos {
		return fmt.Errorf("share revoke: %s: cannot revoke from %s share", shareName, drive.FormatShareType(meta.Type))
	}

	sc := shareClient.NewClient(session)
	shareID := meta.ShareID

	members, err := sc.ListMembers(ctx, shareID)
	if err != nil {
		return fmt.Errorf("share revoke: listing members: %w", err)
	}

	invs, err := sc.ListInvitations(ctx, shareID)
	if err != nil {
		return fmt.Errorf("share revoke: listing invitations: %w", err)
	}

	exts, err := sc.ListExternalInvitations(ctx, shareID)
	if err != nil {
		return fmt.Errorf("share revoke: listing external invitations: %w", err)
	}

	match, err := findRevokeTarget(target, members, invs, exts)
	if err != nil {
		return fmt.Errorf("share revoke: %s: %w", target, err)
	}

	switch match.kind {
	case "member":
		if err := sc.RemoveMember(ctx, shareID, match.id); err != nil {
			return fmt.Errorf("share revoke: %w", err)
		}
		fmt.Printf("Removed member %s\n", target)
	case "invitation":
		if err := sc.DeleteInvitation(ctx, shareID, match.id); err != nil {
			return fmt.Errorf("share revoke: %w", err)
		}
		fmt.Printf("Cancelled invitation for %s\n", target)
	case "external-invitation":
		if err := sc.DeleteExternalInvitation(ctx, shareID, match.id); err != nil {
			return fmt.Errorf("share revoke: %w", err)
		}
		fmt.Printf("Cancelled external invitation for %s\n", target)
	}

	return nil
}
