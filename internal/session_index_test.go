package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/major0/proton-cli/proton"
)

// randomString generates a non-empty alphanumeric string for use in quick.Check generators.
func randomString(r *rand.Rand, maxLen int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	n := r.Intn(maxLen) + 1
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}

// sessionConfigGenerator produces random SessionConfig values for property tests.
type sessionConfigGenerator struct{}

func (sessionConfigGenerator) Generate(r *rand.Rand, _ int) reflect.Value {
	cfg := proton.SessionConfig{
		UID:           randomString(r, 32),
		AccessToken:   randomString(r, 64),
		RefreshToken:  randomString(r, 64),
		SaltedKeyPass: randomString(r, 64),
	}
	return reflect.ValueOf(cfg)
}

// nonEmptyStringGenerator produces non-empty alphanumeric strings.
type nonEmptyStringGenerator struct{}

func (nonEmptyStringGenerator) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(randomString(r, 32))
}

// PropertySaveLoadRoundTrip verifies that for any valid SessionConfig, account
// name, and service name, saving then loading with the same (account, service)
// returns an equal config.
//
// **Validates: Requirements 9.3**
func TestPropertySaveLoadRoundTrip(t *testing.T) {
	cfg := &quick.Config{
		MaxCount: 100,
		Values: func(values []reflect.Value, r *rand.Rand) {
			values[0] = sessionConfigGenerator{}.Generate(r, 0)
			values[1] = nonEmptyStringGenerator{}.Generate(r, 0)
			values[2] = nonEmptyStringGenerator{}.Generate(r, 0)
		},
	}

	prop := func(session proton.SessionConfig, account string, service string) bool {
		dir := t.TempDir()
		indexPath := filepath.Join(dir, "sessions.json")
		kr := NewMockKeyring()

		store := NewSessionStore(indexPath, account, service, kr)
		if err := store.Save(&session); err != nil {
			t.Logf("Save failed: %v", err)
			return false
		}

		loaded, err := store.Load()
		if err != nil {
			t.Logf("Load failed: %v", err)
			return false
		}

		return reflect.DeepEqual(session, *loaded)
	}

	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 9 failed: %v", err)
	}
}

// nonStarStringGenerator produces non-empty alphanumeric strings that are never "*".
type nonStarStringGenerator struct{}

func (nonStarStringGenerator) Generate(r *rand.Rand, _ int) reflect.Value {
	s := randomString(r, 32)
	// Ensure the generated string is never "*".
	if s == "*" {
		s = "fallback"
	}
	return reflect.ValueOf(s)
}

// TestPropertyServiceFallbackToWildcard verifies that for any account with a
// wildcard ("*") session and no service-specific session for service S, loading
// with (account, S) returns the wildcard session's config.
//
// **Validates: Requirements 9.3**
func TestPropertyServiceFallbackToWildcard(t *testing.T) {
	cfg := &quick.Config{
		MaxCount: 100,
		Values: func(values []reflect.Value, r *rand.Rand) {
			values[0] = sessionConfigGenerator{}.Generate(r, 0)
			values[1] = nonEmptyStringGenerator{}.Generate(r, 0) // account
			values[2] = nonStarStringGenerator{}.Generate(r, 0)  // service (never "*")
		},
	}

	prop := func(session proton.SessionConfig, account string, service string) bool {
		dir := t.TempDir()
		indexPath := filepath.Join(dir, "sessions.json")
		kr := NewMockKeyring()

		// Save session under the wildcard service "*".
		wildcardStore := NewSessionStore(indexPath, account, "*", kr)
		if err := wildcardStore.Save(&session); err != nil {
			t.Logf("Save wildcard failed: %v", err)
			return false
		}

		// Load with a different, non-wildcard service name — should fall back to "*".
		serviceStore := NewSessionStore(indexPath, account, service, kr)
		loaded, err := serviceStore.Load()
		if err != nil {
			t.Logf("Load with service %q failed: %v", service, err)
			return false
		}

		return reflect.DeepEqual(session, *loaded)
	}

	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 10 failed: %v", err)
	}
}

// TestSessionIndex_MissingIndexFile verifies that loading from a non-existent
// index file returns ErrKeyNotFound (empty index, no account).
//
// Requirements: 9.3, 9.4
func TestSessionIndex_MissingIndexFile(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "nonexistent", "sessions.json")
	kr := NewMockKeyring()

	store := NewSessionStore(indexPath, "alice", "drive", kr)
	_, err := store.Load()
	if err == nil {
		t.Fatal("Load on missing index file: expected error, got nil")
	}
	if !errors.Is(err, proton.ErrKeyNotFound) {
		t.Errorf("Load error = %v, want wrapped proton.ErrKeyNotFound", err)
	}
}

// TestSessionIndex_CorruptIndexFile verifies that loading from an index file
// containing invalid JSON returns an error (not silently discarded).
//
// Requirements: 9.3, 9.4
func TestSessionIndex_CorruptIndexFile(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "sessions.json")

	if err := os.WriteFile(indexPath, []byte("{not valid json!!!"), 0600); err != nil {
		t.Fatalf("write corrupt index: %v", err)
	}

	kr := NewMockKeyring()
	store := NewSessionStore(indexPath, "alice", "drive", kr)

	_, err := store.Load()
	if err == nil {
		t.Fatal("Load on corrupt index file: expected error, got nil")
	}
	// The error should mention the account/service context.
	if !strings.Contains(err.Error(), "alice") || !strings.Contains(err.Error(), "drive") {
		t.Errorf("error lacks context: %v", err)
	}
}

// TestSessionIndex_KeyringUnavailable verifies that when the keyring backend
// returns an error, Load surfaces a clear error after a successful Save.
//
// Requirements: 9.3, 9.4
func TestSessionIndex_KeyringUnavailable(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "sessions.json")
	kr := NewMockKeyring()

	store := NewSessionStore(indexPath, "alice", "drive", kr)
	session := &proton.SessionConfig{
		UID:           "uid-1",
		AccessToken:   "at-1",
		RefreshToken:  "rt-1",
		SaltedKeyPass: "skp-1",
	}

	if err := store.Save(session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Simulate keyring becoming unavailable after save.
	kr.ErrGet = fmt.Errorf("keyring unavailable: no secret service provider")

	_, err := store.Load()
	if err == nil {
		t.Fatal("Load with unavailable keyring: expected error, got nil")
	}
	if !errors.Is(err, proton.ErrKeyNotFound) {
		t.Errorf("Load error = %v, want wrapped proton.ErrKeyNotFound", err)
	}
}

// TestSessionIndex_StaleEntryCleanup verifies that when a keyring entry is
// deleted externally, Load returns ErrKeyNotFound and removes the stale
// entry from the on-disk index.
//
// Requirements: 9.3, 9.4
func TestSessionIndex_StaleEntryCleanup(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "sessions.json")
	kr := NewMockKeyring()

	store := NewSessionStore(indexPath, "alice", "drive", kr)
	session := &proton.SessionConfig{
		UID:           "uid-1",
		AccessToken:   "at-1",
		RefreshToken:  "rt-1",
		SaltedKeyPass: "skp-1",
	}

	if err := store.Save(session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Delete the keyring entry directly, simulating external removal.
	// Read the index to find the UUID, then delete it from the mock keyring.
	idxData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var idx SessionIndexData
	if err := json.Unmarshal(idxData, &idx); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}
	uuid := idx.Accounts["alice"].Sessions["drive"]
	if err := kr.Delete(keyringService, uuid); err != nil {
		t.Fatalf("delete keyring entry: %v", err)
	}

	// Load should return ErrKeyNotFound.
	_, err = store.Load()
	if err == nil {
		t.Fatal("Load after keyring entry deleted: expected error, got nil")
	}
	if !errors.Is(err, proton.ErrKeyNotFound) {
		t.Errorf("Load error = %v, want wrapped proton.ErrKeyNotFound", err)
	}

	// Verify the stale entry was cleaned up from the index file.
	idxData, err = os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index after cleanup: %v", err)
	}
	var cleaned SessionIndexData
	if err := json.Unmarshal(idxData, &cleaned); err != nil {
		t.Fatalf("unmarshal cleaned index: %v", err)
	}

	if acct, ok := cleaned.Accounts["alice"]; ok {
		if _, ok := acct.Sessions["drive"]; ok {
			t.Error("stale entry for alice/drive still present in index after cleanup")
		}
	}
}
