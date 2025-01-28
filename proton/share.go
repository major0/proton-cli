package proton

import (
	"context"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type ShareMetadata proton.ShareMetadata

type Share struct {
	session     *Session
	Link        *Link
	ProtonShare *proton.Share

	shareAddrKR *crypto.KeyRing
	shareKR     *crypto.KeyRing
}

func (s *Share) GetName(ctx context.Context) string {
	return s.Link.Name
}

func (s *Share) GetLink(ctx context.Context, id string) (Link, error) {
	if s.ProtonShare.LinkID == id {
		return *s.Link, nil
	}

	return s.session.GetLink(ctx, s.ProtonShare.ShareID, id)
}
