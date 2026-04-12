package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-proton-api"
)

// MkDir creates a new folder under the given parent link. Returns the
// newly created Link (lazily decrypted). The parent must be a folder.
func (s *Session) MkDir(ctx context.Context, share *Share, parent *Link, name string) (*Link, error) {
	if parent.Type() != proton.LinkTypeFolder {
		return nil, ErrNotAFolder
	}

	parentKR, err := parent.KeyRing()
	if err != nil {
		return nil, fmt.Errorf("mkdir %s: parent keyring: %w", name, err)
	}

	addrKR := s.addrKRForLink(parent)
	if addrKR == nil {
		return nil, fmt.Errorf("mkdir %s: %w", name, ErrKeyNotFound)
	}

	nodeKey, nodePassphraseEnc, nodePassphraseSig, err := generateNodeKeys(parentKR, addrKR)
	if err != nil {
		return nil, fmt.Errorf("mkdir %s: generating keys: %w", name, err)
	}

	req := proton.CreateFolderReq{
		ParentLinkID:            parent.protonLink.LinkID,
		NodeKey:                 nodeKey,
		NodePassphrase:          nodePassphraseEnc,
		NodePassphraseSignature: nodePassphraseSig,
		SignatureAddress:        s.signatureAddress(parent),
	}

	if err := req.SetName(name, addrKR, parentKR); err != nil {
		return nil, fmt.Errorf("mkdir %s: encrypting name: %w", name, err)
	}

	hashKey, err := parent.protonLink.GetHashKey(parentKR)
	if err != nil {
		return nil, fmt.Errorf("mkdir %s: hash key: %w", name, err)
	}
	if err := req.SetHash(name, hashKey); err != nil {
		return nil, fmt.Errorf("mkdir %s: hash: %w", name, err)
	}

	newNodeKR, err := unlockKeyRing(parentKR, addrKR, nodeKey, nodePassphraseEnc, nodePassphraseSig)
	if err != nil {
		return nil, fmt.Errorf("mkdir %s: unlock keyring: %w", name, err)
	}
	if err := req.SetNodeHashKey(newNodeKR); err != nil {
		return nil, fmt.Errorf("mkdir %s: node hash key: %w", name, err)
	}

	res, err := s.Client.CreateFolder(ctx, share.protonShare.ShareID, req)
	if err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", name, err)
	}

	return s.StatLink(ctx, share, parent, res.ID)
}

// MkDirAll creates a directory path, creating any missing intermediate
// directories. Like mkdir -p. Returns the final (deepest) Link.
func (s *Session) MkDirAll(ctx context.Context, share *Share, root *Link, path string) (*Link, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return root, nil
	}

	parts := strings.Split(path, "/")
	current := root

	for _, name := range parts {
		if name == "" || name == "." {
			continue
		}

		child, err := current.Lookup(ctx, name)
		if err != nil {
			return nil, err
		}

		if child != nil {
			if child.Type() != proton.LinkTypeFolder {
				return nil, fmt.Errorf("mkdir -p: %s: %w", name, ErrNotAFolder)
			}
			current = child
			continue
		}

		newDir, err := s.MkDir(ctx, share, current, name)
		if err != nil {
			if errors.Is(err, proton.ErrFolderNameExist) {
				child, findErr := current.Lookup(ctx, name)
				if findErr != nil {
					return nil, findErr
				}
				if child != nil {
					current = child
					continue
				}
			}
			return nil, err
		}

		current = newDir
	}

	return current, nil
}
