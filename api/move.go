package api

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

// Move moves a link (file or folder) to a new parent directory with a new name.
// The node passphrase is re-encrypted from the old parent's keyring to the new
// parent's keyring.
func (s *Session) Move(ctx context.Context, share *Share, link *Link, newParent *Link, newName string) error {
	if newParent.Type() != proton.LinkTypeFolder {
		return ErrNotAFolder
	}

	newParentKR, err := newParent.KeyRing()
	if err != nil {
		return fmt.Errorf("move: new parent keyring: %w", err)
	}

	addrKR := s.addrKRForLink(link)
	if addrKR == nil {
		return fmt.Errorf("move: %w", ErrKeyNotFound)
	}

	req := proton.MoveLinkReq{
		ParentLinkID:            newParent.protonLink.LinkID,
		OriginalHash:            link.protonLink.Hash,
		NodePassphraseSignature: link.protonLink.NodePassphraseSignature,
		SignatureAddress:        s.signatureAddress(link),
	}

	// Encrypt the new name with the new parent's keyring.
	if err := req.SetName(newName, addrKR, newParentKR); err != nil {
		return fmt.Errorf("move: encrypting name: %w", err)
	}

	// Compute the name hash using the new parent's hash key.
	hashKey, err := newParent.protonLink.GetHashKeyFromParent(newParentKR, addrKR)
	if err != nil {
		return fmt.Errorf("move: hash key: %w", err)
	}
	if err := req.SetHash(newName, hashKey); err != nil {
		return fmt.Errorf("move: hash: %w", err)
	}

	// Re-encrypt the node passphrase from old parent to new parent.
	oldParentKR, err := link.getParentKeyRing()
	if err != nil {
		return fmt.Errorf("move: old parent keyring: %w", err)
	}

	newPassphrase, err := reencryptKeyPacket(oldParentKR, newParentKR, link.protonLink.NodePassphrase)
	if err != nil {
		return fmt.Errorf("move: re-encrypting passphrase: %w", err)
	}
	req.NodePassphrase = newPassphrase

	return s.Client.MoveLink(ctx, share.protonShare.ShareID, link.protonLink.LinkID, req)
}

// Rename renames a link in place (same parent directory).
func (s *Session) Rename(ctx context.Context, share *Share, link *Link, newName string) error {
	if link.parentLink == nil {
		return fmt.Errorf("rename: cannot rename share root")
	}
	return s.Move(ctx, share, link, link.parentLink, newName)
}

// reencryptKeyPacket re-encrypts an armored PGP message from srcKR to dstKR.
func reencryptKeyPacket(srcKR, dstKR *crypto.KeyRing, passphrase string) (string, error) {
	oldSplit, err := crypto.NewPGPSplitMessageFromArmored(passphrase)
	if err != nil {
		return "", err
	}

	sessionKey, err := srcKR.DecryptSessionKey(oldSplit.KeyPacket)
	if err != nil {
		return "", err
	}

	newKeyPacket, err := dstKR.EncryptSessionKey(sessionKey)
	if err != nil {
		return "", err
	}

	newSplit := crypto.NewPGPSplitMessage(newKeyPacket, oldSplit.DataPacket)
	return newSplit.GetArmored()
}
