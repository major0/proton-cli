package proton

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type Link struct {
	Name string

	Type proton.LinkType

	XAttr *proton.RevisionXAttrCommon

	Size int64

	State *proton.LinkState

	ModifyTime     int64
	CreateTime     int64
	ExpirationTime int64

	keyRing     *crypto.KeyRing
	nameKeyRing *crypto.KeyRing
	parentLink  *Link
	protonLink  *proton.Link
	session     *Session
	share       *Share
}

func (l *Link) newLink(ctx context.Context, pLink *proton.Link) (*Link, error) {
	slog.Debug("link.newLink", "linkID", pLink.LinkID)
	return l.session.newLink(ctx, l.share, l, pLink)
}

func (l *Link) ListChildren(ctx context.Context, all bool) ([]Link, error) {
	slog.Debug("link.ListChildren", "all", all)
	//fmt.Println("link.ListChildren: pLink = %#v", l.protonLink)
	//fmt.Println("link.ListChildren: share = %#v", l.share)
	//fmt.Println("link.ListChildren: session = %#v", l.session)

	pChildren, err := l.session.Client.ListChildren(ctx, l.share.protonShare.ShareID, l.protonLink.LinkID, all)
	if err != nil {
		return nil, err
	}

	children := make([]Link, len(pChildren))
	for i := range pChildren {
		link, err := l.newLink(ctx, &pChildren[i])
		if err != nil {
			return nil, err
		}
		children[i] = *link
	}

	return children, nil
}

// We try to handle a specific set of circumstances here:
// - `path/to/file` - Return the file link.
// - `path/to/directory` - Return the folder link.
// - `path/to-directory/` - Return a list of child links of the folder.
func (l *Link) resolveParts(ctx context.Context, parts []string, all bool) (*Link, error) {
	slog.Debug("link.resolveParts", "parts", parts)
	slog.Debug("link.resolveParts", "len(parts)", len(parts))
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		slog.Debug("link.resolveParts", "returning", true)
		return l, nil
	}

	if proton.LinkType(l.Type) != proton.LinkTypeFolder {
		return nil, ErrNotAFolder
	}

	children, err := l.ListChildren(ctx, all)
	if err != nil {
		return nil, err
	}

	slog.Debug("link.resolveParts", "children", len(children))

	for _, child := range children {
		slog.Debug("link.resolveParts", "child.Name", child.Name)
		if child.Name == parts[0] {
			return child.resolveParts(ctx, parts[1:], all)
		}
	}

	return nil, ErrFileNotFound
}

func (l *Link) ResolvePath(ctx context.Context, path string, all bool) (*Link, error) {
	slog.Debug("link.ResolvePath", "path", path, "all", all)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	return l.resolveParts(ctx, parts, all)
}

func (l *Link) getParentKeyRing() (*crypto.KeyRing, error) {
	slog.Debug("link.getParentKeyRing", "address", l.protonLink.NameSignatureEmail)
	if l.parentLink == nil {
		slog.Debug("link.getParentKeyRing", "share", true)
		return l.share.getKeyRing()
	} else {
		slog.Debug("link.getParentKeyRing", "share", false)
		return l.parentLink.nameKeyRing, nil
	}
}

func (l *Link) getKeyRing(address string) (*crypto.KeyRing, error) {
	slog.Debug("link.getKeyRing", "address", l.protonLink.SignatureEmail)

	parentKR, err := l.getParentKeyRing()
	if err != nil {
		return nil, err
	}

	if addr, ok := l.session.addresses[l.protonLink.SignatureEmail]; ok {
		if linkKR, ok := l.session.AddressKeyRing[addr.ID]; ok {
			return l.protonLink.GetKeyRing(parentKR, linkKR)
		}
	}

	return nil, ErrKeyNotFound
}

func (l *Link) getAddrKeyRing(address string) (*crypto.KeyRing, error) {
	slog.Debug("link.getAddrKeyRing", "address", address)
	if addr, ok := l.session.addresses[l.protonLink.NameSignatureEmail]; ok {
		if addrKR, ok := l.session.AddressKeyRing[addr.ID]; ok {
			return addrKR, nil
		}
	}

	return nil, ErrKeyNotFound
}

func (l *Link) getName() (string, error) {
	slog.Debug("link.getName")

	parentKR, err := l.getParentKeyRing()
	if err != nil {
		return "", err
	}

	addrKR, err := l.getAddrKeyRing(l.protonLink.NameSignatureEmail)
	if err != nil {
		return "", err
	}

	name, err := l.protonLink.GetName(parentKR, addrKR)
	return name, err
}
