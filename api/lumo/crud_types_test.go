package lumo

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestSelectBestMasterKey_EmptyList(t *testing.T) {
	_, err := SelectBestMasterKey(nil)
	if err == nil {
		t.Fatal("expected error for empty list")
	}
}

func TestSelectBestMasterKey_SingleEntry(t *testing.T) {
	key := MasterKeyEntry{ID: "k1", IsLatest: false, Version: 1, CreateTime: "2024-01-01T00:00:00Z", MasterKey: "armored"}
	got, err := SelectBestMasterKey([]MasterKeyEntry{key})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != key.ID {
		t.Fatalf("got ID %q, want %q", got.ID, key.ID)
	}
}

func TestSelectBestMasterKey_IsLatestWins(t *testing.T) {
	keys := []MasterKeyEntry{
		{ID: "a", IsLatest: false, Version: 10, CreateTime: "2024-12-01T00:00:00Z"},
		{ID: "b", IsLatest: true, Version: 1, CreateTime: "2024-01-01T00:00:00Z"},
	}
	got, err := SelectBestMasterKey(keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "b" {
		t.Fatalf("got ID %q, want %q", got.ID, "b")
	}
}

func TestSelectBestMasterKey_VersionTiebreak(t *testing.T) {
	keys := []MasterKeyEntry{
		{ID: "a", IsLatest: true, Version: 1, CreateTime: "2024-12-01T00:00:00Z"},
		{ID: "b", IsLatest: true, Version: 5, CreateTime: "2024-01-01T00:00:00Z"},
	}
	got, _ := SelectBestMasterKey(keys)
	if got.ID != "b" {
		t.Fatalf("got ID %q, want %q (higher version)", got.ID, "b")
	}
}

func TestSpacePriv_IsProjectNilVsFalse(t *testing.T) {
	// nil IsProject — omitted from JSON
	spNil := SpacePriv{}
	dataNil, _ := json.Marshal(spNil)
	if string(dataNil) != "{}" {
		t.Fatalf("nil IsProject: got %s, want {}", dataNil)
	}

	// false IsProject — present in JSON as false
	f := false
	spFalse := SpacePriv{IsProject: &f}
	dataFalse, _ := json.Marshal(spFalse)
	want := `{"isProject":false}`
	if string(dataFalse) != want {
		t.Fatalf("false IsProject: got %s, want %s", dataFalse, want)
	}

	// Unmarshal nil case → IsProject stays nil
	var gotNil SpacePriv
	if err := json.Unmarshal(dataNil, &gotNil); err != nil {
		t.Fatalf("unmarshal nil case: %v", err)
	}
	if gotNil.IsProject != nil {
		t.Fatalf("nil case: IsProject = %v, want nil", *gotNil.IsProject)
	}

	// Unmarshal false case → IsProject is *false
	var gotFalse SpacePriv
	if err := json.Unmarshal(dataFalse, &gotFalse); err != nil {
		t.Fatalf("unmarshal false case: %v", err)
	}
	if gotFalse.IsProject == nil || *gotFalse.IsProject != false {
		t.Fatalf("false case: IsProject = %v, want *false", gotFalse.IsProject)
	}
}

func TestCRUDErrorSentinels_Distinct(t *testing.T) {
	sentinels := []error{ErrNotEligible, ErrNotFound, ErrConflict}

	for i := 0; i < len(sentinels); i++ {
		for j := i + 1; j < len(sentinels); j++ {
			if errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinels[%d] == sentinels[%d]: %v", i, j, sentinels[i])
			}
		}
	}

	for _, s := range sentinels {
		if s.Error() == "" {
			t.Errorf("sentinel has empty message: %v", s)
		}
	}
}

func TestCRUDErrorSentinels_Matchable(t *testing.T) {
	// Wrapped errors must be matchable with errors.Is.
	wrapped := fmt.Errorf("lumo: get space: %w", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("wrapped ErrNotFound not matchable")
	}

	wrapped2 := fmt.Errorf("lumo: check eligibility: %w", ErrNotEligible)
	if !errors.Is(wrapped2, ErrNotEligible) {
		t.Error("wrapped ErrNotEligible not matchable")
	}

	wrapped3 := fmt.Errorf("lumo: create space: %w", ErrConflict)
	if !errors.Is(wrapped3, ErrConflict) {
		t.Error("wrapped ErrConflict not matchable")
	}
}

func TestCRUDErrorSentinels_DistinctFromExisting(t *testing.T) {
	// CRUD sentinels must not collide with existing stream sentinels.
	existing := []error{ErrStreamClosed, ErrRejected, ErrHarmful, ErrTimeout, ErrDecryptionFailed}
	crud := []error{ErrNotEligible, ErrNotFound, ErrConflict}

	for _, e := range existing {
		for _, c := range crud {
			if errors.Is(e, c) || errors.Is(c, e) {
				t.Errorf("CRUD sentinel %v collides with existing %v", c, e)
			}
		}
	}
}
