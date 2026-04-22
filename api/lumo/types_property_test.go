package lumo

import (
	"encoding/json"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

// validRoles is the set of valid Role values for generators.
var validRoles = []Role{RoleUser, RoleAssistant, RoleSystem, RoleToolCall, RoleToolResult}

// validTargets is the set of valid GenerationTarget values for generators.
var validTargets = []GenerationTarget{TargetMessage, TargetTitle, TargetToolCall, TargetToolResult, TargetReasoning}

// validTools is the set of valid ToolName values for generators.
var validTools = []ToolName{
	ToolWebSearch, ToolWeather, ToolStock, ToolCryptocurrency,
	ToolGenerateImage, ToolDescribeImage, ToolEditImage, ToolProtonInfo,
}

// validResponseTypes is the set of valid Type discriminators.
var validResponseTypes = []string{
	"queued", "ingesting", "token_data", "image_data",
	"done", "timeout", "error", "rejected", "harmful",
}

func genRole(t *rapid.T) Role {
	return validRoles[rapid.IntRange(0, len(validRoles)-1).Draw(t, "role")]
}

func genTarget(t *rapid.T) GenerationTarget {
	return validTargets[rapid.IntRange(0, len(validTargets)-1).Draw(t, "target")]
}

func genWireImage(t *rapid.T) WireImage {
	return WireImage{
		Encrypted: rapid.Bool().Draw(t, "img_encrypted"),
		ImageID:   rapid.StringMatching(`[a-zA-Z0-9]{1,32}`).Draw(t, "image_id"),
		Data:      rapid.StringMatching(`[a-zA-Z0-9+/=]{0,64}`).Draw(t, "img_data"),
	}
}

func genTurn(t *rapid.T) Turn {
	turn := Turn{
		Role:      genRole(t),
		Content:   rapid.String().Draw(t, "content"),
		Encrypted: rapid.Bool().Draw(t, "encrypted"),
	}
	n := rapid.IntRange(0, 3).Draw(t, "num_images")
	if n > 0 {
		turn.Images = make([]WireImage, n)
		for i := range turn.Images {
			turn.Images[i] = genWireImage(t)
		}
	}
	return turn
}

func genOptions(t *rapid.T) *Options {
	if !rapid.Bool().Draw(t, "has_options") {
		return nil
	}
	n := rapid.IntRange(0, len(validTools)).Draw(t, "num_tools")
	if n == 0 {
		return &Options{} // nil Tools, not empty slice
	}
	tools := make([]ToolName, n)
	for i := range tools {
		tools[i] = validTools[rapid.IntRange(0, len(validTools)-1).Draw(t, "tool")]
	}
	return &Options{Tools: tools}
}

func genChatEndpointGenerationRequest(t *rapid.T) ChatEndpointGenerationRequest {
	numTurns := rapid.IntRange(1, 10).Draw(t, "num_turns")
	turns := make([]Turn, numTurns)
	for i := range turns {
		turns[i] = genTurn(t)
	}

	numTargets := rapid.IntRange(0, len(validTargets)).Draw(t, "num_targets")
	var targets []GenerationTarget
	if numTargets > 0 {
		targets = make([]GenerationTarget, numTargets)
		for i := range targets {
			targets[i] = genTarget(t)
		}
	}

	return ChatEndpointGenerationRequest{
		Prompt: GenerationRequest{
			Type:       "generation_request",
			Turns:      turns,
			Options:    genOptions(t),
			Targets:    targets,
			RequestKey: rapid.StringMatching(`[a-zA-Z0-9+/=]{0,64}`).Draw(t, "request_key"),
			RequestID:  rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "request_id"),
		},
	}
}

// TestGenerationRequest_JSONRoundTrip_Property verifies that for any valid
// ChatEndpointGenerationRequest, JSON marshal → unmarshal produces an
// equal value.
//
// Feature: lumo-api, Property 1: GenerationRequest JSON round-trip
//
// **Validates: Requirements 1.2, 1.3, 1.4, 1.5, 1.6, 1.11**
func TestGenerationRequest_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genChatEndpointGenerationRequest(t)

		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		var got ChatEndpointGenerationRequest
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		if !chatRequestEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

// chatRequestEqual compares two ChatEndpointGenerationRequest values,
// handling *Options pointer comparison by value.
func chatRequestEqual(a, b ChatEndpointGenerationRequest) bool {
	ap, bp := a.Prompt, b.Prompt
	if ap.Type != bp.Type || ap.RequestKey != bp.RequestKey || ap.RequestID != bp.RequestID {
		return false
	}
	if !reflect.DeepEqual(ap.Turns, bp.Turns) || !reflect.DeepEqual(ap.Targets, bp.Targets) {
		return false
	}
	switch {
	case ap.Options == nil && bp.Options == nil:
		return true
	case ap.Options == nil || bp.Options == nil:
		return false
	default:
		return reflect.DeepEqual(*ap.Options, *bp.Options)
	}
}

func genResponseMessage(t *rapid.T) GenerationResponseMessage {
	typ := validResponseTypes[rapid.IntRange(0, len(validResponseTypes)-1).Draw(t, "msg_type")]

	msg := GenerationResponseMessage{Type: typ}

	switch typ {
	case "queued":
		if rapid.Bool().Draw(t, "has_target") {
			msg.Target = genTarget(t)
		}
	case "ingesting":
		msg.Target = genTarget(t)
	case "token_data":
		msg.Target = genTarget(t)
		msg.Count = rapid.IntRange(0, 10000).Draw(t, "count")
		msg.Content = rapid.String().Draw(t, "content")
		msg.Encrypted = rapid.Bool().Draw(t, "encrypted")
	case "image_data":
		if rapid.Bool().Draw(t, "has_image_id") {
			msg.ImageID = rapid.StringMatching(`[a-zA-Z0-9]{1,32}`).Draw(t, "image_id")
		}
		if rapid.Bool().Draw(t, "has_data") {
			msg.Data = rapid.StringMatching(`[a-zA-Z0-9+/=]{0,64}`).Draw(t, "data")
		}
		msg.IsFinal = rapid.Bool().Draw(t, "is_final")
		msg.Seed = rapid.IntRange(0, 999999).Draw(t, "seed")
		msg.Encrypted = rapid.Bool().Draw(t, "encrypted")
	}
	// done, timeout, error, rejected, harmful have no extra fields.

	return msg
}

// TestResponseMessage_JSONRoundTrip_Property verifies that for any valid
// GenerationResponseMessage with a valid type discriminator, JSON marshal →
// unmarshal produces an equal value.
//
// Feature: lumo-api, Property 2: GenerationResponseMessage JSON round-trip
//
// **Validates: Requirements 1.9, 1.11**
func TestResponseMessage_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genResponseMessage(t)

		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		var got GenerationResponseMessage
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}
