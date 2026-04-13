package drive

import "github.com/ProtonMail/go-proton-api"

// NewTestLink creates a pre-decrypted Link for use in tests that need
// working Name() calls without real crypto infrastructure.
func NewTestLink(pLink *proton.Link, parent *Link, share *Share, resolver LinkResolver, name string) *Link {
	l := NewLink(pLink, parent, share, resolver)
	l.name = name
	l.decrypted = true
	return l
}
