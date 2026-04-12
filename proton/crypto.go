package proton

import (
	"encoding/base64"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
)

// generateNodeKeys creates a new node key pair for a Drive link. The
// passphrase is encrypted with parentKR and signed with addrKR.
// Returns (armoredKey, encryptedPassphrase, passphraseSignature).
func generateNodeKeys(parentKR, addrKR *crypto.KeyRing) (string, string, string, error) {
	passphrase, err := crypto.RandomToken(32)
	if err != nil {
		return "", "", "", err
	}

	passphraseB64 := base64.StdEncoding.EncodeToString(passphrase)

	key, err := helper.GenerateKey("Drive key", "noreply@protonmail.com", []byte(passphraseB64), "x25519", 0)
	if err != nil {
		return "", "", "", err
	}

	plainPassphrase := crypto.NewPlainMessage([]byte(passphraseB64))

	enc, err := parentKR.Encrypt(plainPassphrase, nil)
	if err != nil {
		return "", "", "", err
	}

	encArm, err := enc.GetArmored()
	if err != nil {
		return "", "", "", err
	}

	sig, err := addrKR.SignDetached(plainPassphrase)
	if err != nil {
		return "", "", "", err
	}

	sigArm, err := sig.GetArmored()
	if err != nil {
		return "", "", "", err
	}

	return key, encArm, sigArm, nil
}

// unlockKeyRing decrypts a node key using the parent keyring and the
// encrypted passphrase. The signature is verified against addrKR.
func unlockKeyRing(parentKR, addrKR *crypto.KeyRing, key, passphrase, passphraseSig string) (*crypto.KeyRing, error) {
	enc, err := crypto.NewPGPMessageFromArmored(passphrase)
	if err != nil {
		return nil, err
	}

	dec, err := parentKR.Decrypt(enc, nil, crypto.GetUnixTime())
	if err != nil {
		return nil, err
	}

	sig, err := crypto.NewPGPSignatureFromArmored(passphraseSig)
	if err != nil {
		return nil, err
	}

	if err := addrKR.VerifyDetached(dec, sig, crypto.GetUnixTime()); err != nil {
		return nil, err
	}

	lockedKey, err := crypto.NewKeyFromArmored(key)
	if err != nil {
		return nil, err
	}

	unlockedKey, err := lockedKey.Unlock(dec.GetBinary())
	if err != nil {
		return nil, err
	}

	return crypto.NewKeyRing(unlockedKey)
}

// addrKRForLink returns the address keyring for the link's signature email.
// Falls back to the first available address keyring.
func (s *Session) addrKRForLink(l *Link) *crypto.KeyRing {
	if addr, ok := s.addresses[l.protonLink.SignatureEmail]; ok {
		if kr, ok := s.AddressKeyRing[addr.ID]; ok {
			return kr
		}
	}
	for _, kr := range s.AddressKeyRing {
		return kr
	}
	return nil
}

// signatureAddress returns the signature email address for the link.
func (s *Session) signatureAddress(l *Link) string {
	if l.protonLink.SignatureEmail != "" {
		return l.protonLink.SignatureEmail
	}
	for email := range s.addresses {
		return email
	}
	return ""
}
