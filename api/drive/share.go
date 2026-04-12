package drive

import (
	"context"
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api"
)

// ShareMetadata represents the metadata for a Proton Drive share.
type ShareMetadata proton.ShareMetadata

// Share represents a fully-resolved Proton Drive share with its keyring.
type Share struct {
	Link        *Link
	keyRing     *crypto.KeyRing
	protonShare *proton.Share
	resolver    LinkResolver
}

// GetName returns the decrypted name of the share's root link.
func (s *Share) GetName(_ context.Context) (string, error) {
	return s.Link.Name()
}

// Metadata returns the share's metadata (type, state, flags, creator, etc.).
func (s *Share) Metadata() proton.ShareMetadata {
	return s.protonShare.ShareMetadata
}

// ListChildren returns the child links of the share's root folder.
func (s *Share) ListChildren(ctx context.Context, all bool) ([]*Link, error) {
	slog.Debug("share.ListChildren", "all", all)
	return s.Link.ListChildren(ctx, all)
}

// ResolvePath resolves a slash-separated path relative to the share's root link.
func (s *Share) ResolvePath(ctx context.Context, path string, all bool) (*Link, error) {
	slog.Debug("share.ResolvePath", "path", path, "all", all)
	return s.Link.ResolvePath(ctx, path, all)
}

func (s *Share) getKeyRing() (*crypto.KeyRing, error) {
	linkKR, ok := s.resolver.AddressKeyRing(s.protonShare.AddressID)
	if !ok {
		return nil, api.ErrKeyNotFound
	}
	return s.protonShare.GetKeyRing(linkKR)
}
