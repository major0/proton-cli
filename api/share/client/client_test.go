package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/api/share"
	shareclient "github.com/major0/proton-cli/api/share/client"
)

// newTestClient creates a share client backed by a mock HTTP server.
func newTestClient(t *testing.T, handler http.Handler) (*shareclient.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	session := &api.Session{
		Auth:    proton.Auth{UID: "test-uid", AccessToken: "test-token"},
		BaseURL: srv.URL,
	}
	return shareclient.NewClient(session), srv
}

func TestListMembers(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/drive/v2/shares/share-1/members") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(share.MembersResponse{
			Code: 1000,
			Members: []share.Member{
				{MemberID: "m1", Email: "alice@example.com", Permissions: share.PermViewer},
			},
		})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	members, err := c.ListMembers(context.Background(), "share-1")
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 || members[0].MemberID != "m1" {
		t.Fatalf("unexpected members: %+v", members)
	}
}

func TestRemoveMember(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/drive/v2/shares/share-1/members/m1") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]int{"Code": 1000})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := c.RemoveMember(context.Background(), "share-1", "m1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
}

func TestListInvitations(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/drive/v2/shares/share-1/invitations") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(share.InvitationsResponse{
			Code: 1000,
			Invitations: []share.Invitation{
				{InvitationID: "inv-1", InviteeEmail: "bob@example.com", Permissions: share.PermEditor},
			},
		})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	invs, err := c.ListInvitations(context.Background(), "share-1")
	if err != nil {
		t.Fatalf("ListInvitations: %v", err)
	}
	if len(invs) != 1 || invs[0].InvitationID != "inv-1" {
		t.Fatalf("unexpected invitations: %+v", invs)
	}
}

func TestInviteProtonUser(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "bob@example.com") {
			t.Fatalf("body missing invitee email: %s", body)
		}
		json.NewEncoder(w).Encode(map[string]int{"Code": 1000})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	var payload share.InviteProtonUserPayload
	payload.Invitation.InviteeEmail = "bob@example.com"
	payload.Invitation.Permissions = share.PermViewer

	if err := c.InviteProtonUser(context.Background(), "share-1", payload); err != nil {
		t.Fatalf("InviteProtonUser: %v", err)
	}
}

func TestDeleteInvitation(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/drive/v2/shares/share-1/invitations/inv-1") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]int{"Code": 1000})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := c.DeleteInvitation(context.Background(), "share-1", "inv-1"); err != nil {
		t.Fatalf("DeleteInvitation: %v", err)
	}
}

func TestListExternalInvitations(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/drive/v2/shares/share-1/external-invitations") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(share.ExternalInvitationsResponse{
			Code: 1000,
			ExternalInvitations: []share.ExternalInvitation{
				{ExternalInvitationID: "ext-1", InviteeEmail: "ext@gmail.com"},
			},
		})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	exts, err := c.ListExternalInvitations(context.Background(), "share-1")
	if err != nil {
		t.Fatalf("ListExternalInvitations: %v", err)
	}
	if len(exts) != 1 || exts[0].ExternalInvitationID != "ext-1" {
		t.Fatalf("unexpected external invitations: %+v", exts)
	}
}

func TestDeleteExternalInvitation(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/drive/v2/shares/share-1/external-invitations/ext-1") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]int{"Code": 1000})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	if err := c.DeleteExternalInvitation(context.Background(), "share-1", "ext-1"); err != nil {
		t.Fatalf("DeleteExternalInvitation: %v", err)
	}
}

func TestAPIErrorPropagation(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"Code":  2501,
			"Error": "Share not found",
		})
	})
	c, srv := newTestClient(t, h)
	defer srv.Close()

	_, err := c.ListMembers(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "2501") {
		t.Fatalf("error should contain API code: %v", err)
	}
}
