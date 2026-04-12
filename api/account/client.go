// Package account provides Proton Account-specific types and operations.
package account

import (
	"context"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api"
)

// Client wraps an api.Session with Account-specific operations.
type Client struct {
	Session *api.Session
}

// NewClient constructs an Account client from an existing session.
func NewClient(session *api.Session) *Client {
	return &Client{Session: session}
}

// GetUser returns the authenticated user's profile and quota information.
func (c *Client) GetUser(ctx context.Context) (proton.User, error) {
	return c.Session.Client.GetUser(ctx)
}

// GetAddresses returns all email addresses associated with the account.
func (c *Client) GetAddresses(ctx context.Context) ([]proton.Address, error) {
	return c.Session.Client.GetAddresses(ctx)
}
