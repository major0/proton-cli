package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api/lumo"
)

// testCryptoChain sets up a master key and returns the raw key bytes
// and the PGP-armored version for mock server responses.
type testCryptoChain struct {
	masterKey    []byte
	armored      string
	kr           *pgpcrypto.KeyRing
	lastSpaceKey []byte
}

func newTestCryptoChain(t *testing.T) *testCryptoChain {
	t.Helper()
	_, kr := testKeyPair(t)
	mk := make([]byte, 32)
	for i := range mk {
		mk[i] = byte(i + 1)
	}
	return &testCryptoChain{
		masterKey: mk,
		armored:   pgpEncrypt(t, kr, mk),
		kr:        kr,
	}
}

// masterKeyHandler returns an HTTP handler that serves the master key.
func (tc *testCryptoChain) masterKeyHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, lumo.ListMasterKeysResponse{
			Code:        1000,
			Eligibility: 1,
			MasterKeys: []lumo.MasterKeyEntry{
				{ID: "mk1", IsLatest: true, Version: 1, CreateTime: "2024-01-01T00:00:00Z", MasterKey: tc.armored},
			},
		})
	}
}

// makeEncryptedSpace creates a Space with properly encrypted metadata
// using the test crypto chain.
func (tc *testCryptoChain) makeEncryptedSpace(t *testing.T, id, tag string, isProject bool) lumo.Space {
	t.Helper()
	spaceKey, err := GenerateSpaceKey()
	if err != nil {
		t.Fatalf("generate space key: %v", err)
	}
	wrapped, err := lumo.WrapSpaceKey(tc.masterKey, spaceKey)
	if err != nil {
		t.Fatalf("wrap space key: %v", err)
	}
	tc.lastSpaceKey = spaceKey
	dek, err := lumo.DeriveDataEncryptionKey(spaceKey)
	if err != nil {
		t.Fatalf("derive DEK: %v", err)
	}
	priv := lumo.SpacePriv{IsProject: &isProject}
	privJSON, _ := json.Marshal(priv)
	ad := lumo.SpaceAD(tag)
	encrypted, err := lumo.EncryptString(string(privJSON), dek, ad)
	if err != nil {
		t.Fatalf("encrypt space priv: %v", err)
	}
	return lumo.Space{
		ID:         id,
		SpaceKey:   base64.StdEncoding.EncodeToString(wrapped),
		SpaceTag:   tag,
		Encrypted:  encrypted,
		CreateTime: "2024-01-01T00:00:00Z",
	}
}

// deriveSpaceDEK derives the DEK from the last space key created by
// makeEncryptedSpace.
func (tc *testCryptoChain) deriveSpaceDEK(t *testing.T) ([]byte, error) {
	t.Helper()
	return lumo.DeriveDataEncryptionKey(tc.lastSpaceKey)
}

func TestListSpaces_HappyPath(t *testing.T) {
	spaces := []lumo.Space{
		{ID: "s1", SpaceTag: "tag1", CreateTime: "2024-01-01T00:00:00Z"},
		{ID: "s2", SpaceTag: "tag2", CreateTime: "2024-01-02T00:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, lumo.ListSpacesResponse{Code: 1000, Spaces: spaces})
	}))
	defer srv.Close()

	sess := testSession(t)
	c := NewClient(sess)
	c.BaseURL = srv.URL + "/api"

	got, err := c.ListSpaces(context.Background())
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d spaces, want 2", len(got))
	}
	if got[0].ID != "s1" || got[1].ID != "s2" {
		t.Fatalf("unexpected space IDs: %s, %s", got[0].ID, got[1].ID)
	}
}

func TestCreateSpace_RequestBody(t *testing.T) {
	tc := newTestCryptoChain(t)

	var capturedReq lumo.CreateSpaceReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/lumo/v1/masterkeys":
			tc.masterKeyHandler(t)(w, r)
		case "/api/lumo/v1/spaces":
			if err := json.NewDecoder(r.Body).Decode(&capturedReq); err != nil {
				t.Errorf("decode request: %v", err)
			}
			writeJSON(t, w, lumo.GetSpaceResponse{
				Code: 1000,
				Space: lumo.Space{
					ID:         "new-space-id",
					SpaceKey:   capturedReq.SpaceKey,
					SpaceTag:   capturedReq.SpaceTag,
					Encrypted:  capturedReq.Encrypted,
					CreateTime: "2024-01-01T00:00:00Z",
				},
			})
		}
	}))
	defer srv.Close()

	sess := testSession(t)
	sess.UserKeyRing = tc.kr
	c := NewClient(sess)
	c.BaseURL = srv.URL + "/api"

	space, err := c.CreateSpace(context.Background(), "My Space", false)
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}

	// Verify the space was created with proper fields.
	if space.ID != "new-space-id" {
		t.Fatalf("space ID = %q, want %q", space.ID, "new-space-id")
	}
	if capturedReq.SpaceKey == "" {
		t.Fatal("SpaceKey is empty")
	}
	if capturedReq.SpaceTag == "" {
		t.Fatal("SpaceTag is empty")
	}
	if capturedReq.Encrypted == "" {
		t.Fatal("Encrypted is empty")
	}

	// Verify we can decrypt the metadata.
	wrappedKey, _ := base64.StdEncoding.DecodeString(capturedReq.SpaceKey)
	spaceKey, err := lumo.UnwrapSpaceKey(tc.masterKey, wrappedKey)
	if err != nil {
		t.Fatalf("unwrap space key: %v", err)
	}
	dek, err := lumo.DeriveDataEncryptionKey(spaceKey)
	if err != nil {
		t.Fatalf("derive DEK: %v", err)
	}
	ad := lumo.SpaceAD(capturedReq.SpaceTag)
	privJSON, err := lumo.DecryptString(capturedReq.Encrypted, dek, ad)
	if err != nil {
		t.Fatalf("decrypt metadata: %v", err)
	}
	var priv lumo.SpacePriv
	if err := json.Unmarshal([]byte(privJSON), &priv); err != nil {
		t.Fatalf("unmarshal priv: %v", err)
	}
	if priv.IsProject == nil || *priv.IsProject != false {
		t.Fatalf("IsProject = %v, want *false", priv.IsProject)
	}
}

func TestGetDefaultSpace_FindsSimple(t *testing.T) {
	tc := newTestCryptoChain(t)

	projectSpace := tc.makeEncryptedSpace(t, "s-project", "tag-project", true)
	simpleSpace := tc.makeEncryptedSpace(t, "s-simple", "tag-simple", false)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/lumo/v1/masterkeys":
			tc.masterKeyHandler(t)(w, r)
		case "/api/lumo/v1/spaces":
			writeJSON(t, w, lumo.ListSpacesResponse{
				Code:   1000,
				Spaces: []lumo.Space{projectSpace, simpleSpace},
			})
		}
	}))
	defer srv.Close()

	sess := testSession(t)
	sess.UserKeyRing = tc.kr
	c := NewClient(sess)
	c.BaseURL = srv.URL + "/api"

	got, err := c.GetDefaultSpace(context.Background())
	if err != nil {
		t.Fatalf("GetDefaultSpace: %v", err)
	}
	if got.ID != "s-simple" {
		t.Fatalf("got space ID %q, want %q", got.ID, "s-simple")
	}
}

func TestGetDefaultSpace_CreatesWhenNone(t *testing.T) {
	tc := newTestCryptoChain(t)

	projectSpace := tc.makeEncryptedSpace(t, "s-project", "tag-project", true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/lumo/v1/masterkeys":
			tc.masterKeyHandler(t)(w, r)
		case r.URL.Path == "/api/lumo/v1/spaces" && r.Method == "GET":
			writeJSON(t, w, lumo.ListSpacesResponse{
				Code:   1000,
				Spaces: []lumo.Space{projectSpace},
			})
		case r.URL.Path == "/api/lumo/v1/spaces" && r.Method == "POST":
			var req lumo.CreateSpaceReq
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode: %v", err)
			}
			writeJSON(t, w, lumo.GetSpaceResponse{
				Code: 1000,
				Space: lumo.Space{
					ID:         "new-default",
					SpaceKey:   req.SpaceKey,
					SpaceTag:   req.SpaceTag,
					Encrypted:  req.Encrypted,
					CreateTime: "2024-01-01T00:00:00Z",
				},
			})
		}
	}))
	defer srv.Close()

	sess := testSession(t)
	sess.UserKeyRing = tc.kr
	c := NewClient(sess)
	c.BaseURL = srv.URL + "/api"

	got, err := c.GetDefaultSpace(context.Background())
	if err != nil {
		t.Fatalf("GetDefaultSpace: %v", err)
	}
	if got.ID != "new-default" {
		t.Fatalf("got space ID %q, want %q", got.ID, "new-default")
	}
}

func TestDeleteSpace_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJSON(t, w, map[string]any{"Code": 2501, "Error": "resource deleted"})
	}))
	defer srv.Close()

	sess := testSession(t)
	c := NewClient(sess)
	c.BaseURL = srv.URL + "/api"

	err := c.DeleteSpace(context.Background(), "deleted-id")
	if !errors.Is(err, lumo.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestCreateSpace_Conflict(t *testing.T) {
	tc := newTestCryptoChain(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/lumo/v1/masterkeys":
			tc.masterKeyHandler(t)(w, r)
		case "/api/lumo/v1/spaces":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			writeJSON(t, w, map[string]any{"Code": 2000, "Error": "duplicate"})
		}
	}))
	defer srv.Close()

	sess := testSession(t)
	sess.UserKeyRing = tc.kr
	c := NewClient(sess)
	c.BaseURL = srv.URL + "/api"

	_, err := c.CreateSpace(context.Background(), "dup", false)
	if !errors.Is(err, lumo.ErrConflict) {
		t.Fatalf("expected ErrConflict, got: %v", err)
	}
}
