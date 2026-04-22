// Package lumo provides types for the Proton Lumo AI assistant API.
package lumo

// Role identifies the sender of a conversation turn.
type Role string

const (
	// RoleUser is a message from the user.
	RoleUser Role = "user"
	// RoleAssistant is a message from the assistant.
	RoleAssistant Role = "assistant"
	// RoleSystem is a system prompt.
	RoleSystem Role = "system"
	// RoleToolCall is a tool invocation by the assistant.
	RoleToolCall Role = "tool_call"
	// RoleToolResult is the result of a tool invocation.
	RoleToolResult Role = "tool_result"
)

// Turn is a single message in a conversation.
type Turn struct {
	Role      Role        `json:"role"`
	Content   string      `json:"content,omitempty"`
	Encrypted bool        `json:"encrypted,omitempty"`
	Images    []WireImage `json:"images,omitempty"`
}

// WireImage is an image attachment on a turn.
type WireImage struct {
	Encrypted bool   `json:"encrypted"`
	ImageID   string `json:"image_id"`
	Data      string `json:"data"`
}

// ToolName identifies a tool available to the assistant.
type ToolName string

// Tool name constants for available assistant tools.
const (
	ToolWebSearch      ToolName = "web_search"
	ToolWeather        ToolName = "weather"
	ToolStock          ToolName = "stock"
	ToolCryptocurrency ToolName = "cryptocurrency"
	ToolGenerateImage  ToolName = "generate_image"
	ToolDescribeImage  ToolName = "describe_image"
	ToolEditImage      ToolName = "edit_image"
	ToolProtonInfo     ToolName = "proton_info"
)

// GenerationTarget identifies what the model should generate.
type GenerationTarget string

const (
	// TargetMessage generates a chat message.
	TargetMessage GenerationTarget = "message"
	// TargetTitle generates a conversation title.
	TargetTitle GenerationTarget = "title"
	// TargetToolCall generates a tool invocation.
	TargetToolCall GenerationTarget = "tool_call"
	// TargetToolResult generates a tool result.
	TargetToolResult GenerationTarget = "tool_result"
	// TargetReasoning generates reasoning output.
	TargetReasoning GenerationTarget = "reasoning"
)

// Options configures generation behavior.
type Options struct {
	Tools []ToolName `json:"tools,omitempty"`
}

// GenerationRequest is the payload sent to the Lumo chat endpoint.
type GenerationRequest struct {
	Type       string             `json:"type"`
	Turns      []Turn             `json:"turns"`
	Options    *Options           `json:"options,omitempty"`
	Targets    []GenerationTarget `json:"targets,omitempty"`
	RequestKey string             `json:"request_key,omitempty"`
	RequestID  string             `json:"request_id,omitempty"`
}

// ChatEndpointGenerationRequest wraps GenerationRequest in the legacy
// Prompt field expected by the PHP backend at /chat.
type ChatEndpointGenerationRequest struct {
	Prompt GenerationRequest `json:"Prompt"`
}

// GenerationResponseMessage is a flat union of all SSE response message
// types. The Type field discriminates which fields are meaningful.
//
// Valid Type values: "queued", "ingesting", "token_data", "image_data",
// "done", "timeout", "error", "rejected", "harmful".
type GenerationResponseMessage struct {
	Type      string           `json:"type"`
	Target    GenerationTarget `json:"target,omitempty"`
	Count     int              `json:"count,omitempty"`
	Content   string           `json:"content,omitempty"`
	Encrypted bool             `json:"encrypted,omitempty"`
	ImageID   string           `json:"image_id,omitempty"`
	Data      string           `json:"data,omitempty"`
	IsFinal   bool             `json:"is_final,omitempty"`
	Seed      int              `json:"seed,omitempty"`
}

// IsTerminal reports whether the message ends the SSE stream.
func (m *GenerationResponseMessage) IsTerminal() bool {
	switch m.Type {
	case "done", "timeout", "error", "rejected", "harmful":
		return true
	}
	return false
}

// IsEncrypted reports whether the message carries encrypted content.
func (m *GenerationResponseMessage) IsEncrypted() bool {
	return m.Encrypted
}
