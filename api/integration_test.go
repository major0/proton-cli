package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	proton "github.com/ProtonMail/go-proton-api"
)

// --- Integration test helpers ---

// testSessionIndex is a minimal in-memory SessionStore for integration tests.
type testSessionIndex struct {
	configs map[string]*SessionConfig // service → config
}

func newTestSessionIndex() *testSessionIndex {
	return &testSessionIndex{configs: make(map[string]*SessionConfig)}
}

func (s *testSessionIndex) Load() (*SessionConfig, error) {
	// Return the first config found (single-service store).
	for _, cfg := range s.configs {
		c := *cfg
		return &c, nil
	}
	return nil, ErrKeyNotFound
}

func (s *testSessionIndex) Save(cfg *SessionConfig) error {
	svc := cfg.Service
	if svc == "" {
		svc = "default"
	}
	s.configs[svc] = cfg
	return nil
}

func (s *testSessionIndex) Delete() error           { return nil }
func (s *testSessionIndex) List() ([]string, error) { return nil, nil }
func (s *testSessionIndex) Switch(string) error     { return nil }

func (s *testSessionIndex) SetConfig(svc string, cfg *SessionConfig) {
	s.configs[svc] = cfg
}

// --- Integration test: full fork flow with mock endpoints ---

// TestIntegration_FullForkFlow verifies the complete fork flow:
// push fork request → receive selector → pull fork → build child session.
func TestIntegration_FullForkFlow(t *testing.T) {
	// The push endpoint captures the payload sent by ForkSessionWithKeyPass.
	// The pull endpoint returns it back — simulating the server storing and
	// returning the fork payload.
	var capturedPayload string

	// Mock push endpoint (parent host).
	var pushReceived ForkPushReq
	pushSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		_ = json.Unmarshal(body, &pushReceived)
		capturedPayload = pushReceived.Payload
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Code":     1000,
			"Selector": "fork-selector-xyz",
		})
	}))
	defer pushSrv.Close()

	// Mock pull endpoint (child host) — returns the captured payload.
	pullSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Code":         1000,
			"UID":          "child-uid",
			"AccessToken":  "child-at",
			"RefreshToken": "child-rt",
			"Payload":      capturedPayload,
		})
	}))
	defer pullSrv.Close()

	jar, _ := cookiejar.New(nil)
	parent := &Session{
		Auth: proton.Auth{
			UID:         "parent-uid",
			AccessToken: "parent-at",
		},
		BaseURL:   pushSrv.URL,
		cookieJar: jar,
	}

	targetSvc := ServiceConfig{
		Name:     "lumo",
		Host:     pullSrv.URL,
		ClientID: "web-lumo",
	}

	// Execute the fork.
	child, childKeyPass, err := ForkSessionWithKeyPass(
		context.Background(), parent, targetSvc, DefaultVersion,
		[]byte("test-salted-key-pass"),
	)
	if err != nil {
		t.Fatalf("ForkSessionWithKeyPass: %v", err)
	}
	defer child.Stop()

	// Verify push request.
	if pushReceived.ChildClientID != "web-lumo" {
		t.Errorf("push ChildClientID = %q, want %q", pushReceived.ChildClientID, "web-lumo")
	}
	if pushReceived.Independent != 0 {
		t.Errorf("push Independent = %d, want 0", pushReceived.Independent)
	}

	// Verify child session.
	if child.Auth.UID != "child-uid" {
		t.Errorf("child UID = %q, want %q", child.Auth.UID, "child-uid")
	}
	if child.Auth.AccessToken != "child-at" {
		t.Errorf("child AccessToken = %q, want %q", child.Auth.AccessToken, "child-at")
	}
	if child.BaseURL != pullSrv.URL {
		t.Errorf("child BaseURL = %q, want %q", child.BaseURL, pullSrv.URL)
	}

	// Verify decrypted key pass matches what we sent.
	if string(childKeyPass) != "test-salted-key-pass" {
		t.Errorf("childKeyPass = %q, want %q", string(childKeyPass), "test-salted-key-pass")
	}
}
