// Package client provides the share management API client. It wraps
// api.Session.DoJSON to call Proton Drive sharing endpoints that are
// not covered by go-proton-api.
package client

import (
	"context"
	"fmt"

	"github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/api/share"
)

// Client wraps an api.Session for share management API calls.
type Client struct {
	Session *api.Session
}

// NewClient constructs a share client from an existing session.
func NewClient(session *api.Session) *Client {
	return &Client{Session: session}
}

// ListMembers returns all members of a share.
func (c *Client) ListMembers(ctx context.Context, shareID string) ([]share.Member, error) {
	path := fmt.Sprintf("/drive/shares/%s/members", shareID)
	var resp share.MembersResponse
	if err := c.Session.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("share.ListMembers %s: %w", shareID, err)
	}
	return resp.Members, nil
}

// RemoveMember removes a member from a share.
func (c *Client) RemoveMember(ctx context.Context, shareID, memberID string) error {
	path := fmt.Sprintf("/drive/shares/%s/members/%s", shareID, memberID)
	if err := c.Session.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("share.RemoveMember %s/%s: %w", shareID, memberID, err)
	}
	return nil
}

// ListInvitations returns all pending Proton-user invitations for a share.
func (c *Client) ListInvitations(ctx context.Context, shareID string) ([]share.Invitation, error) {
	path := fmt.Sprintf("/drive/v2/shares/%s/invitations", shareID)
	var resp share.InvitationsResponse
	if err := c.Session.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("share.ListInvitations %s: %w", shareID, err)
	}
	return resp.Invitations, nil
}

// InviteProtonUser sends an invitation to a Proton user.
func (c *Client) InviteProtonUser(ctx context.Context, shareID string, payload share.InviteProtonUserPayload) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/invitations", shareID)
	if err := c.Session.DoJSON(ctx, "POST", path, payload, nil); err != nil {
		return fmt.Errorf("share.InviteProtonUser %s: %w", shareID, err)
	}
	return nil
}

// DeleteInvitation cancels a pending Proton-user invitation.
func (c *Client) DeleteInvitation(ctx context.Context, shareID, invitationID string) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/invitations/%s", shareID, invitationID)
	if err := c.Session.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("share.DeleteInvitation %s/%s: %w", shareID, invitationID, err)
	}
	return nil
}

// ListExternalInvitations returns all pending external invitations for a share.
func (c *Client) ListExternalInvitations(ctx context.Context, shareID string) ([]share.ExternalInvitation, error) {
	path := fmt.Sprintf("/drive/v2/shares/%s/external-invitations", shareID)
	var resp share.ExternalInvitationsResponse
	if err := c.Session.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("share.ListExternalInvitations %s: %w", shareID, err)
	}
	return resp.ExternalInvitations, nil
}

// InviteExternalUser sends an invitation to a non-Proton email.
func (c *Client) InviteExternalUser(ctx context.Context, shareID string, payload share.InviteExternalUserPayload) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/external-invitations", shareID)
	if err := c.Session.DoJSON(ctx, "POST", path, payload, nil); err != nil {
		return fmt.Errorf("share.InviteExternalUser %s: %w", shareID, err)
	}
	return nil
}

// DeleteExternalInvitation cancels a pending external invitation.
func (c *Client) DeleteExternalInvitation(ctx context.Context, shareID, externalInvitationID string) error {
	path := fmt.Sprintf("/drive/v2/shares/%s/external-invitations/%s", shareID, externalInvitationID)
	if err := c.Session.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("share.DeleteExternalInvitation %s/%s: %w", shareID, externalInvitationID, err)
	}
	return nil
}
