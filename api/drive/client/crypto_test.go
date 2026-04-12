package client

import (
	"errors"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/api/drive"
	"pgregory.net/rapid"
)

// TestAddrKRForLink_NoMatch_Property verifies that addrKRForLink returns
// an error wrapping ErrKeyNotFound (never a non-nil keyring) when the
// link's SignatureEmail does not match any address in the Client.
//
// **Property 5: Crypto Determinism — No Arbitrary Fallback**
// **Validates: Requirements 7.1, 7.2**
func TestAddrKRForLink_NoMatch_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		email := rapid.String().Draw(t, "email")

		c := &Client{
			addresses:       make(map[string]proton.Address),
			addressKeyRings: make(map[string]*crypto.KeyRing),
		}

		pLink := &proton.Link{
			LinkID:         "test-link",
			SignatureEmail: email,
		}
		link := drive.NewLink(pLink, nil, nil, nil)

		kr, err := c.addrKRForLink(link)
		if kr != nil {
			t.Fatalf("expected nil keyring for unmatched email %q, got non-nil", email)
		}
		if err == nil {
			t.Fatalf("expected error for unmatched email %q, got nil", email)
		}
		if !errors.Is(err, api.ErrKeyNotFound) {
			t.Fatalf("expected error wrapping ErrKeyNotFound, got: %v", err)
		}
	})
}

// TestSignatureAddress_Empty_Property verifies that signatureAddress returns
// an error wrapping ErrKeyNotFound (never a non-empty string) when the
// link's SignatureEmail is empty.
//
// **Property 5: Crypto Determinism — No Arbitrary Fallback**
// **Validates: Requirements 7.1, 7.2**
func TestSignatureAddress_Empty_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Populate the address map with random entries to verify no fallback.
		n := rapid.IntRange(0, 10).Draw(t, "addrCount")
		addrs := make(map[string]proton.Address, n)
		for i := 0; i < n; i++ {
			email := rapid.StringMatching(`[a-z]{3,10}@[a-z]{3,8}\.[a-z]{2,4}`).Draw(t, "mapEmail")
			addrs[email] = proton.Address{ID: email, Email: email}
		}

		c := &Client{
			addresses:       addrs,
			addressKeyRings: make(map[string]*crypto.KeyRing),
		}

		pLink := &proton.Link{
			LinkID:         "test-link",
			SignatureEmail: "", // empty
		}
		link := drive.NewLink(pLink, nil, nil, nil)

		addr, err := c.signatureAddress(link)
		if addr != "" {
			t.Fatalf("expected empty string for empty SignatureEmail, got %q", addr)
		}
		if err == nil {
			t.Fatal("expected error for empty SignatureEmail, got nil")
		}
		if !errors.Is(err, api.ErrKeyNotFound) {
			t.Fatalf("expected error wrapping ErrKeyNotFound, got: %v", err)
		}
	})
}
