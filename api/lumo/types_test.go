package lumo

import "testing"

func TestRoleConstants(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleSystem, "system"},
		{RoleToolCall, "tool_call"},
		{RoleToolResult, "tool_result"},
	}
	for _, tt := range tests {
		if string(tt.role) != tt.want {
			t.Errorf("Role %q != %q", tt.role, tt.want)
		}
	}
}

func TestToolNameConstants(t *testing.T) {
	tests := []struct {
		tool ToolName
		want string
	}{
		{ToolWebSearch, "web_search"},
		{ToolWeather, "weather"},
		{ToolStock, "stock"},
		{ToolCryptocurrency, "cryptocurrency"},
		{ToolGenerateImage, "generate_image"},
		{ToolDescribeImage, "describe_image"},
		{ToolEditImage, "edit_image"},
		{ToolProtonInfo, "proton_info"},
	}
	for _, tt := range tests {
		if string(tt.tool) != tt.want {
			t.Errorf("ToolName %q != %q", tt.tool, tt.want)
		}
	}
}

func TestGenerationTargetConstants(t *testing.T) {
	tests := []struct {
		target GenerationTarget
		want   string
	}{
		{TargetMessage, "message"},
		{TargetTitle, "title"},
		{TargetToolCall, "tool_call"},
		{TargetToolResult, "tool_result"},
		{TargetReasoning, "reasoning"},
	}
	for _, tt := range tests {
		if string(tt.target) != tt.want {
			t.Errorf("GenerationTarget %q != %q", tt.target, tt.want)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []string{"done", "timeout", "error", "rejected", "harmful"}
	for _, typ := range terminal {
		msg := GenerationResponseMessage{Type: typ}
		if !msg.IsTerminal() {
			t.Errorf("IsTerminal() = false for type %q", typ)
		}
	}

	nonTerminal := []string{"queued", "ingesting", "token_data", "image_data"}
	for _, typ := range nonTerminal {
		msg := GenerationResponseMessage{Type: typ}
		if msg.IsTerminal() {
			t.Errorf("IsTerminal() = true for type %q", typ)
		}
	}
}

func TestIsEncrypted(t *testing.T) {
	msg := GenerationResponseMessage{Encrypted: true}
	if !msg.IsEncrypted() {
		t.Error("IsEncrypted() = false, want true")
	}
	msg.Encrypted = false
	if msg.IsEncrypted() {
		t.Error("IsEncrypted() = true, want false")
	}
}
