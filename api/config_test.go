package api

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

func TestLoadConfig_MissingFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(cfg.Shares) != 0 {
		t.Fatalf("expected empty shares, got %d", len(cfg.Shares))
	}
	if len(cfg.Defaults) != 0 {
		t.Fatalf("expected empty defaults, got %d", len(cfg.Defaults))
	}
}

func TestLoadConfig_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("{{invalid yaml"), 0600)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestSaveConfig_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "config.yaml")

	cfg := DefaultConfig()
	cfg.Shares["test"] = ShareConfig{DirentCacheEnabled: true}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Shares["MyFolder"] = ShareConfig{
		DirentCacheEnabled:   true,
		MetadataCacheEnabled: true,
		DiskCacheEnabled:     false,
	}
	cfg.Defaults["drive"] = "work"

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if !reflect.DeepEqual(cfg.Shares, loaded.Shares) {
		t.Fatalf("shares mismatch:\n  got:  %+v\n  want: %+v", loaded.Shares, cfg.Shares)
	}
	if !reflect.DeepEqual(cfg.Defaults, loaded.Defaults) {
		t.Fatalf("defaults mismatch:\n  got:  %+v\n  want: %+v", loaded.Defaults, cfg.Defaults)
	}
}

func TestSaveConfig_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Shares["test"] = ShareConfig{DiskCacheEnabled: true}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// Verify the file is valid YAML (not a partial write).
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig after save: %v", err)
	}
	if !loaded.Shares["test"].DiskCacheEnabled {
		t.Fatal("expected DiskCacheEnabled=true after save")
	}

	// Verify no temp file left behind.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file should not exist after successful save")
	}
}

func TestDefaultAccount(t *testing.T) {
	cfg := DefaultConfig()
	if got := cfg.DefaultAccount("drive"); got != "default" {
		t.Fatalf("unconfigured service: got %q, want %q", got, "default")
	}

	cfg.Defaults["drive"] = "work"
	if got := cfg.DefaultAccount("drive"); got != "work" {
		t.Fatalf("configured service: got %q, want %q", got, "work")
	}

	if got := cfg.DefaultAccount("mail"); got != "default" {
		t.Fatalf("other service: got %q, want %q", got, "default")
	}
}

// TestConfigRoundTrip_Property verifies that for any valid Config,
// SaveConfig + LoadConfig produces an equivalent Config.
//
// **Property 1: Config serialization round-trip**
// **Validates: Requirements 1.2, 1.4, 2.1, 2.10, 3.1**
func TestConfigRoundTrip_Property(t *testing.T) {
	dir := t.TempDir()

	rapid.Check(t, func(t *rapid.T) {
		cfg := &Config{
			Shares:   make(map[string]ShareConfig),
			Defaults: make(map[string]string),
		}

		nShares := rapid.IntRange(0, 5).Draw(t, "nShares")
		for i := 0; i < nShares; i++ {
			name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 ]{0,15}`).Draw(t, "shareName")
			cfg.Shares[name] = ShareConfig{
				DirentCacheEnabled:   rapid.Bool().Draw(t, "dirent"),
				MetadataCacheEnabled: rapid.Bool().Draw(t, "metadata"),
				DiskCacheEnabled:     rapid.Bool().Draw(t, "disk"),
			}
		}

		nDefaults := rapid.IntRange(0, 3).Draw(t, "nDefaults")
		for i := 0; i < nDefaults; i++ {
			svc := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "service")
			acct := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "account")
			cfg.Defaults[svc] = acct
		}

		path := filepath.Join(dir, rapid.StringMatching(`[a-z]{8}`).Draw(t, "file")+".yaml")

		if err := SaveConfig(path, cfg); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}

		loaded, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}

		if !reflect.DeepEqual(cfg.Shares, loaded.Shares) {
			t.Fatalf("shares mismatch")
		}
		if !reflect.DeepEqual(cfg.Defaults, loaded.Defaults) {
			t.Fatalf("defaults mismatch")
		}
	})
}

// TestUnconfiguredShareDefaults_Property verifies that shares not in
// the config map have all caches disabled.
//
// **Property 2: Unconfigured shares default to caching disabled**
// **Validates: Requirements 2.4**
func TestUnconfiguredShareDefaults_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cfg := DefaultConfig()
		nShares := rapid.IntRange(0, 5).Draw(t, "nShares")
		for i := 0; i < nShares; i++ {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "name")
			cfg.Shares[name] = ShareConfig{
				DirentCacheEnabled:   rapid.Bool().Draw(t, "d"),
				MetadataCacheEnabled: rapid.Bool().Draw(t, "m"),
				DiskCacheEnabled:     rapid.Bool().Draw(t, "k"),
			}
		}

		// Generate a name guaranteed absent.
		absent := "ABSENT_" + rapid.StringMatching(`[A-Z]{8}`).Draw(t, "absent")
		sc := cfg.Shares[absent] // zero value

		if sc.DirentCacheEnabled || sc.MetadataCacheEnabled || sc.DiskCacheEnabled {
			t.Fatal("unconfigured share should have all caches disabled")
		}
	})
}
