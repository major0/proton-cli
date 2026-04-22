package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/major0/proton-cli/api/lumo"
)

// CreateConversation creates a conversation in the given space with an
// encrypted title.
func (c *Client) CreateConversation(ctx context.Context, spaceID, title string) (*lumo.Conversation, error) {
	// Fetch the space to get its key and tag.
	space, err := c.GetSpace(ctx, spaceID)
	if err != nil {
		return nil, fmt.Errorf("lumo: create conversation: %w", err)
	}

	dek, err := c.deriveSpaceDEK(ctx, space)
	if err != nil {
		return nil, fmt.Errorf("lumo: create conversation: %w", err)
	}

	convTag := GenerateTag()
	ad := lumo.ConversationAD(convTag, space.SpaceTag)

	var encrypted string
	if title != "" {
		privJSON, err := json.Marshal(map[string]string{"title": title})
		if err != nil {
			return nil, fmt.Errorf("lumo: create conversation: marshal: %w", err)
		}
		encrypted, err = lumo.EncryptString(string(privJSON), dek, ad)
		if err != nil {
			return nil, fmt.Errorf("lumo: create conversation: encrypt: %w", err)
		}
	}

	req := lumo.CreateConversationReq{
		SpaceID:         spaceID,
		ConversationTag: convTag,
		Encrypted:       encrypted,
	}

	var resp lumo.GetConversationResponse
	err = c.Session.DoJSON(ctx, "POST", "/api/lumo/v1/spaces/"+spaceID+"/conversations", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("lumo: create conversation: %w", mapCRUDError(err))
	}
	return &resp.Conversation, nil
}

// ListConversations fetches all conversations in a space.
func (c *Client) ListConversations(ctx context.Context, spaceID string) ([]lumo.Conversation, error) {
	var resp lumo.ListConversationsResponse
	if err := c.Session.DoJSON(ctx, "GET", "/api/lumo/v1/spaces/"+spaceID+"/conversations", nil, &resp); err != nil {
		return nil, fmt.Errorf("lumo: list conversations: %w", err)
	}
	return resp.Conversations, nil
}

// GetConversation fetches a conversation by ID.
func (c *Client) GetConversation(ctx context.Context, conversationID string) (*lumo.Conversation, error) {
	var resp lumo.GetConversationResponse
	err := c.Session.DoJSON(ctx, "GET", "/api/lumo/v1/conversations/"+conversationID, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("lumo: get conversation: %w", mapCRUDError(err))
	}
	return &resp.Conversation, nil
}

// DeleteConversation deletes a conversation by ID.
func (c *Client) DeleteConversation(ctx context.Context, conversationID string) error {
	err := c.Session.DoJSON(ctx, "DELETE", "/api/lumo/v1/conversations/"+conversationID, nil, nil)
	if err != nil {
		return fmt.Errorf("lumo: delete conversation: %w", mapCRUDError(err))
	}
	return nil
}

// deriveSpaceDEK unwraps a space's key and derives the DEK.
func (c *Client) deriveSpaceDEK(ctx context.Context, space *lumo.Space) ([]byte, error) {
	masterKey, err := c.GetMasterKey(ctx)
	if err != nil {
		return nil, err
	}

	wrappedKey, err := base64.StdEncoding.DecodeString(space.SpaceKey)
	if err != nil {
		return nil, fmt.Errorf("decode space key: %w", err)
	}

	spaceKey, err := lumo.UnwrapSpaceKey(masterKey, wrappedKey)
	if err != nil {
		return nil, err
	}

	return lumo.DeriveDataEncryptionKey(spaceKey)
}

// DeriveSpaceDEK is the exported version of deriveSpaceDEK for use by
// command-layer code that needs to decrypt conversation content.
func (c *Client) DeriveSpaceDEK(ctx context.Context, space *lumo.Space) ([]byte, error) {
	return c.deriveSpaceDEK(ctx, space)
}
