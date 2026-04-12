package drive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api"
)

// Link represents a file or folder in a Proton Drive share. Fields are
// decrypted lazily — the raw encrypted proton.Link is retained and
// decryption happens on first access. This keeps encrypted data in memory
// as long as possible and avoids decrypting nodes that are never read.
type Link struct {
	// Raw encrypted link from the API. Always populated.
	protonLink *proton.Link

	// Relationships — always set at construction time.
	parentLink *Link
	resolver   LinkResolver
	share      *Share

	// Lazy-decrypted fields, protected by mu.
	// decrypted is true when decryption succeeded or failed permanently.
	// Transient errors leave decrypted false to allow retry.
	mu          sync.Mutex
	decrypted   bool
	decryptErr  error
	name        string
	keyRing     *crypto.KeyRing
	nameKeyRing *crypto.KeyRing
}

// Type returns the link type (file or folder) without decryption.
func (l *Link) Type() proton.LinkType { return l.protonLink.Type }

// State returns the link state without decryption.
func (l *Link) State() proton.LinkState { return l.protonLink.State }

// CreateTime returns the creation timestamp without decryption.
func (l *Link) CreateTime() int64 { return l.protonLink.CreateTime }

// ModifyTime returns the modification timestamp. For files with an active
// revision, returns the revision's create time (which is the upload time).
func (l *Link) ModifyTime() int64 {
	if l.protonLink.Type == proton.LinkTypeFile && l.protonLink.FileProperties != nil {
		return l.protonLink.FileProperties.ActiveRevision.CreateTime
	}
	return l.protonLink.ModifyTime
}

// ExpirationTime returns the expiration timestamp without decryption.
func (l *Link) ExpirationTime() int64 { return l.protonLink.ExpirationTime }

// Size returns the file size. Folders return 0.
func (l *Link) Size() int64 {
	if l.protonLink.Type == proton.LinkTypeFile && l.protonLink.FileProperties != nil {
		return l.protonLink.FileProperties.ActiveRevision.Size
	}
	return 0
}

// MIMEType returns the MIME type without decryption.
func (l *Link) MIMEType() string { return l.protonLink.MIMEType }

// LinkID returns the encrypted link ID without decryption.
func (l *Link) LinkID() string { return l.protonLink.LinkID }

// isTransient returns true for errors that may succeed on retry.
func isTransient(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

// decrypt performs lazy decryption of the link's name and keyrings.
// Permanent errors are cached; transient errors allow retry.
// Safe to call from multiple goroutines.
func (l *Link) decrypt() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.decrypted {
		return l.decryptErr
	}

	parentKR, err := l.getParentKeyRing()
	if err != nil {
		if isTransient(err) {
			return fmt.Errorf("decrypt %s: parent keyring: %w", l.protonLink.LinkID, err)
		}
		l.decryptErr = fmt.Errorf("decrypt %s: parent keyring: %w", l.protonLink.LinkID, err)
		l.decrypted = true
		return l.decryptErr
	}

	l.keyRing, err = l.deriveKeyRing(parentKR)
	if err != nil {
		if isTransient(err) {
			return fmt.Errorf("decrypt %s: keyring: %w", l.protonLink.LinkID, err)
		}
		l.decryptErr = fmt.Errorf("decrypt %s: keyring: %w", l.protonLink.LinkID, err)
		l.decrypted = true
		return l.decryptErr
	}

	l.nameKeyRing = l.keyRing

	l.name, err = l.decryptName(parentKR)
	if err != nil {
		if isTransient(err) {
			// Clear partially-set fields on transient failure.
			l.keyRing = nil
			l.nameKeyRing = nil
			return fmt.Errorf("decrypt %s: name: %w", l.protonLink.LinkID, err)
		}
		l.decryptErr = fmt.Errorf("decrypt %s: name: %w", l.protonLink.LinkID, err)
		l.decrypted = true
		return l.decryptErr
	}

	l.decrypted = true
	return nil
}

// Name returns the decrypted name. Triggers lazy decryption on first call.
func (l *Link) Name() (string, error) {
	if err := l.decrypt(); err != nil {
		return "", err
	}
	return l.name, nil
}

// KeyRing returns the link's decrypted keyring. Triggers lazy decryption.
func (l *Link) KeyRing() (*crypto.KeyRing, error) {
	if err := l.decrypt(); err != nil {
		return nil, err
	}
	return l.keyRing, nil
}

// getParentKeyRing returns the parent's keyring for decryption.
func (l *Link) getParentKeyRing() (*crypto.KeyRing, error) {
	if l.parentLink == nil {
		return l.share.getKeyRing()
	}
	return l.parentLink.KeyRing()
}

// deriveKeyRing derives this link's keyring from the parent keyring.
func (l *Link) deriveKeyRing(parentKR *crypto.KeyRing) (*crypto.KeyRing, error) {
	email := l.protonLink.SignatureEmail
	if addr, ok := l.resolver.AddressForEmail(email); ok {
		if linkKR, ok := l.resolver.AddressKeyRing(addr.ID); ok {
			return l.protonLink.GetKeyRing(parentKR, linkKR)
		}
	}
	return nil, fmt.Errorf("deriveKeyRing: signature email %q: %w", email, api.ErrKeyNotFound)
}

// decryptName decrypts the link name using the parent keyring.
func (l *Link) decryptName(parentKR *crypto.KeyRing) (string, error) {
	email := l.protonLink.NameSignatureEmail
	if addr, ok := l.resolver.AddressForEmail(email); ok {
		if addrKR, ok := l.resolver.AddressKeyRing(addr.ID); ok {
			return l.protonLink.GetName(parentKR, addrKR)
		}
	}
	return "", fmt.Errorf("decryptName: name signature email %q: %w", email, api.ErrKeyNotFound)
}

// ProtonLink returns the raw encrypted proton.Link. Used by the client
// package for API operations that need raw link fields.
func (l *Link) ProtonLink() *proton.Link { return l.protonLink }

// ParentLink returns the parent Link, or nil for share roots.
func (l *Link) ParentLink() *Link { return l.parentLink }

// Share returns the Link's associated Share.
func (l *Link) Share() *Share { return l.share }

// NewLink creates a Link wrapper without decrypting anything.
func NewLink(pLink *proton.Link, parent *Link, share *Share, resolver LinkResolver) *Link {
	return &Link{
		protonLink: pLink,
		parentLink: parent,
		share:      share,
		resolver:   resolver,
	}
}

// newChildLink creates a child Link from a raw proton.Link, delegating
// to the resolver for construction.
func (l *Link) newChildLink(ctx context.Context, pLink *proton.Link) *Link {
	return l.resolver.NewChildLink(ctx, l, pLink)
}

// ResolvePath resolves a slash-separated path relative to this link.
// Only decrypts names along the path — siblings are not decrypted.
func (l *Link) ResolvePath(ctx context.Context, path string, _ bool) (*Link, error) {
	slog.Debug("link.ResolvePath", "path", path)
	path = strings.Trim(path, "/")
	if path == "" {
		return l, nil
	}
	parts := strings.Split(path, "/")
	return l.resolveParts(ctx, parts)
}

// resolveParts walks path components using Lookup (concurrent, cancellable).
// Only the matching child at each level is decrypted.
func (l *Link) resolveParts(ctx context.Context, parts []string) (*Link, error) {
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		return l, nil
	}

	if l.Type() != proton.LinkTypeFolder {
		return nil, ErrNotAFolder
	}

	child, err := l.Lookup(ctx, parts[0])
	if err != nil {
		return nil, err
	}
	if child == nil {
		return nil, ErrFileNotFound
	}

	return child.resolveParts(ctx, parts[1:])
}
