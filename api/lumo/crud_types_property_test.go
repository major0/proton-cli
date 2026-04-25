package lumo

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// --- Generators ---

func genMasterKeyEntry(t *rapid.T) MasterKeyEntry {
	return MasterKeyEntry{
		ID:         rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "id"),
		IsLatest:   rapid.Bool().Draw(t, "is_latest"),
		Version:    rapid.IntRange(1, 100).Draw(t, "version"),
		CreateTime: rapid.StringMatching(`2024-0[1-9]-[012][0-9]T[01][0-9]:[0-5][0-9]:[0-5][0-9]Z`).Draw(t, "create_time"),
		MasterKey:  rapid.StringMatching(`[a-zA-Z0-9+/=]{8,32}`).Draw(t, "master_key"),
	}
}

func genSpace(t *rapid.T) Space {
	return Space{
		ID:         rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "id"),
		SpaceKey:   rapid.StringMatching(`[a-zA-Z0-9+/=]{8,32}`).Draw(t, "space_key"),
		SpaceTag:   rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "space_tag"),
		Encrypted:  rapid.StringMatching(`[a-zA-Z0-9+/=]{0,32}`).Draw(t, "encrypted"),
		CreateTime: rapid.StringMatching(`2024-0[1-9]-[012][0-9]T[01][0-9]:[0-5][0-9]:[0-5][0-9]Z`).Draw(t, "create_time"),
		UpdateTime: rapid.StringMatching(`[0-9T:Z-]{0,20}`).Draw(t, "update_time"),
		DeleteTime: rapid.StringMatching(`[0-9T:Z-]{0,20}`).Draw(t, "delete_time"),
	}
}

func genConversation(t *rapid.T) Conversation {
	return Conversation{
		ID:              rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "id"),
		SpaceID:         rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "space_id"),
		ConversationTag: rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "conv_tag"),
		Encrypted:       rapid.StringMatching(`[a-zA-Z0-9+/=]{0,32}`).Draw(t, "encrypted"),
		IsStarred:       rapid.Bool().Draw(t, "is_starred"),
		CreateTime:      rapid.StringMatching(`2024-0[1-9]-[012][0-9]T[01][0-9]:[0-5][0-9]:[0-5][0-9]Z`).Draw(t, "create_time"),
		UpdateTime:      rapid.StringMatching(`[0-9T:Z-]{0,20}`).Draw(t, "update_time"),
		DeleteTime:      rapid.StringMatching(`[0-9T:Z-]{0,20}`).Draw(t, "delete_time"),
	}
}

func genMessage(t *rapid.T) Message {
	return Message{
		ID:             rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "id"),
		ConversationID: rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "conv_id"),
		MessageTag:     rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "msg_tag"),
		Role:           rapid.IntRange(1, 2).Draw(t, "role"),
		Encrypted:      rapid.StringMatching(`[a-zA-Z0-9+/=]{0,32}`).Draw(t, "encrypted"),
		Status:         rapid.IntRange(0, 2).Draw(t, "status"),
		CreateTime:     rapid.StringMatching(`2024-0[1-9]-[012][0-9]T[01][0-9]:[0-5][0-9]:[0-5][0-9]Z`).Draw(t, "create_time"),
		ParentID:       rapid.StringMatching(`[a-zA-Z0-9]{0,16}`).Draw(t, "parent_id"),
	}
}

func genCreateSpaceReq(t *rapid.T) CreateSpaceReq {
	return CreateSpaceReq{
		SpaceKey:  rapid.StringMatching(`[a-zA-Z0-9+/=]{8,32}`).Draw(t, "space_key"),
		SpaceTag:  rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "space_tag"),
		Encrypted: rapid.StringMatching(`[a-zA-Z0-9+/=]{0,32}`).Draw(t, "encrypted"),
	}
}

func genCreateConversationReq(t *rapid.T) CreateConversationReq {
	return CreateConversationReq{
		SpaceID:         rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "space_id"),
		IsStarred:       rapid.Bool().Draw(t, "is_starred"),
		Encrypted:       rapid.StringMatching(`[a-zA-Z0-9+/=]{0,32}`).Draw(t, "encrypted"),
		ConversationTag: rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "conv_tag"),
	}
}

func genCreateMessageReq(t *rapid.T) CreateMessageReq {
	return CreateMessageReq{
		ConversationID: rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "conv_id"),
		MessageTag:     rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "msg_tag"),
		Role:           rapid.IntRange(1, 2).Draw(t, "role"),
		Status:         rapid.IntRange(0, 2).Draw(t, "status"),
		Encrypted:      rapid.StringMatching(`[a-zA-Z0-9+/=]{0,32}`).Draw(t, "encrypted"),
		ParentID:       rapid.StringMatching(`[a-zA-Z0-9]{0,16}`).Draw(t, "parent_id"),
	}
}

func genLinkedDriveFolder(t *rapid.T) *LinkedDriveFolder {
	if !rapid.Bool().Draw(t, "has_folder") {
		return nil
	}
	return &LinkedDriveFolder{
		FolderID:   rapid.StringMatching(`[a-zA-Z0-9]{1,16}`).Draw(t, "folder_id"),
		FolderName: rapid.StringMatching(`[a-zA-Z0-9 ]{1,16}`).Draw(t, "folder_name"),
		FolderPath: rapid.StringMatching(`/[a-zA-Z0-9/]{0,32}`).Draw(t, "folder_path"),
	}
}

func genSpacePriv(t *rapid.T) SpacePriv {
	sp := SpacePriv{}
	// Three cases: nil (omitted), false (simple), true (project)
	switch rapid.IntRange(0, 2).Draw(t, "is_project_case") {
	case 0:
		// nil — omitted
	case 1:
		f := false
		sp.IsProject = &f
	case 2:
		tr := true
		sp.IsProject = &tr
		sp.ProjectName = rapid.StringMatching(`[a-zA-Z0-9 ]{0,16}`).Draw(t, "project_name")
		sp.ProjectInstructions = rapid.StringMatching(`[a-zA-Z0-9 ]{0,32}`).Draw(t, "project_instructions")
		sp.ProjectIcon = rapid.StringMatching(`[a-z]{0,8}`).Draw(t, "project_icon")
		sp.LinkedDriveFolder = genLinkedDriveFolder(t)
	}
	return sp
}

// --- Property 1: Master key best-key selection ---

// TestSelectBestMasterKey_Property verifies that for any non-empty list
// of MasterKeyEntry values, SelectBestMasterKey returns the entry that
// is maximal under the ordering: IsLatest=true > false, then highest
// Version, then most recent CreateTime, then lowest ID.
//
// Feature: lumo-crud, Property 1: Master key best-key selection
//
// **Validates: Requirements 1.1**
func TestSelectBestMasterKey_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(t, "num_keys")
		keys := make([]MasterKeyEntry, n)
		for i := range keys {
			keys[i] = genMasterKeyEntry(t)
		}

		best, err := SelectBestMasterKey(keys)
		if err != nil {
			t.Fatalf("SelectBestMasterKey: %v", err)
		}

		// Sort a copy and verify best matches the first element.
		sorted := make([]MasterKeyEntry, len(keys))
		copy(sorted, keys)
		SortMasterKeys(sorted)

		if !reflect.DeepEqual(best, sorted[0]) {
			t.Fatalf("best key mismatch:\ngot:  %+v\nwant: %+v", best, sorted[0])
		}
	})
}

// --- Property 6: CRUD wire-format types JSON round-trip ---

// TestCRUDTypes_JSONRoundTrip_Property verifies that for any valid
// Space, Conversation, Message, MasterKeyEntry, CreateSpaceReq,
// CreateConversationReq, or CreateMessageReq, JSON marshal → unmarshal
// produces an equal value.
//
// Feature: lumo-crud, Property 6: CRUD wire-format types JSON round-trip
//
// **Validates: Requirements 6.1, 6.2, 6.3, 6.4, 6.6, 6.7**
func TestCRUDTypes_JSONRoundTrip_Property(t *testing.T) {
	t.Run("MasterKeyEntry", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := genMasterKeyEntry(t)
			assertJSONRoundTrip(t, orig)
		})
	})

	t.Run("Space", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := genSpace(t)
			assertJSONRoundTrip(t, orig)
		})
	})

	t.Run("Conversation", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := genConversation(t)
			assertJSONRoundTrip(t, orig)
		})
	})

	t.Run("Message", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := genMessage(t)
			assertJSONRoundTrip(t, orig)
		})
	})

	t.Run("CreateSpaceReq", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := genCreateSpaceReq(t)
			assertJSONRoundTrip(t, orig)
		})
	})

	t.Run("CreateConversationReq", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := genCreateConversationReq(t)
			assertJSONRoundTrip(t, orig)
		})
	})

	t.Run("CreateMessageReq", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := genCreateMessageReq(t)
			assertJSONRoundTrip(t, orig)
		})
	})

	// Feature: lumo-space, Property 4: UpdateSpaceReq JSON round-trip
	//
	// **Validates: Requirements 3.5**
	t.Run("UpdateSpaceReq", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orig := UpdateSpaceReq{
				Encrypted: rapid.StringMatching(`[a-zA-Z0-9+/=]{0,64}`).Draw(t, "encrypted"),
			}
			assertJSONRoundTrip(t, orig)
		})
	})
}

// assertJSONRoundTrip marshals v to JSON, unmarshals into a new value
// of the same type, and verifies equality.
func assertJSONRoundTrip[T any](t *rapid.T, orig T) {
	t.Helper()
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got T
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(orig, got) {
		t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
	}
}

// --- Property 7 (lumo-chat): New response types JSON round-trip ---

// TestListResponses_JSONRoundTrip_Property verifies that for any valid
// ListConversationsResponse or ListMessagesResponse, JSON marshal
// followed by unmarshal produces a value equal to the original.
//
// Feature: lumo-chat, Property 7: New response types JSON round-trip
//
// **Validates: Requirements 1.3, 3.4**
func TestListResponses_JSONRoundTrip_Property(t *testing.T) {
	t.Run("ListConversationsResponse", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			n := rapid.IntRange(0, 10).Draw(t, "num_conversations")
			convs := make([]Conversation, n)
			for i := range convs {
				convs[i] = genConversation(t)
			}
			orig := ListConversationsResponse{
				Code:          rapid.IntRange(1000, 9999).Draw(t, "code"),
				Conversations: convs,
			}
			assertJSONRoundTrip(t, orig)
		})
	})

	t.Run("ListMessagesResponse", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			n := rapid.IntRange(0, 10).Draw(t, "num_messages")
			msgs := make([]Message, n)
			for i := range msgs {
				msgs[i] = genMessage(t)
			}
			orig := ListMessagesResponse{
				Code:     rapid.IntRange(1000, 9999).Draw(t, "code"),
				Messages: msgs,
			}
			assertJSONRoundTrip(t, orig)
		})
	})
}

// --- Property 7 (lumo-crud): SpacePriv JSON round-trip ---

// TestSpacePriv_JSONRoundTrip_Property verifies that for any valid
// SpacePriv value, JSON marshal → unmarshal produces an equal value,
// and the JSON output uses camelCase keys.
//
// Feature: lumo-crud, Property 7: SpacePriv JSON round-trip
//
// **Validates: Requirements 6.5**
func TestSpacePriv_JSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genSpacePriv(t)

		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		// Verify camelCase keys — no PascalCase keys should appear.
		s := string(data)
		for _, bad := range []string{`"IsProject"`, `"ProjectName"`, `"ProjectInstructions"`, `"ProjectIcon"`, `"LinkedDriveFolder"`, `"FolderID"`, `"FolderName"`, `"FolderPath"`} {
			if strings.Contains(s, bad) {
				t.Fatalf("found PascalCase key %s in JSON: %s", bad, s)
			}
		}

		var got SpacePriv
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v\njson: %s", orig, got, s)
		}
	})
}
