package proton

import (
	"context"
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type ShareMetadata proton.ShareMetadata

type Share struct {
	Link        *Link
	keyRing     *crypto.KeyRing
	protonShare *proton.Share
	session     *Session
}

func (s *Share) GetName(ctx context.Context) string {
	slog.Debug("share.GetName")
	return s.Link.Name
}

func (s *Share) ListChildren(ctx context.Context, all bool) ([]Link, error) {
	slog.Debug("share.ListChildren", "all", all)
	return s.Link.ListChildren(ctx, all)
}

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
