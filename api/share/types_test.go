package share

import (
	"encoding/json"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

func genMember(t *rapid.T) Member {
	return Member{
		MemberID:            rapid.String().Draw(t, "MemberID"),
		Email:               rapid.String().Draw(t, "Email"),
		InviterEmail:        rapid.String().Draw(t, "InviterEmail"),
		AddressID:           rapid.String().Draw(t, "AddressID"),
		CreateTime:          rapid.Int64().Draw(t, "CreateTime"),
		ModifyTime:          rapid.Int64().Draw(t, "ModifyTime"),
		Permissions:         rapid.Int().Draw(t, "Permissions"),
		KeyPacketSignature:  rapid.String().Draw(t, "KeyPacketSignature"),
		SessionKeySignature: rapid.String().Draw(t, "SessionKeySignature"),
	}
}

func genInvitation(t *rapid.T) Invitation {
	return Invitation{
		InvitationID:       rapid.String().Draw(t, "InvitationID"),
		InviterEmail:       rapid.String().Draw(t, "InviterEmail"),
		InviteeEmail:       rapid.String().Draw(t, "InviteeEmail"),
		Permissions:        rapid.Int().Draw(t, "Permissions"),
		KeyPacket:          rapid.String().Draw(t, "KeyPacket"),
		KeyPacketSignature: rapid.String().Draw(t, "KeyPacketSignature"),
		CreateTime:         rapid.Int64().Draw(t, "CreateTime"),
		State:              rapid.Int().Draw(t, "State"),
	}
}

func genExternalInvitation(t *rapid.T) ExternalInvitation {
	return ExternalInvitation{
		ExternalInvitationID:        rapid.String().Draw(t, "ExternalInvitationID"),
		InviterEmail:                rapid.String().Draw(t, "InviterEmail"),
		InviteeEmail:                rapid.String().Draw(t, "InviteeEmail"),
		CreateTime:                  rapid.Int64().Draw(t, "CreateTime"),
		Permissions:                 rapid.Int().Draw(t, "Permissions"),
		State:                       rapid.Int().Draw(t, "State"),
		ExternalInvitationSignature: rapid.String().Draw(t, "ExternalInvitationSignature"),
	}
}

// TestMemberJSONRoundTrip_Property verifies that Member survives JSON
// marshal/unmarshal without data loss.
//
// **Property 2: Share Type JSON Round-Trip**
// **Validates: Requirements 1.4**
func TestMemberJSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genMember(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got Member
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

func TestInvitationJSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genInvitation(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got Invitation
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

func TestExternalInvitationJSONRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genExternalInvitation(t)
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got ExternalInvitation
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

func TestFormatPermissions(t *testing.T) {
	tests := []struct {
		perm int
		want string
	}{
		{PermViewer, "viewer"},
		{PermEditor, "editor"},
		{PermAdmin | PermRead, "admin"},
		{0, "unknown"},
		{1, "unknown"},
	}
	for _, tt := range tests {
		got := FormatPermissions(tt.perm)
		if got != tt.want {
			t.Errorf("FormatPermissions(%d) = %q, want %q", tt.perm, got, tt.want)
		}
	}
}
