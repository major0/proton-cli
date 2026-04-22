package openai

import (
	"encoding/json"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

// genMessage generates an arbitrary Message.
func genMessage(t *rapid.T) Message {
	return Message{
		Role:    rapid.StringMatching(`(system|user|assistant)`).Draw(t, "role"),
		Content: rapid.String().Draw(t, "content"),
	}
}

// genMessages generates a non-empty slice of Messages.
func genMessages(t *rapid.T) []Message {
	n := rapid.IntRange(1, 10).Draw(t, "num_messages")
	msgs := make([]Message, n)
	for i := range msgs {
		msgs[i] = genMessage(t)
	}
	return msgs
}

// genOptFloat64 generates an optional *float64.
func genOptFloat64(t *rapid.T) *float64 {
	if !rapid.Bool().Draw(t, "has_temp") {
		return nil
	}
	v := rapid.Float64Range(0, 2).Draw(t, "temperature")
	return &v
}

// genOptInt generates an optional *int.
func genOptInt(t *rapid.T) *int {
	if !rapid.Bool().Draw(t, "has_max_tokens") {
		return nil
	}
	v := rapid.IntRange(1, 4096).Draw(t, "max_tokens")
	return &v
}

// genOptString generates an optional *string.
func genOptString(t *rapid.T) *string {
	if !rapid.Bool().Draw(t, "has_finish_reason") {
		return nil
	}
	v := rapid.StringMatching(`(stop|length|content_filter)`).Draw(t, "finish_reason")
	return &v
}

// genChoice generates an arbitrary Choice.
func genChoice(t *rapid.T, streaming bool) Choice {
	c := Choice{
		Index:        rapid.IntRange(0, 5).Draw(t, "index"),
		FinishReason: genOptString(t),
	}
	if streaming {
		msg := genMessage(t)
		c.Delta = &msg
	} else {
		msg := genMessage(t)
		c.Message = &msg
	}
	return c
}

// genUsage generates an optional *Usage.
func genUsage(t *rapid.T) *Usage {
	if !rapid.Bool().Draw(t, "has_usage") {
		return nil
	}
	prompt := rapid.IntRange(0, 10000).Draw(t, "prompt_tokens")
	completion := rapid.IntRange(0, 10000).Draw(t, "completion_tokens")
	return &Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
}

func genChatCompletionRequest(t *rapid.T) ChatCompletionRequest {
	return ChatCompletionRequest{
		Model:       rapid.StringMatching(`[a-z0-9-]{1,32}`).Draw(t, "model"),
		Messages:    genMessages(t),
		Stream:      rapid.Bool().Draw(t, "stream"),
		Temperature: genOptFloat64(t),
		MaxTokens:   genOptInt(t),
	}
}

func genChatCompletionResponse(t *rapid.T) ChatCompletionResponse {
	n := rapid.IntRange(1, 4).Draw(t, "num_choices")
	choices := make([]Choice, n)
	for i := range choices {
		choices[i] = genChoice(t, false)
	}
	return ChatCompletionResponse{
		ID:      rapid.StringMatching(`chatcmpl-[a-zA-Z0-9]{10}`).Draw(t, "id"),
		Object:  "chat.completion",
		Created: rapid.Int64Range(0, 1<<40).Draw(t, "created"),
		Model:   rapid.StringMatching(`[a-z0-9-]{1,32}`).Draw(t, "model"),
		Choices: choices,
		Usage:   genUsage(t),
	}
}

func genChatCompletionChunk(t *rapid.T) ChatCompletionChunk {
	n := rapid.IntRange(1, 4).Draw(t, "num_choices")
	choices := make([]Choice, n)
	for i := range choices {
		choices[i] = genChoice(t, true)
	}
	return ChatCompletionChunk{
		ID:      rapid.StringMatching(`chatcmpl-[a-zA-Z0-9]{10}`).Draw(t, "id"),
		Object:  "chat.completion.chunk",
		Created: rapid.Int64Range(0, 1<<40).Draw(t, "created"),
		Model:   rapid.StringMatching(`[a-z0-9-]{1,32}`).Draw(t, "model"),
		Choices: choices,
	}
}

func genModel(t *rapid.T) Model {
	return Model{
		ID:      rapid.StringMatching(`[a-z0-9-]{1,32}`).Draw(t, "id"),
		Object:  "model",
		Created: rapid.Int64Range(0, 1<<40).Draw(t, "created"),
		OwnedBy: rapid.StringMatching(`[a-z0-9-]{1,32}`).Draw(t, "owned_by"),
	}
}

func genModelList(t *rapid.T) ModelList {
	n := rapid.IntRange(1, 5).Draw(t, "num_models")
	models := make([]Model, n)
	for i := range models {
		models[i] = genModel(t)
	}
	return ModelList{
		Object: "list",
		Data:   models,
	}
}

func genErrorBody(t *rapid.T) ErrorBody {
	var code *string
	if rapid.Bool().Draw(t, "has_code") {
		c := rapid.StringMatching(`[a-z_]{1,32}`).Draw(t, "code")
		code = &c
	}
	return ErrorBody{
		Message: rapid.String().Draw(t, "message"),
		Type:    rapid.StringMatching(`[a-z_]{1,32}`).Draw(t, "type"),
		Code:    code,
	}
}

func genErrorResponse(t *rapid.T) ErrorResponse {
	return ErrorResponse{Error: genErrorBody(t)}
}

// TestChatCompletionRequest_JSONRoundTrip_Property verifies that for any
// valid ChatCompletionRequest, JSON marshal → unmarshal produces an equal value.
//
// Feature: lumo-serve, Property 1: OpenAI types JSON round-trip
//
// **Validates: Requirements 4.1, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6**
func TestChatCompletionRequest_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genChatCompletionRequest(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var got ChatCompletionRequest
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

// TestChatCompletionResponse_JSONRoundTrip_Property verifies that for any
// valid ChatCompletionResponse, JSON marshal → unmarshal produces an equal value.
//
// Feature: lumo-serve, Property 1: OpenAI types JSON round-trip
//
// **Validates: Requirements 4.1, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6**
func TestChatCompletionResponse_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genChatCompletionResponse(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var got ChatCompletionResponse
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

// TestChatCompletionChunk_JSONRoundTrip_Property verifies that for any
// valid ChatCompletionChunk, JSON marshal → unmarshal produces an equal value.
//
// Feature: lumo-serve, Property 1: OpenAI types JSON round-trip
//
// **Validates: Requirements 4.1, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6**
func TestChatCompletionChunk_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genChatCompletionChunk(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var got ChatCompletionChunk
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

// TestModelList_JSONRoundTrip_Property verifies that for any valid ModelList,
// JSON marshal → unmarshal produces an equal value.
//
// Feature: lumo-serve, Property 1: OpenAI types JSON round-trip
//
// **Validates: Requirements 4.1, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6**
func TestModelList_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genModelList(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var got ModelList
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

// TestErrorResponse_JSONRoundTrip_Property verifies that for any valid
// ErrorResponse, JSON marshal → unmarshal produces an equal value.
//
// Feature: lumo-serve, Property 1: OpenAI types JSON round-trip
//
// **Validates: Requirements 4.1, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6**
func TestErrorResponse_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genErrorResponse(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var got ErrorResponse
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}
