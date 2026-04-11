package proton

import (
	"context"
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

// ShareMetadata represents the metadata for a Proton Drive share.
type ShareMetadata proton.ShareMetadata

// Share represents a fully-resolved Proton Drive share with its keyring.
type Share struct {
	Link        *Link
	keyRing     *crypto.KeyRing
	protonShare *proton.Share
	session     *Session
}

// GetName returns the name of the share's root link.
func (s *Share) GetName(_ context.Context) string {
	slog.Debug("share.GetName")
	return s.Link.Name
}

// ListChildren returns the child links of the share's root folder.
func (s *Share) ListChildren(ctx context.Context, all bool) ([]Link, error) {
	slog.Debug("share.ListChildren", "all", all)
	return s.Link.ListChildren(ctx, all)
}

// ResolvePath resolves a slash-separated path relative to the share's root link.
func (s *Share) ResolvePath(ctx context.Context, path string, all bool) (*Link, error) {
	slog.Debug("share.ResolvePath", "path", path, "all", all)
	return s.Link.ResolvePath(ctx, path, all)
}

func (s *Share) getKeyRing() (*crypto.KeyRing, error) {
	linkKR, ok := s.session.AddressKeyRing[s.protonShare.AddressID]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return s.protonShare.GetKeyRing(linkKR)
}
