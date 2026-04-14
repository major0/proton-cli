package shareCmd

import (
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/api/drive"
	"pgregory.net/rapid"
)

func TestProhibitedShareType(t *testing.T) {
	prohibited := []proton.ShareType{proton.ShareTypeMain, drive.ShareTypePhotos, proton.ShareTypeDevice}
	for _, st := range prohibited {
		if !prohibitedShareType(st) {
			t.Errorf("expected share type %d to be prohibited", st)
		}
	}

	if prohibitedShareType(proton.ShareTypeStandard) {
		t.Error("ShareTypeStandard should be allowed")
	}
}

func TestBoolState(t *testing.T) {
	if boolState(true) != "enabled" {
		t.Fatal("true should be 'enabled'")
	}
	if boolState(false) != "disabled" {
		t.Fatal("false should be 'disabled'")
	}
}

// TestShareCacheToggleRoundTrip_Property verifies that toggling cache
// flags via config save/load produces the expected state.
//
// **Property 5: Share cache toggle round-trip**
// **Validates: Requirements 4.2, 4.3, 4.5**
func TestShareCacheToggleRoundTrip_Property(t *testing.T) {
	dir := t.TempDir()

	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 ]{2,15}`).Draw(t, "name")
		dirent := rapid.Bool().Draw(t, "dirent")
		metadata := rapid.Bool().Draw(t, "metadata")
		disk := rapid.Bool().Draw(t, "disk")

		cfg := api.DefaultConfig()
		cfg.Shares[name] = api.ShareConfig{
			DirentCacheEnabled:   dirent,
			MetadataCacheEnabled: metadata,
			DiskCacheEnabled:     disk,
		}

		path := filepath.Join(dir, rapid.StringMatching(`[a-z]{8}`).Draw(t, "file")+".yaml")
		if err := api.SaveConfig(path, cfg); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}

		loaded, err := api.LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}

		sc := loaded.Shares[name]
		if sc.DirentCacheEnabled != dirent {
			t.Fatalf("dirent: got %v, want %v", sc.DirentCacheEnabled, dirent)
		}
		if sc.MetadataCacheEnabled != metadata {
			t.Fatalf("metadata: got %v, want %v", sc.MetadataCacheEnabled, metadata)
		}
		if sc.DiskCacheEnabled != disk {
			t.Fatalf("disk: got %v, want %v", sc.DiskCacheEnabled, disk)
		}
	})
}
