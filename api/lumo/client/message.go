package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/major0/proton-cli/api/lumo"
)

const (
	// RoleUser is the wire-format integer for user messages.
	RoleUser = 1
	// RoleAssistant is the wire-format integer for assistant messages.
	RoleAssistant = 2
)

// roleString maps wire-format role integers to AD string values.
func roleString(role int) string {
	switch role {
	case RoleUser:
		return "user"
	case RoleAssistant:
		return "assistant"
	default:
		return "unknown"
	}
}

// CreateMessage creates a message in the given conversation with
// encrypted content.
func (c *Client) CreateMessage(ctx context.Context, conversationID string, role int, content string) (*lumo.Message, error) {
	// Fetch the conversation to get its space and tag.
	conv, err := c.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("lumo: create message: %w", err)
	}

	// Fetch the space to get its key.
	space, err := c.GetSpace(ctx, conv.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("lumo: create message: %w", err)
	}

	dek, err := c.deriveSpaceDEK(ctx, space)
	if err != nil {
		return nil, fmt.Errorf("lumo: create message: %w", err)
	}

	msgTag := GenerateTag()
	ad := lumo.MessageAD(msgTag, roleString(role), "", conv.ConversationTag)

	var encrypted string
	if content != "" {
		privJSON, err := json.Marshal(map[string]string{"content": content})
		if err != nil {
			return nil, fmt.Errorf("lumo: create message: marshal: %w", err)
		}
		encrypted, err = lumo.EncryptString(string(privJSON), dek, ad)
		if err != nil {
			return nil, fmt.Errorf("lumo: create message: encrypt: %w", err)
		}
	}

	req := lumo.CreateMessageReq{
		ConversationID: conversationID,
		MessageTag:     msgTag,
		Role:           role,
		Encrypted:      encrypted,
	}

	var resp lumo.GetMessageResponse
	err = c.Session.DoJSON(ctx, "POST", "/api/lumo/v1/conversations/"+conversationID+"/messages", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("lumo: create message: %w", mapCRUDError(err))
	}
	return &resp.Message, nil
}

// GetMessage fetches a message by ID.
func (c *Client) GetMessage(ctx context.Context, messageID string) (*lumo.Message, error) {
	var resp lumo.GetMessageResponse
	err := c.Session.DoJSON(ctx, "GET", "/api/lumo/v1/messages/"+messageID, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("lumo: get message: %w", mapCRUDError(err))
	}
	return &resp.Message, nil
}

// ListMessages fetches all messages in a conversation.
func (c *Client) ListMessages(ctx context.Context, conversationID string) ([]lumo.Message, error) {
	var resp lumo.ListMessagesResponse
	if err := c.Session.DoJSON(ctx, "GET", "/api/lumo/v1/conversations/"+conversationID+"/messages", nil, &resp); err != nil {
		return nil, fmt.Errorf("lumo: list messages: %w", err)
	}
	return resp.Messages, nil
}
