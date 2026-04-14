package client

import (
	"context"
	"fmt"

	"github.com/major0/proton-cli/api/drive"
)

// ListMembers returns all members of a share.
func (c *Client) ListMembers(ctx context.Context, shareID string) ([]drive.Member, error) {
	path := fmt.Sprintf("/drive/v2/shares/%s/members", shareID)
	var resp drive.MembersResponse
	if err := c.Session.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("drive.ListMembers %s: %w", shareID, err)
	}
	return resp.Members, nil
}

// RemoveMember removes a member from a share.
func (c *Client) RemoveMember(ctx context.Context, shareID, memberID string) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/members/%s", shareID, memberID)
	if err := c.Session.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("drive.RemoveMember %s/%s: %w", shareID, memberID, err)
	}
	return nil
}

// ListInvitations returns all pending Proton-user invitations for a share.
func (c *Client) ListInvitations(ctx context.Context, shareID string) ([]drive.Invitation, error) {
	path := fmt.Sprintf("/drive/v2/shares/%s/invitations", shareID)
	var resp drive.InvitationsResponse
	if err := c.Session.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("drive.ListInvitations %s: %w", shareID, err)
	}
	return resp.Invitations, nil
}

// InviteProtonUser sends an invitation to a Proton user.
func (c *Client) InviteProtonUser(ctx context.Context, shareID string, payload drive.InviteProtonUserPayload) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/invitations", shareID)
	if err := c.Session.DoJSON(ctx, "POST", path, payload, nil); err != nil {
		return fmt.Errorf("drive.InviteProtonUser %s: %w", shareID, err)
	}
	return nil
}

// DeleteInvitation cancels a pending Proton-user invitation.
func (c *Client) DeleteInvitation(ctx context.Context, shareID, invitationID string) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/invitations/%s", shareID, invitationID)
	if err := c.Session.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("drive.DeleteInvitation %s/%s: %w", shareID, invitationID, err)
	}
	return nil
}

// ListExternalInvitations returns all pending external invitations for a share.
func (c *Client) ListExternalInvitations(ctx context.Context, shareID string) ([]drive.ExternalInvitation, error) {
	path := fmt.Sprintf("/drive/v2/shares/%s/external-invitations", shareID)
	var resp drive.ExternalInvitationsResponse
	if err := c.Session.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("drive.ListExternalInvitations %s: %w", shareID, err)
	}
	return resp.ExternalInvitations, nil
}

// InviteExternalUser sends an invitation to a non-Proton email.
func (c *Client) InviteExternalUser(ctx context.Context, shareID string, payload drive.InviteExternalUserPayload) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/external-invitations", shareID)
	if err := c.Session.DoJSON(ctx, "POST", path, payload, nil); err != nil {
		return fmt.Errorf("drive.InviteExternalUser %s: %w", shareID, err)
	}
	return nil
}

// DeleteExternalInvitation cancels a pending external invitation.
func (c *Client) DeleteExternalInvitation(ctx context.Context, shareID, externalInvitationID string) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/external-invitations/%s", shareID, externalInvitationID)
	if err := c.Session.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("drive.DeleteExternalInvitation %s/%s: %w", shareID, externalInvitationID, err)
	}
	return nil
}

// CreateShareFromLink creates a new share via POST /drive/volumes/{volumeID}/shares.
// Returns the new share ID.
func (c *Client) CreateShareFromLink(ctx context.Context, volumeID string, payload drive.CreateDriveSharePayload) (string, error) {
	path := fmt.Sprintf("/drive/volumes/%s/shares", volumeID)
	var resp drive.CreateShareResponse
	if err := c.Session.DoJSON(ctx, "POST", path, payload, &resp); err != nil {
		return "", fmt.Errorf("drive.CreateShare %s: %w", volumeID, err)
	}
	return resp.Share.ID, nil
}

// DeleteShareByID deletes a share via DELETE /drive/shares/{shareID}.
// When force is true, passes Force=1 to delete even if members exist.
func (c *Client) DeleteShareByID(ctx context.Context, shareID string, force bool) error {
	forceVal := "0"
	if force {
		forceVal = "1"
	}
	path := fmt.Sprintf("/drive/shares/%s?Force=%s", shareID, forceVal)
	if err := c.Session.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("drive.DeleteShare %s: %w", shareID, err)
	}
	return nil
}
