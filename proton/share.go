package proton

import (
	"context"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type ShareMetadata proton.ShareMetadata

type Share struct {
	session *Session
	link    *Link
	pShare  *proton.Share

	shareAddrKR *crypto.KeyRing
	shareKR     *crypto.KeyRing
}

func (s *Share) GetName(ctx context.Context) string {
	return s.link.Name
}

func (s *Share) GetLink(ctx context.Context, id string) (Link, error) {
	if s.pShare.LinkID == id {
		return *s.link, nil
	}

	return s.session.GetLink(ctx, s.pShare.ShareID, id)
}
