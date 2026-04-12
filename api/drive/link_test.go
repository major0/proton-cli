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

// TestIsTransient_Property verifies that isTransient returns true for any
// error wrapping context.Canceled or context.DeadlineExceeded, and false
// for any error that does not wrap either (including nil).
//
// **Property 7: Transient Error Classification**
// **Validates: Requirement 9.1**
func TestIsTransient_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		wrapCtx := rapid.Bool().Draw(t, "wrapContext")
		msg := rapid.String().Draw(t, "msg")

		var err error
		if wrapCtx {
			base := rapid.SampledFrom([]error{
				context.Canceled,
				context.DeadlineExceeded,
			}).Draw(t, "base")
			err = fmt.Errorf("%s: %w", msg, base)
		} else {
			err = errors.New(msg)
		}

		got := isTransient(err)
		if wrapCtx && !got {
			t.Fatalf("expected transient for %v, got false", err)
		}
		if !wrapCtx && got {
			t.Fatalf("expected non-transient for %v, got true", err)
		}
	})
}

// TestIsTransient_Nil verifies that isTransient(nil) returns false.
func TestIsTransient_Nil(t *testing.T) {
	if isTransient(nil) {
		t.Fatal("expected isTransient(nil) == false")
	}
}

// TestIsTransient_KnownErrors verifies isTransient for specific known errors.
func TestIsTransient_KnownErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"context.Canceled", context.Canceled, true},
		{"context.DeadlineExceeded", context.DeadlineExceeded, true},
		{"wrapped Canceled", fmt.Errorf("op: %w", context.Canceled), true},
		{"wrapped DeadlineExceeded", fmt.Errorf("op: %w", context.DeadlineExceeded), true},
		{"plain error", errors.New("foo"), false},
		{"ErrKeyNotFound", api.ErrKeyNotFound, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.err); got != tt.want {
				t.Fatalf("isTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// transientResolver is a mock LinkResolver that returns transient errors
// on the first N calls to getParentKeyRing (via the parent link's KeyRing),
// then succeeds. Used to test decrypt retry semantics.
type transientResolver struct {
	mockLinkResolver
	calls    int
	failFor  int
	failWith error
}

func (r *transientResolver) AddressForEmail(_ string) (proton.Address, bool) {
	r.calls++
	if r.calls <= r.failFor {
		return proton.Address{}, false
	}
	return proton.Address{ID: "addr-1"}, true
}

func (r *transientResolver) AddressKeyRing(id string) (*crypto.KeyRing, bool) {
	if id == "addr-1" {
		// Return a non-nil keyring stub. The actual crypto operations
		// will fail, but we're testing the retry/cache logic, not crypto.
		return nil, false
	}
	return nil, false
}

// TestDecrypt_TransientNotCached verifies that a transient error during
// decrypt does not set decrypted=true, allowing retry on subsequent calls.
func TestDecrypt_TransientNotCached(t *testing.T) {
	resolver := &mockLinkResolver{}
	// Create a share root with no parent — getParentKeyRing will call
	// share.getKeyRing() which will fail. We simulate transient by
	// wrapping context.Canceled in the share's keyring error path.
	// Instead, we test the struct fields directly after a transient error.

	pLink := &proton.Link{
		LinkID:         "test-link",
		SignatureEmail: "test@example.com",
	}
	link := NewLink(pLink, nil, nil, resolver)
	// share is nil, so getParentKeyRing will panic. Instead, set up a
	// parent link that returns a transient error from KeyRing().

	// Create a parent link that has a transient decryptErr.
	parentPLink := &proton.Link{LinkID: "parent"}
	parent := NewLink(parentPLink, nil, nil, resolver)
	parent.decryptErr = context.Canceled
	parent.decrypted = true

	link.parentLink = parent

	err := link.decrypt()
	if err == nil {
		t.Fatal("expected error from decrypt, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	// Transient: decrypted should still be false.
	if link.decrypted {
		t.Fatal("expected decrypted=false after transient error")
	}
	if link.decryptErr != nil {
		t.Fatalf("expected decryptErr=nil after transient error, got: %v", link.decryptErr)
	}
}

// TestDecrypt_PermanentCached verifies that a permanent error during
// decrypt sets decrypted=true and caches the error.
func TestDecrypt_PermanentCached(t *testing.T) {
	resolver := &mockLinkResolver{}
	pLink := &proton.Link{
		LinkID:         "test-link",
		SignatureEmail: "test@example.com",
	}
	link := NewLink(pLink, nil, nil, resolver)

	// Create a parent link that has a permanent (non-transient) decryptErr.
	parentPLink := &proton.Link{LinkID: "parent"}
	parent := NewLink(parentPLink, nil, nil, resolver)
	parent.decryptErr = fmt.Errorf("parent: %w", api.ErrKeyNotFound)
	parent.decrypted = true

	link.parentLink = parent

	err1 := link.decrypt()
	if err1 == nil {
		t.Fatal("expected error from first decrypt, got nil")
	}
	if !errors.Is(err1, api.ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got: %v", err1)
	}
	// Permanent: decrypted should be true.
	if !link.decrypted {
		t.Fatal("expected decrypted=true after permanent error")
	}

	// Second call should return the same cached error.
	err2 := link.decrypt()
	if err2 == nil {
		t.Fatal("expected error from second decrypt, got nil")
	}
	if err1.Error() != err2.Error() {
		t.Fatalf("expected same cached error, got %v vs %v", err1, err2)
	}
}

// TestDecrypt_TransientThenRetry verifies that after a transient error,
// a subsequent call to decrypt retries from scratch.
func TestDecrypt_TransientThenRetry(t *testing.T) {
	resolver := &mockLinkResolver{}
	pLink := &proton.Link{
		LinkID:         "test-link",
		SignatureEmail: "test@example.com",
	}
	link := NewLink(pLink, nil, nil, resolver)

	// First call: parent returns transient error.
	parentPLink := &proton.Link{LinkID: "parent"}
	parent := NewLink(parentPLink, nil, nil, resolver)
	parent.decryptErr = context.DeadlineExceeded
	parent.decrypted = true
	link.parentLink = parent

	err := link.decrypt()
	if err == nil {
		t.Fatal("expected transient error, got nil")
	}
	if link.decrypted {
		t.Fatal("expected decrypted=false after transient error")
	}

	// Now fix the parent to return a permanent error (simulating a
	// different failure on retry — the point is decrypt retries).
	parent.decryptErr = fmt.Errorf("parent: %w", api.ErrKeyNotFound)

	err = link.decrypt()
	if err == nil {
		t.Fatal("expected permanent error on retry, got nil")
	}
	if !errors.Is(err, api.ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound on retry, got: %v", err)
	}
	// Now it should be cached.
	if !link.decrypted {
		t.Fatal("expected decrypted=true after permanent error on retry")
	}
}
