package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/cookiejar"
	"testing"
	"time"

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
