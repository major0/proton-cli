package lumoCmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/major0/proton-cli/api/lumo"
	lumoClient "github.com/major0/proton-cli/api/lumo/client"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var chatSpaceFlag string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat with Proton Lumo",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
}

func init() {
	AddCommand(chatCmd)
	chatCmd.PersistentFlags().StringVar(&chatSpaceFlag, "space", "", "Space ID (defaults to simple space)")
}

// resolveSpace returns the space ID from the --space flag or the default space.
func resolveSpace(ctx context.Context, client *lumoClient.Client) (string, error) {
	if chatSpaceFlag != "" {
		return chatSpaceFlag, nil
	}
	space, err := client.GetDefaultSpace(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving default space: %w", err)
	}
	return space.ID, nil
}

// restoreClient restores the session and creates a Lumo client.
func restoreClient(cmd *cobra.Command) (*lumoClient.Client, error) {
	session, err := cli.RestoreSession(cmd.Context())
	if err != nil {
		return nil, fmt.Errorf("no active session (run 'proton account login' first): %w", err)
	}
	return lumoClient.NewClient(session), nil
}

// --- chat create ---

var chatCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new conversation and enter interactive chat",
	RunE:  runChatCreate,
}

func init() {
	chatCmd.AddCommand(chatCreateCmd)
}

func runChatCreate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	client, err := restoreClient(cmd)
	if err != nil {
		return err
	}

	spaceID, err := resolveSpace(ctx, client)
	if err != nil {
		return err
	}

	conv, err := client.CreateConversation(ctx, spaceID, "")
	if err != nil {
		return fmt.Errorf("creating conversation: %w", err)
	}

	session := &ChatSession{
		Client:       client,
		Conversation: conv,
		SpaceID:      spaceID,
		Writer:       os.Stdout,
		Reader:       os.Stdin,
	}

	return session.Run(ctx)
}

// --- chat resume ---

var chatResumeCmd = &cobra.Command{
	Use:   "resume <conversation-id>",
	Short: "Resume an existing conversation",
	Args:  cobra.ExactArgs(1),
	RunE:  runChatResume,
}

func init() {
	chatCmd.AddCommand(chatResumeCmd)
}

func runChatResume(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := restoreClient(cmd)
	if err != nil {
		return err
	}

	convID := args[0]
	conv, err := client.GetConversation(ctx, convID)
	if err != nil {
		return fmt.Errorf("loading conversation: %w", err)
	}

	messages, err := client.ListMessages(ctx, convID)
	if err != nil {
		return fmt.Errorf("loading messages: %w", err)
	}

	space, err := client.GetSpace(ctx, conv.SpaceID)
	if err != nil {
		return fmt.Errorf("loading space: %w", err)
	}

	dek, err := client.DeriveSpaceDEK(ctx, space)
	if err != nil {
		return fmt.Errorf("deriving decryption key: %w", err)
	}

	decrypt := func(msg lumo.Message) string {
		return decryptMessageContent(msg, dek, conv.ConversationTag)
	}

	// Build turns from history.
	var turns []lumo.Turn
	for _, msg := range messages {
		content := decrypt(msg)
		role := lumo.RoleUser
		if msg.Role == lumoClient.RoleAssistant {
			role = lumo.RoleAssistant
		}
		turns = append(turns, lumo.Turn{Role: role, Content: content})
	}

	// Print history.
	if history := FormatHistory(messages, decrypt); history != "" {
		_, _ = fmt.Fprint(os.Stdout, history)
	}

	session := &ChatSession{
		Client:         client,
		Conversation:   conv,
		SpaceID:        conv.SpaceID,
		Turns:          turns,
		Writer:         os.Stdout,
		Reader:         os.Stdin,
		TitleGenerated: len(turns) > 0,
	}

	return session.Run(ctx)
}

// decryptMessageContent decrypts a message's encrypted content, returning
// the plaintext. Returns an empty string on decryption failure.
func decryptMessageContent(msg lumo.Message, dek []byte, convTag string) string {
	if msg.Encrypted == "" {
		return ""
	}

	role := "user"
	if msg.Role == lumoClient.RoleAssistant {
		role = "assistant"
	}

	ad := lumo.MessageAD(msg.MessageTag, role, msg.ParentID, convTag)
	plainJSON, err := lumo.DecryptString(msg.Encrypted, dek, ad)
	if err != nil {
		return ""
	}

	var priv struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(plainJSON), &priv); err != nil {
		return ""
	}
	return priv.Content
}

// --- chat list ---

var chatListCmd = &cobra.Command{
	Use:   "list",
	Short: "List conversations in a space",
	RunE:  runChatList,
}

func init() {
	chatCmd.AddCommand(chatListCmd)
}

func runChatList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	client, err := restoreClient(cmd)
	if err != nil {
		return err
	}

	spaceID, err := resolveSpace(ctx, client)
	if err != nil {
		return err
	}

	convs, err := client.ListConversations(ctx, spaceID)
	if err != nil {
		return fmt.Errorf("listing conversations: %w", err)
	}

	active := FilterActiveConversations(convs)

	// Derive DEK for title decryption.
	space, err := client.GetSpace(ctx, spaceID)
	if err != nil {
		return fmt.Errorf("loading space: %w", err)
	}

	dek, err := client.DeriveSpaceDEK(ctx, space)
	if err != nil {
		return fmt.Errorf("deriving decryption key: %w", err)
	}

	rows := make([]ConversationRow, len(active))
	for i, c := range active {
		rows[i] = ConversationRow{
			ID:         c.ID,
			Title:      decryptConversationTitle(c, dek, space.SpaceTag),
			CreateTime: c.CreateTime,
		}
	}

	_, _ = fmt.Fprint(os.Stdout, FormatConversationList(rows))
	return nil
}

// decryptConversationTitle decrypts a conversation's encrypted title.
// Returns an empty string on failure (FormatConversationList will show
// "Untitled").
func decryptConversationTitle(conv lumo.Conversation, dek []byte, spaceTag string) string {
	if conv.Encrypted == "" {
		return ""
	}

	ad := lumo.ConversationAD(conv.ConversationTag, spaceTag)
	plainJSON, err := lumo.DecryptString(conv.Encrypted, dek, ad)
	if err != nil {
		return ""
	}

	var priv struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(plainJSON), &priv); err != nil {
		return ""
	}
	return priv.Title
}

// --- chat delete ---

var chatDeleteCmd = &cobra.Command{
	Use:   "delete <conversation-id>",
	Short: "Delete a conversation",
	Args:  cobra.ExactArgs(1),
	RunE:  runChatDelete,
}

func init() {
	chatCmd.AddCommand(chatDeleteCmd)
}

func runChatDelete(cmd *cobra.Command, args []string) error {
	client, err := restoreClient(cmd)
	if err != nil {
		return err
	}

	convID := args[0]
	if err := client.DeleteConversation(cmd.Context(), convID); err != nil {
		return fmt.Errorf("deleting conversation: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Conversation %s deleted.\n", convID)
	return nil
}
