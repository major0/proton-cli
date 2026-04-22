package lumo

// Feature: lumo-api, Property 6: AD string derivation format
// Feature: lumo-api, Property 7: AES-GCM encryption round-trip
// Feature: lumo-api, Property 8: PGP request key encryption round-trip

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	pgphelper "github.com/ProtonMail/gopenpgp/v2/helper"
	"pgregory.net/rapid"
)

// TestAD_DerivationFormat_Property verifies that for any request ID,
// RequestAD returns "lumo.request.<id>.turn" and ResponseAD returns
// "lumo.response.<id>.chunk".
//
// **Validates: Requirements 4.3**
func TestAD_DerivationFormat_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.String().Draw(t, "requestID")

		reqAD := RequestAD(id)
		wantReq := "lumo.request." + id + ".turn"
		if reqAD != wantReq {
			t.Fatalf("RequestAD(%q) = %q, want %q", id, reqAD, wantReq)
		}

		respAD := ResponseAD(id)
		wantResp := "lumo.response." + id + ".chunk"
		if respAD != wantResp {
			t.Fatalf("ResponseAD(%q) = %q, want %q", id, respAD, wantResp)
		}
	})
}

// TestAESGCM_EncryptDecryptRoundTrip_Property verifies that for any
// plaintext, any 32-byte key, and any AD string, encrypting then
// decrypting produces the original plaintext.
//
// **Validates: Requirements 4.4, 4.5, 4.7, 4.8**
func TestAESGCM_EncryptDecryptRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		plaintext := rapid.SliceOf(rapid.Byte()).Draw(t, "plaintext")
		key := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "key")
		adStr := rapid.String().Draw(t, "ad")
		ad := []byte(adStr)

		encoded, err := encryptAESGCM(plaintext, key, ad)
		if err != nil {
			t.Fatalf("encryptAESGCM: %v", err)
		}

		decoded, err := decryptAESGCM(encoded, key, ad)
		if err != nil {
			t.Fatalf("decryptAESGCM: %v", err)
		}

		// Both nil and empty slices represent "no data".
		if len(plaintext) == 0 && len(decoded) == 0 {
			return
		}
		if !bytes.Equal(decoded, plaintext) {
			t.Fatalf("round-trip mismatch: got %x, want %x", decoded, plaintext)
		}
	})
}

// TestPGP_RequestKeyRoundTrip_Property verifies that for any 32-byte
// request key, encrypting with a PGP public key then decrypting with
// the corresponding private key produces the original key bytes.
//
// A test keypair is generated once (not the production key — we don't
// have its private key).
//
// **Validates: Requirements 4.6**
func TestPGP_RequestKeyRoundTrip_Property(t *testing.T) {
	// Generate a test keypair (x25519 for speed).
	armoredPriv, err := pgphelper.GenerateKey("Test", "test@test.local", nil, "x25519", 0)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	privKey, err := crypto.NewKeyFromArmored(armoredPriv)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}
	armoredPub, err := privKey.GetArmoredPublicKey()
	if err != nil {
		t.Fatalf("extract public key: %v", err)
	}
	privRing, err := crypto.NewKeyRing(privKey)
	if err != nil {
		t.Fatalf("create private keyring: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		key := rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "key")

		encoded, err := EncryptRequestKey(key, armoredPub)
		if err != nil {
			t.Fatalf("EncryptRequestKey: %v", err)
		}

		// Decode base64 → binary PGP message → decrypt.
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("base64 decode: %v", err)
		}
		pgpMsg := crypto.NewPGPMessage(raw)
		plain, err := privRing.Decrypt(pgpMsg, nil, 0)
		if err != nil {
			t.Fatalf("PGP decrypt: %v", err)
		}
		if !bytes.Equal(plain.GetBinary(), key) {
			t.Fatalf("PGP round-trip mismatch: got %x, want %x", plain.GetBinary(), key)
		}
	})
}
