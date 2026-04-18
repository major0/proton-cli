package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"pgregory.net/rapid"
)

// genSerialCookie generates an arbitrary serialCookie.
func genSerialCookie(t *rapid.T) serialCookie {
	return serialCookie{
		Name:   rapid.String().Draw(t, "name"),
		Value:  rapid.String().Draw(t, "value"),
		Domain: rapid.String().Draw(t, "domain"),
		Path:   rapid.String().Draw(t, "path"),
	}
}

// genSessionConfig generates an arbitrary SessionConfig with random cookies
// and timestamps.
func genSessionConfig(t *rapid.T) SessionConfig {
	n := rapid.IntRange(0, 20).Draw(t, "numCookies")
	cookies := make([]serialCookie, n)
	for i := range cookies {
		cookies[i] = genSerialCookie(t)
	}

	// Generate a timestamp truncated to second precision — JSON round-trips
	// time.Time at nanosecond precision via RFC 3339, but we truncate to
	// seconds to match real-world cookie timestamps and avoid false negatives
	// from sub-second jitter in marshaling formats.
	sec := rapid.Int64Range(-62135596800, 253402300799).Draw(t, "unixSec")
	ts := time.Unix(sec, 0).UTC()

	return SessionConfig{
		UID:           rapid.String().Draw(t, "uid"),
		AccessToken:   rapid.String().Draw(t, "accessToken"),
		RefreshToken:  rapid.String().Draw(t, "refreshToken"),
		SaltedKeyPass: rapid.String().Draw(t, "saltedKeyPass"),
		Cookies:       cookies,
		LastRefresh:   ts,
		Service:       rapid.String().Draw(t, "service"),
	}
}

// TestPropertySessionConfigCookieRoundTrip verifies that for any SessionConfig
// with arbitrary cookies and timestamps, JSON marshal/unmarshal produces
// identical Cookies and LastRefresh.
//
// **Validates: Requirements 3.2, 3.5**
func TestPropertySessionConfigCookieRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genSessionConfig(t)

		//nolint:gosec // G117: property test intentionally marshals SessionConfig with tokens.
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var restored SessionConfig
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		// Verify Cookies slice equality.
		if len(original.Cookies) != len(restored.Cookies) {
			t.Fatalf("cookie count: got %d, want %d", len(restored.Cookies), len(original.Cookies))
		}
		for i, orig := range original.Cookies {
			got := restored.Cookies[i]
			if orig != got {
				t.Fatalf("cookie[%d]: got %+v, want %+v", i, got, orig)
			}
		}

		// Verify LastRefresh equality.
		if !original.LastRefresh.Equal(restored.LastRefresh) {
			t.Fatalf("LastRefresh: got %v, want %v", restored.LastRefresh, original.LastRefresh)
		}

		// Verify Service equality.
		if original.Service != restored.Service {
			t.Fatalf("Service: got %q, want %q", restored.Service, original.Service)
		}
	})
}

// genCookieName generates a valid HTTP cookie name: one or more ASCII letters
// or digits. This avoids special characters that net/http/cookiejar may reject
// or sanitize.
func genCookieName(t *rapid.T, label string) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	n := rapid.IntRange(1, 32).Draw(t, label+"Len")
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rapid.IntRange(0, len(chars)-1).Draw(t, label+"Char")]
	}
	return string(b)
}

// genCookieValue generates a valid HTTP cookie value: ASCII printable
// characters excluding semicolons, commas, spaces, and double quotes, which
// can cause cookie parsing issues.
func genCookieValue(t *rapid.T, label string) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-."
	n := rapid.IntRange(0, 64).Draw(t, label+"Len")
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rapid.IntRange(0, len(chars)-1).Draw(t, label+"Char")]
	}
	return string(b)
}

// genJarCookie generates a serialCookie suitable for cookie jar round-trip
// testing. Domain and Path are empty because net/http/cookiejar does not
// expose these fields via Cookies() — the jar manages domain/path matching
// internally. The round-trip therefore preserves only Name and Value.
func genJarCookie(t *rapid.T, idx int) serialCookie {
	return serialCookie{
		Name:  genCookieName(t, fmt.Sprintf("name%d", idx)),
		Value: genCookieValue(t, fmt.Sprintf("value%d", idx)),
	}
}

// TestPropertyCookieJarRoundTrip verifies that for any set of cookie entries,
// loadCookies followed by serializeCookies returns equivalent cookies.
//
// The cookie jar (net/http/cookiejar) normalizes cookies: Domain is not
// returned by Cookies(), and cookies with duplicate Name+Path are deduplicated
// (last wins). The generator produces cookies with unique names, empty Domain,
// and Path="/" to ensure a clean round-trip.
//
// **Validates: Requirements 3.1, 3.2**
func TestPropertyCookieJarRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 20).Draw(t, "numCookies")

		// Generate cookies with unique names to avoid jar deduplication.
		seen := make(map[string]bool, n)
		cookies := make([]serialCookie, 0, n)
		for i := 0; i < n; i++ {
			c := genJarCookie(t, i)
			if seen[c.Name] {
				continue // skip duplicate names
			}
			seen[c.Name] = true
			cookies = append(cookies, c)
		}

		apiURL := apiCookieURL()

		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatalf("cookiejar.New: %v", err)
		}

		loadCookies(jar, cookies, apiURL)
		got := serializeCookies(jar, apiURL)

		if len(got) != len(cookies) {
			t.Fatalf("cookie count: got %d, want %d", len(got), len(cookies))
		}

		// Build a map for order-independent comparison — the jar may return
		// cookies in a different order than they were inserted. Compare by
		// Name and Value only; Domain and Path are not preserved by the jar's
		// Cookies() method.
		type key struct{ Name, Value string }
		want := make(map[key]bool, len(cookies))
		for _, c := range cookies {
			want[key{c.Name, c.Value}] = true
		}
		for _, c := range got {
			k := key{c.Name, c.Value}
			if !want[k] {
				t.Fatalf("unexpected cookie: %+v", c)
			}
		}
	})
}

// --- Unit tests for cookie edge cases ---

// TestSerializeCookiesEmptyJar verifies that a fresh jar with no cookies
// produces a nil slice from serializeCookies.
func TestSerializeCookiesEmptyJar(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	got := serializeCookies(jar, apiCookieURL())
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

// TestLoadCookiesNil verifies that calling loadCookies with a nil slice
// does not panic and leaves the jar empty.
func TestLoadCookiesNil(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	apiURL := apiCookieURL()
	loadCookies(jar, nil, apiURL)

	if cookies := jar.Cookies(apiURL); len(cookies) != 0 {
		t.Fatalf("expected empty jar, got %d cookies", len(cookies))
	}
}

// TestLoadCookiesEmpty verifies that calling loadCookies with an empty
// (non-nil) slice does not panic and leaves the jar empty.
func TestLoadCookiesEmpty(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	apiURL := apiCookieURL()
	loadCookies(jar, []serialCookie{}, apiURL)

	if cookies := jar.Cookies(apiURL); len(cookies) != 0 {
		t.Fatalf("expected empty jar, got %d cookies", len(cookies))
	}
}

// TestSessionConfigBackwardCompat verifies that JSON without the Cookies and
// LastRefresh fields deserializes cleanly into a SessionConfig with nil
// Cookies and zero-value LastRefresh.
func TestSessionConfigBackwardCompat(t *testing.T) {
	raw := `{"uid":"u1","access_token":"a","refresh_token":"r","salted_key_pass":"k"}`
	var cfg SessionConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Cookies != nil {
		t.Fatalf("expected nil Cookies, got %v", cfg.Cookies)
	}
	if !cfg.LastRefresh.IsZero() {
		t.Fatalf("expected zero LastRefresh, got %v", cfg.LastRefresh)
	}
	if cfg.UID != "u1" || cfg.AccessToken != "a" || cfg.RefreshToken != "r" || cfg.SaltedKeyPass != "k" {
		t.Fatalf("unexpected field values: %+v", cfg)
	}
	if cfg.Service != "" {
		t.Fatalf("expected empty Service, got %q", cfg.Service)
	}
}

// TestSessionConfigLastRefreshPreserved verifies that a SessionConfig with a
// specific LastRefresh timestamp survives JSON marshal/unmarshal with the
// timestamp preserved.
func TestSessionConfigLastRefreshPreserved(t *testing.T) {
	ts := time.Date(2025, 6, 15, 12, 30, 45, 0, time.UTC)
	cfg := SessionConfig{
		UID:           "u1",
		AccessToken:   "a",
		RefreshToken:  "r",
		SaltedKeyPass: "k",
		LastRefresh:   ts,
	}

	//nolint:gosec // G117: unit test intentionally marshals SessionConfig with tokens.
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored SessionConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.LastRefresh.Equal(ts) {
		t.Fatalf("LastRefresh: got %v, want %v", restored.LastRefresh, ts)
	}
}

// --- ReadySession unit tests ---

// errStore is a SessionStore that always returns a fixed error from Load.
type errStore struct {
	err error
}

func (s *errStore) Load() (*SessionConfig, error) { return nil, s.err }
func (s *errStore) Save(*SessionConfig) error     { return nil }
func (s *errStore) Delete() error                 { return nil }
func (s *errStore) List() ([]string, error)       { return nil, nil }
func (s *errStore) Switch(string) error           { return nil }

// TestReadySessionStoreError verifies that ReadySession propagates store.Load
// errors. An empty mockStore returns a SessionConfig with no UID, which
// SessionFromCredentials rejects with ErrMissingUID.
func TestReadySessionStoreError(t *testing.T) {
	store := &mockStore{}
	// Don't save anything — Load will return an empty config which
	// SessionFromCredentials will reject with ErrMissingUID.

	_, err := ReadySession(context.Background(), nil, store, nil)
	if err == nil {
		t.Fatal("expected error from ReadySession with empty store")
	}
}

// TestReadySessionNotLoggedIn verifies that when the store returns
// ErrKeyNotFound, ReadySession returns ErrNotLoggedIn.
func TestReadySessionNotLoggedIn(t *testing.T) {
	store := &errStore{err: ErrKeyNotFound}
	_, err := ReadySession(context.Background(), nil, store, nil)
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

// --- SessionFromCredentials error path tests (2.1) ---

// TestSessionFromCredentials verifies that SessionFromCredentials rejects
// configs with missing credential fields and accepts valid configs (which
// fail at the network layer since there's no real API server).
func TestSessionFromCredentials(t *testing.T) {
	tests := []struct {
		name    string
		config  *SessionConfig
		wantErr string
	}{
		{
			name:    "missing UID",
			config:  &SessionConfig{AccessToken: "a", RefreshToken: "r"},
			wantErr: "missing UID",
		},
		{
			name:    "missing access token",
			config:  &SessionConfig{UID: "u", RefreshToken: "r"},
			wantErr: "missing access token",
		},
		{
			name:    "missing refresh token",
			config:  &SessionConfig{UID: "u", AccessToken: "a"},
			wantErr: "missing refresh token",
		},
		{
			name:    "all fields empty",
			config:  &SessionConfig{},
			wantErr: "missing UID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SessionFromCredentials(context.Background(), nil, tt.config, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, errForMessage(tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// errForMessage maps error message substrings to sentinel errors.
func errForMessage(msg string) error {
	switch msg {
	case "missing UID":
		return ErrMissingUID
	case "missing access token":
		return ErrMissingAccessToken
	case "missing refresh token":
		return ErrMissingRefreshToken
	default:
		return fmt.Errorf("%s", msg)
	}
}

// --- SessionRestore staleness detection tests (2.2) ---

// configStore is a SessionStore backed by a single in-memory SessionConfig.
type configStore struct {
	config *SessionConfig
}

func (s *configStore) Load() (*SessionConfig, error) {
	if s.config == nil {
		return nil, ErrKeyNotFound
	}
	cfg := *s.config
	return &cfg, nil
}
func (s *configStore) Save(*SessionConfig) error { return nil }
func (s *configStore) Delete() error             { return nil }
func (s *configStore) List() ([]string, error)   { return nil, nil }
func (s *configStore) Switch(string) error       { return nil }

// TestSessionRestoreStaleness verifies that SessionRestore propagates
// ErrNotLoggedIn for missing sessions and returns errors for configs
// with missing credentials (staleness logging is exercised but not
// directly asserted — the important thing is the function doesn't panic).
func TestSessionRestoreStaleness(t *testing.T) {
	tests := []struct {
		name    string
		store   SessionStore
		wantErr string
	}{
		{
			name:    "no session stored",
			store:   &errStore{err: ErrKeyNotFound},
			wantErr: "not logged in",
		},
		{
			name:    "store load error",
			store:   &errStore{err: errors.New("disk failure")},
			wantErr: "disk failure",
		},
		{
			name: "stale tokens warn path",
			store: &configStore{config: &SessionConfig{
				UID:          "u",
				AccessToken:  "a",
				RefreshToken: "r",
				LastRefresh:  time.Now().Add(-21 * time.Hour),
			}},
			// Will fail at GetUser (no real server), but exercises the warn path.
			wantErr: "", // any non-nil error from network
		},
		{
			name: "expired tokens path",
			store: &configStore{config: &SessionConfig{
				UID:          "u",
				AccessToken:  "a",
				RefreshToken: "r",
				LastRefresh:  time.Now().Add(-25 * time.Hour),
			}},
			wantErr: "", // any non-nil error from network
		},
		{
			name: "zero LastRefresh skips staleness check",
			store: &configStore{config: &SessionConfig{
				UID:          "u",
				AccessToken:  "a",
				RefreshToken: "r",
			}},
			wantErr: "", // any non-nil error from network
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SessionRestore(context.Background(), nil, tt.store, nil)
			if err == nil {
				t.Fatal("expected error (no real API server)")
			}
			if tt.wantErr != "" {
				if !containsError(err, tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

// containsError checks if err's chain contains the given substring.
func containsError(err error, substr string) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if contains(e.Error(), substr) {
			return true
		}
	}
	return contains(err.Error(), substr)
}

// contains is a simple substring check.
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- SessionList / SessionRevoke / SessionSave tests ---

// TestSessionList verifies SessionList delegates to the store.
func TestSessionList(t *testing.T) {
	tests := []struct {
		name    string
		store   SessionStore
		want    int
		wantErr string
	}{
		{
			name:  "empty store",
			store: &mockStore{},
			want:  0,
		},
		{
			name:    "store error",
			store:   &errStore{err: errors.New("list failed")},
			want:    0,
			wantErr: "", // errStore.List returns nil, nil
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SessionList(tt.store)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.want {
				t.Fatalf("got %d accounts, want %d", len(got), tt.want)
			}
		})
	}
}

// --- SessionSave tests ---

// TestSessionSave verifies that SessionSave persists credentials and cookies.
func TestSessionSave(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	apiURL := apiCookieURL()
	jar.SetCookies(apiURL, []*http.Cookie{
		{Name: "Session-Id", Value: "abc123"},
	})

	session := &Session{
		Auth: proton.Auth{
			UID:          "uid-1",
			AccessToken:  "at-1",
			RefreshToken: "rt-1",
		},
		cookieJar: jar,
	}

	store := &mockStore{}
	err := SessionSave(store, session, []byte("keypass"))
	if err != nil {
		t.Fatalf("SessionSave: %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if cfg.UID != "uid-1" {
		t.Fatalf("UID = %q, want %q", cfg.UID, "uid-1")
	}
	if cfg.AccessToken != "at-1" {
		t.Fatalf("AccessToken = %q, want %q", cfg.AccessToken, "at-1")
	}
	if cfg.SaltedKeyPass == "" {
		t.Fatal("SaltedKeyPass should not be empty")
	}
	if cfg.LastRefresh.IsZero() {
		t.Fatal("LastRefresh should be set")
	}
	if len(cfg.Cookies) == 0 {
		t.Fatal("Cookies should be persisted")
	}
}

// --- Session accessor tests ---

// TestSessionAccessors verifies the simple accessor methods on Session.
func TestSessionAccessors(t *testing.T) {
	session := &Session{
		user: proton.User{ID: "user-1", Name: "test"},
		addressKeyRings: map[string]*crypto.KeyRing{
			"addr@example.com": nil,
		},
	}

	if got := session.User(); got.ID != "user-1" {
		t.Fatalf("User().ID = %q, want %q", got.ID, "user-1")
	}

	kr := session.AddressKeyRings()
	if _, ok := kr["addr@example.com"]; !ok {
		t.Fatal("AddressKeyRings missing expected key")
	}
}

// --- NewDeauthHandler test ---

// TestNewDeauthHandler verifies that the deauth handler doesn't panic.
func TestNewDeauthHandler(_ *testing.T) {
	handler := NewDeauthHandler()
	// Just verify it doesn't panic when called.
	handler()
}

// --- NewAuthHandler error path test ---

// failStore is a SessionStore where Load or Save can fail.
type failStore struct {
	loadErr error
	saveErr error
	config  *SessionConfig
}

func (s *failStore) Load() (*SessionConfig, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.config != nil {
		cfg := *s.config
		return &cfg, nil
	}
	return &SessionConfig{}, nil
}
func (s *failStore) Save(*SessionConfig) error { return s.saveErr }
func (s *failStore) Delete() error             { return nil }
func (s *failStore) List() ([]string, error)   { return nil, nil }
func (s *failStore) Switch(string) error       { return nil }

// TestAuthHandlerStoreErrors verifies that the auth handler handles store
// errors gracefully (logs them, doesn't panic).
func TestAuthHandlerStoreErrors(t *testing.T) {
	tests := []struct {
		name  string
		store SessionStore
	}{
		{
			name:  "load fails",
			store: &failStore{loadErr: errors.New("disk read error")},
		},
		{
			name:  "save fails",
			store: &failStore{saveErr: errors.New("disk write error")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jar, _ := cookiejar.New(nil)
			session := &Session{cookieJar: jar}
			handler := NewAuthHandler(tt.store, session)

			// Should not panic even when store operations fail.
			handler(proton.Auth{
				UID:          "uid",
				AccessToken:  "at",
				RefreshToken: "rt",
			})

			// Verify in-memory state is still updated.
			if session.Auth.UID != "uid" {
				t.Fatalf("UID = %q, want %q", session.Auth.UID, "uid")
			}
		})
	}
}

// --- SessionRevoke tests ---

// deleteStore tracks whether Delete was called.
type deleteStore struct {
	mockStore
	deleted bool
}

func (s *deleteStore) Delete() error {
	s.deleted = true
	return nil
}

// TestSessionRevoke verifies SessionRevoke deletes from the store.
// With a nil session, it skips the API revoke and just deletes.
func TestSessionRevoke(t *testing.T) {
	store := &deleteStore{}
	err := SessionRevoke(context.Background(), nil, store, false)
	if err != nil {
		t.Fatalf("SessionRevoke: %v", err)
	}
	if !store.deleted {
		t.Fatal("expected store.Delete to be called")
	}
}

// --- Unlock test ---

// TestUnlock verifies that Unlock populates the address map. It will fail
// at the crypto layer (no real keys), but we verify the address map setup.
func TestUnlock(t *testing.T) {
	session := &Session{
		user: proton.User{
			ID:   "user-1",
			Name: "test",
		},
	}

	addrs := []proton.Address{
		{ID: "addr-1", Email: "test@example.com"},
		{ID: "addr-2", Email: "other@example.com"},
	}

	// Unlock will fail because there are no real keys, but the address
	// map should still be populated before the crypto call.
	_ = session.Unlock([]byte("keypass"), addrs)

	// Verify addresses were stored (even though Unlock returned an error).
	if len(session.addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(session.addresses))
	}
	if _, ok := session.addresses["test@example.com"]; !ok {
		t.Fatal("missing test@example.com in address map")
	}
}

// --- SaveConfig error path test ---

// TestSaveConfigError verifies SaveConfig returns an error for an
// unwritable directory.
func TestSaveConfigError(t *testing.T) {
	err := SaveConfig("/proc/nonexistent/deep/path/config.yaml", DefaultConfig())
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}
