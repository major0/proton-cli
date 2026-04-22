package lumo

import (
	"bytes"
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// --- Property 2: AES-KW wrap/unwrap round-trip ---

// TestAESKW_RoundTrip_Property verifies that for any 32-byte master key
// and any 32-byte space key, UnwrapSpaceKey(mk, WrapSpaceKey(mk, sk))
// returns the original space key bytes.
//
// Feature: lumo-crud, Property 2: AES-KW wrap/unwrap round-trip
//
// **Validates: Requirements 2.1**
func TestAESKW_RoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		mk := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "master_key")
		sk := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "space_key")

		wrapped, err := WrapSpaceKey(mk, sk)
		if err != nil {
			t.Fatalf("WrapSpaceKey: %v", err)
		}

		unwrapped, err := UnwrapSpaceKey(mk, wrapped)
		if err != nil {
			t.Fatalf("UnwrapSpaceKey: %v", err)
		}

		if !bytes.Equal(sk, unwrapped) {
			t.Fatalf("round-trip mismatch:\norig: %x\ngot:  %x", sk, unwrapped)
		}
	})
}

// --- Property 3: HKDF derivation determinism ---

// TestHKDF_Determinism_Property verifies that DeriveDataEncryptionKey
// is deterministic (same input → same output) and that distinct inputs
// produce distinct outputs.
//
// Feature: lumo-crud, Property 3: HKDF derivation determinism
//
// **Validates: Requirements 2.2**
func TestHKDF_Determinism_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sk := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "space_key")

		dek1, err := DeriveDataEncryptionKey(sk)
		if err != nil {
			t.Fatalf("DeriveDataEncryptionKey (1): %v", err)
		}
		dek2, err := DeriveDataEncryptionKey(sk)
		if err != nil {
			t.Fatalf("DeriveDataEncryptionKey (2): %v", err)
		}
		if !bytes.Equal(dek1, dek2) {
			t.Fatalf("determinism failed: %x != %x", dek1, dek2)
		}
	})

	// Distinct inputs → distinct outputs.
	rapid.Check(t, func(t *rapid.T) {
		sk1 := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "space_key_1")
		sk2 := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "space_key_2")
		if bytes.Equal(sk1, sk2) {
			t.Skip("identical keys")
		}

		dek1, err := DeriveDataEncryptionKey(sk1)
		if err != nil {
			t.Fatalf("DeriveDataEncryptionKey (1): %v", err)
		}
		dek2, err := DeriveDataEncryptionKey(sk2)
		if err != nil {
			t.Fatalf("DeriveDataEncryptionKey (2): %v", err)
		}
		if bytes.Equal(dek1, dek2) {
			t.Fatalf("distinct keys produced same DEK: %x", dek1)
		}
	})
}

// --- Property 4: EncryptString/DecryptString round-trip ---

// TestEncryptDecrypt_RoundTrip_Property verifies that for any plaintext,
// 32-byte DEK, and AD string, DecryptString(EncryptString(pt, dek, ad),
// dek, ad) returns the original plaintext.
//
// Feature: lumo-crud, Property 4: EncryptString/DecryptString round-trip
//
// **Validates: Requirements 2.3, 2.4, 4.4, 5.3**
func TestEncryptDecrypt_RoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		plaintext := rapid.String().Draw(t, "plaintext")
		dek := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "dek")
		ad := rapid.String().Draw(t, "ad")

		encrypted, err := EncryptString(plaintext, dek, ad)
		if err != nil {
			t.Fatalf("EncryptString: %v", err)
		}

		decrypted, err := DecryptString(encrypted, dek, ad)
		if err != nil {
			t.Fatalf("DecryptString: %v", err)
		}

		if decrypted != plaintext {
			t.Fatalf("round-trip mismatch:\norig: %q\ngot:  %q", plaintext, decrypted)
		}
	})
}

// --- Property 5: AD string determinism and format ---

// TestAD_Determinism_Property verifies that AD construction functions
// are deterministic and produce valid JSON with alphabetically sorted keys.
//
// Feature: lumo-crud, Property 5: AD string determinism and format
//
// **Validates: Requirements 4.4, 5.3**
func TestAD_Determinism_Property(t *testing.T) {
	t.Run("SpaceAD", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			tag := rapid.StringMatching(`[a-zA-Z0-9-]{1,36}`).Draw(t, "space_tag")
			ad1 := SpaceAD(tag)
			ad2 := SpaceAD(tag)
			if ad1 != ad2 {
				t.Fatalf("SpaceAD not deterministic: %q != %q", ad1, ad2)
			}
			assertValidSortedJSON(t, ad1)
		})
	})

	t.Run("ConversationAD", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			convTag := rapid.StringMatching(`[a-zA-Z0-9-]{1,36}`).Draw(t, "conv_tag")
			spaceTag := rapid.StringMatching(`[a-zA-Z0-9-]{1,36}`).Draw(t, "space_tag")
			ad1 := ConversationAD(convTag, spaceTag)
			ad2 := ConversationAD(convTag, spaceTag)
			if ad1 != ad2 {
				t.Fatalf("ConversationAD not deterministic: %q != %q", ad1, ad2)
			}
			assertValidSortedJSON(t, ad1)
		})
	})

	t.Run("MessageAD", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			msgTag := rapid.StringMatching(`[a-zA-Z0-9-]{1,36}`).Draw(t, "msg_tag")
			role := rapid.SampledFrom([]string{"user", "assistant"}).Draw(t, "role")
			parentID := rapid.StringMatching(`[a-zA-Z0-9-]{0,36}`).Draw(t, "parent_id")
			convTag := rapid.StringMatching(`[a-zA-Z0-9-]{1,36}`).Draw(t, "conv_tag")
			ad1 := MessageAD(msgTag, role, parentID, convTag)
			ad2 := MessageAD(msgTag, role, parentID, convTag)
			if ad1 != ad2 {
				t.Fatalf("MessageAD not deterministic: %q != %q", ad1, ad2)
			}
			assertValidSortedJSON(t, ad1)
		})
	})
}

// assertValidSortedJSON verifies that s is valid JSON with alphabetically
// sorted keys.
func assertValidSortedJSON(t *rapid.T, s string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON: %v\nstring: %s", err, s)
	}

	// Extract keys in order from the raw JSON string.
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	tok, _ := dec.Token() // opening {
	if tok != json.Delim('{') {
		t.Fatalf("expected {, got %v", tok)
	}
	var keys []string
	for dec.More() {
		tok, _ := dec.Token()
		key, ok := tok.(string)
		if !ok {
			t.Fatalf("expected string key, got %T", tok)
		}
		keys = append(keys, key)
		// Skip the value.
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("decode value: %v", err)
		}
	}

	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Fatalf("keys not sorted: %v", keys)
		}
	}
}
