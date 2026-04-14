package shareCmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/major0/proton-cli/api/drive"
	"pgregory.net/rapid"
)

// genDistinctIDs generates n distinct non-empty string IDs.
func genDistinctIDs(t *rapid.T, n int, label string) []string {
	seen := make(map[string]bool, n)
	ids := make([]string, 0, n)
	for len(ids) < n {
		id := fmt.Sprintf("%s-%d-%s", label, len(ids), rapid.StringMatching(`[a-z0-9]{4,8}`).Draw(t, label))
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// TestFindRevokeTarget_UniqueMatch_Property verifies that when a search
// argument matches exactly one entity, findRevokeTarget returns it.
//
// **Property 3: Revoke Entity Lookup**
// **Validates: Requirements 4.1, 4.5**
func TestFindRevokeTarget_UniqueMatch_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nMembers := rapid.IntRange(0, 5).Draw(t, "nMembers")
		nInvs := rapid.IntRange(0, 5).Draw(t, "nInvs")
		nExts := rapid.IntRange(0, 5).Draw(t, "nExts")
		total := nMembers + nInvs + nExts
		if total == 0 {
			return // skip empty case
		}

		// Generate distinct IDs and emails for all entities.
		allIDs := genDistinctIDs(t, total, "id")
		allEmails := genDistinctIDs(t, total, "email")

		idx := 0
		members := make([]drive.Member, nMembers)
		for i := range members {
			members[i] = drive.Member{MemberID: allIDs[idx], Email: allEmails[idx]}
			idx++
		}
		invs := make([]drive.Invitation, nInvs)
		for i := range invs {
			invs[i] = drive.Invitation{InvitationID: allIDs[idx], InviteeEmail: allEmails[idx]}
			idx++
		}
		exts := make([]drive.ExternalInvitation, nExts)
		for i := range exts {
			exts[i] = drive.ExternalInvitation{ExternalInvitationID: allIDs[idx], InviteeEmail: allEmails[idx]}
			idx++
		}

		// Pick a random entity and search by its email.
		pickIdx := rapid.IntRange(0, total-1).Draw(t, "pickIdx")
		var searchArg, wantKind, wantID string
		if pickIdx < nMembers {
			searchArg = members[pickIdx].Email
			wantKind = "member"
			wantID = members[pickIdx].MemberID
		} else if pickIdx < nMembers+nInvs {
			i := pickIdx - nMembers
			searchArg = invs[i].InviteeEmail
			wantKind = "invitation"
			wantID = invs[i].InvitationID
		} else {
			i := pickIdx - nMembers - nInvs
			searchArg = exts[i].InviteeEmail
			wantKind = "external-invitation"
			wantID = exts[i].ExternalInvitationID
		}

		got, err := findRevokeTarget(searchArg, members, invs, exts)
		if err != nil {
			t.Fatalf("findRevokeTarget(%q): %v", searchArg, err)
		}
		if got.kind != wantKind {
			t.Fatalf("kind = %q, want %q", got.kind, wantKind)
		}
		if got.id != wantID {
			t.Fatalf("id = %q, want %q", got.id, wantID)
		}
	})
}

// TestFindRevokeTarget_Ambiguous_Property verifies that when a search
// argument matches entities in multiple lists, an ambiguity error is returned.
func TestFindRevokeTarget_Ambiguous_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Create a member and an invitation with the same email.
		sharedEmail := fmt.Sprintf("shared-%s@test.local", rapid.StringMatching(`[a-z]{4}`).Draw(t, "email"))

		members := []drive.Member{{MemberID: "m1", Email: sharedEmail}}
		invs := []drive.Invitation{{InvitationID: "inv1", InviteeEmail: sharedEmail}}

		_, err := findRevokeTarget(sharedEmail, members, invs, nil)
		if err == nil {
			t.Fatal("expected ambiguity error, got nil")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected ambiguity error, got: %v", err)
		}
	})
}

// TestFindRevokeTarget_NoMatch verifies that a non-matching argument
// returns a "no matching" error.
func TestFindRevokeTarget_NoMatch(t *testing.T) {
	members := []drive.Member{{MemberID: "m1", Email: "alice@test.local"}}
	_, err := findRevokeTarget("nobody@test.local", members, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no matching") {
		t.Fatalf("expected 'no matching' error, got: %v", err)
	}
}

// TestFindRevokeTarget_ByID verifies lookup by ID (not email).
func TestFindRevokeTarget_ByID(t *testing.T) {
	members := []drive.Member{{MemberID: "m1", Email: "alice@test.local"}}
	got, err := findRevokeTarget("m1", members, nil, nil)
	if err != nil {
		t.Fatalf("findRevokeTarget by ID: %v", err)
	}
	if got.kind != "member" || got.id != "m1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}
