package drive

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api"
)

// ShareMetadata represents the metadata for a Proton Drive share.
type ShareMetadata proton.ShareMetadata

// ShareTypePhotos is the undocumented share type for Proton Photos.
const ShareTypePhotos proton.ShareType = 4

// FormatShareType returns a human-readable label for a share type.
func FormatShareType(st proton.ShareType) string {
	switch st {
	case proton.ShareTypeMain:
		return "main"
	case proton.ShareTypeStandard:
		return "shared"
	case proton.ShareTypeDevice:
		return "device"
	case ShareTypePhotos:
		return "photos"
	default:
		return fmt.Sprintf("unknown(%d)", st)
	}
}

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

// ProtonShare returns the raw proton.Share. Used by the client package
// for API operations that need raw share fields.
func (s *Share) ProtonShare() *proton.Share { return s.protonShare }

// KeyRingValue returns the share's keyring.
func (s *Share) KeyRingValue() *crypto.KeyRing { return s.keyRing }

// NewShare constructs a Share. Used by the client package.
func NewShare(pShare *proton.Share, keyRing *crypto.KeyRing, link *Link, resolver LinkResolver) *Share {
	return &Share{
		protonShare: pShare,
		keyRing:     keyRing,
		Link:        link,
		resolver:    resolver,
	}
}

func (s *Share) getKeyRing() (*crypto.KeyRing, error) {
	linkKR, ok := s.resolver.AddressKeyRing(s.protonShare.AddressID)
	if !ok {
		return nil, api.ErrKeyNotFound
	}
	return s.protonShare.GetKeyRing(linkKR)
}
