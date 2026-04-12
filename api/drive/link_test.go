package drive

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api"
	"pgregory.net/rapid"
)

// mockLinkResolver is a minimal LinkResolver that always fails address
// lookups. Used to test error context in deriveKeyRing and decryptName.
type mockLinkResolver struct{}

func (m *mockLinkResolver) ListLinkChildren(_ context.Context, _, _ string, _ bool) ([]proton.Link, error) {
	return nil, nil
}

func (m *mockLinkResolver) NewChildLink(_ context.Context, parent *Link, pLink *proton.Link) *Link {
	return NewLink(pLink, parent, parent.share, m)
}

func (m *mockLinkResolver) AddressForEmail(_ string) (proton.Address, bool) {
	return proton.Address{}, false
}

func (m *mockLinkResolver) AddressKeyRing(_ string) (*crypto.KeyRing, bool) {
	return nil, false
}

func (m *mockLinkResolver) Throttle() *api.Throttle { return nil }
func (m *mockLinkResolver) MaxWorkers() int         { return 1 }

// TestDeriveKeyRing_ErrorContext_Property verifies that deriveKeyRing returns
// an error wrapping ErrKeyNotFound that contains the signature email string
// when the resolver has no matching address.
//
// **Property 6: Key-Not-Found Errors Include Email Context**
// **Validates: Requirements 8.1, 8.2**
func TestDeriveKeyRing_ErrorContext_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		email := rapid.String().Draw(t, "signatureEmail")

		resolver := &mockLinkResolver{}
		pLink := &proton.Link{
			LinkID:         "test-link",
			SignatureEmail: email,
		}
		link := NewLink(pLink, nil, nil, resolver)

		kr, err := link.deriveKeyRing(nil)
		if kr != nil {
			t.Fatalf("expected nil keyring for unmatched email %q, got non-nil", email)
		}
		if err == nil {
			t.Fatalf("expected error for unmatched email %q, got nil", email)
		}
		if !errors.Is(err, api.ErrKeyNotFound) {
			t.Fatalf("expected error wrapping ErrKeyNotFound, got: %v", err)
		}
		// The error uses %q formatting, so check for the quoted email.
		quoted := fmt.Sprintf("%q", email)
		if !strings.Contains(err.Error(), quoted) {
			t.Fatalf("error %q does not contain quoted email %s", err.Error(), quoted)
		}
	})
}

// TestDecryptName_ErrorContext_Property verifies that decryptName returns
// an error wrapping ErrKeyNotFound that contains the name signature email
// string when the resolver has no matching address.
//
// **Property 6: Key-Not-Found Errors Include Email Context**
// **Validates: Requirements 8.1, 8.2**
func TestDecryptName_ErrorContext_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		email := rapid.String().Draw(t, "nameSignatureEmail")

		resolver := &mockLinkResolver{}
		pLink := &proton.Link{
			LinkID:             "test-link",
			NameSignatureEmail: email,
		}
		link := NewLink(pLink, nil, nil, resolver)

		name, err := link.decryptName(nil)
		if name != "" {
			t.Fatalf("expected empty name for unmatched email %q, got %q", email, name)
		}
		if err == nil {
			t.Fatalf("expected error for unmatched email %q, got nil", email)
		}
		if !errors.Is(err, api.ErrKeyNotFound) {
			t.Fatalf("expected error wrapping ErrKeyNotFound, got: %v", err)
		}
		// The error uses %q formatting, so check for the quoted email.
		quoted := fmt.Sprintf("%q", email)
		if !strings.Contains(err.Error(), quoted) {
			t.Fatalf("error %q does not contain quoted email %s", err.Error(), quoted)
		}
	})
}
