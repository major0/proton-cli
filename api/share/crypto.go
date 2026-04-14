package share

import (
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
)

// GenerateKeyPacket encrypts the share's session key for the invitee.
// It decrypts the share passphrase using shareKR, re-encrypts the resulting
// session key with inviteeKR, and signs the key packet with inviterAddrKR.
// Returns (keyPacketBase64, keyPacketSignatureArmored, error).
// The decrypted session key is not retained beyond this function call.
func GenerateKeyPacket(shareKR, inviterAddrKR, inviteeKR *crypto.KeyRing, sharePassphrase string) (string, string, error) {
	// Decrypt the share passphrase to obtain the session key material.
	enc, err := crypto.NewPGPMessageFromArmored(sharePassphrase)
	if err != nil {
		return "", "", fmt.Errorf("generate key packet: parse passphrase: %w", err)
	}

	dec, err := shareKR.Decrypt(enc, nil, crypto.GetUnixTime())
	if err != nil {
		return "", "", fmt.Errorf("generate key packet: decrypt passphrase: %w", err)
	}

	// Re-encrypt the session key material with the invitee's public key.
	plainMsg := crypto.NewPlainMessage(dec.GetBinary())
	encMsg, err := inviteeKR.Encrypt(plainMsg, nil)
	if err != nil {
		return "", "", fmt.Errorf("generate key packet: encrypt for invitee: %w", err)
	}

	keyPacketBytes := encMsg.GetBinary()
	keyPacketB64 := base64.StdEncoding.EncodeToString(keyPacketBytes)

	// Sign the encrypted key packet with the inviter's address key.
	sig, err := inviterAddrKR.SignDetached(crypto.NewPlainMessage(keyPacketBytes))
	if err != nil {
		return "", "", fmt.Errorf("generate key packet: sign: %w", err)
	}

	sigArmored, err := sig.GetArmored()
	if err != nil {
		return "", "", fmt.Errorf("generate key packet: armor signature: %w", err)
	}

	return keyPacketB64, sigArmored, nil
}

// GenerateShareCrypto produces all crypto fields needed for CreateDriveSharePayload.
// It generates a share key pair (passphrase encrypted with linkNodeKR and addrKR,
// signed by addrKR), then re-encrypts the link's passphrase session key and name
// session key to the new share key.
//
// Parameters:
//   - addrKR: the address keyring for signing and encryption
//   - linkNodeKR: the link's node keyring (parent of the new share key)
//   - parentKR: the parent keyring that can decrypt linkPassphrase and linkName
//   - linkPassphrase: the link's encrypted passphrase (armored PGP message)
//   - linkName: the link's encrypted name (armored PGP message)
//
// Returns the populated crypto fields. No decrypted key material is retained.
func GenerateShareCrypto(addrKR, linkNodeKR, parentKR *crypto.KeyRing,
	linkPassphrase, linkName string) (shareKey, sharePassphrase, sharePassphraseSignature,
	passphraseKeyPacket, nameKeyPacket string, err error) {

	// 1. Generate a random passphrase for the share key.
	rawPassphrase, err := crypto.RandomToken(32)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: random passphrase: %w", err)
	}
	passphraseB64 := base64.StdEncoding.EncodeToString(rawPassphrase)

	// 2. Generate a new PGP key pair for the share.
	shareKeyArmored, err := helper.GenerateKey("Share key", "noreply@protonmail.com", []byte(passphraseB64), "x25519", 0)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: generate key: %w", err)
	}

	// 3. Encrypt the passphrase with [linkNodeKR, addrKR] (link node key first).
	plainPassphrase := crypto.NewPlainMessage([]byte(passphraseB64))
	encPassphrase, err := linkNodeKR.Encrypt(plainPassphrase, addrKR)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: encrypt passphrase: %w", err)
	}
	encPassphraseArmored, err := encPassphrase.GetArmored()
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: armor passphrase: %w", err)
	}

	// 4. Sign the passphrase with addrKR.
	sig, err := addrKR.SignDetached(plainPassphrase)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: sign passphrase: %w", err)
	}
	sigArmored, err := sig.GetArmored()
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: armor signature: %w", err)
	}

	// 5. Unlock the share key to get the share keyring.
	lockedKey, err := crypto.NewKeyFromArmored(shareKeyArmored)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: parse share key: %w", err)
	}
	unlockedKey, err := lockedKey.Unlock([]byte(passphraseB64))
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: unlock share key: %w", err)
	}
	shareKR, err := crypto.NewKeyRing(unlockedKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: share keyring: %w", err)
	}

	// 6. Get the link passphrase session key and re-encrypt to share key.
	encLinkPassphrase, err := crypto.NewPGPMessageFromArmored(linkPassphrase)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: parse link passphrase: %w", err)
	}
	splitPassphrase, err := encLinkPassphrase.SeparateKeyAndData(len(encLinkPassphrase.GetBinary()), 0)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: split link passphrase: %w", err)
	}
	passphraseSessionKey, err := parentKR.DecryptSessionKey(splitPassphrase.GetBinaryKeyPacket())
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: decrypt passphrase session key: %w", err)
	}
	reEncPassphraseKP, err := shareKR.EncryptSessionKey(passphraseSessionKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: re-encrypt passphrase key packet: %w", err)
	}
	passphraseKPB64 := base64.StdEncoding.EncodeToString(reEncPassphraseKP)

	// 7. Get the link name session key and re-encrypt to share key.
	encLinkName, err := crypto.NewPGPMessageFromArmored(linkName)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: parse link name: %w", err)
	}
	splitName, err := encLinkName.SeparateKeyAndData(len(encLinkName.GetBinary()), 0)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: split link name: %w", err)
	}
	nameSessionKey, err := parentKR.DecryptSessionKey(splitName.GetBinaryKeyPacket())
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: decrypt name session key: %w", err)
	}
	reEncNameKP, err := shareKR.EncryptSessionKey(nameSessionKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("generate share crypto: re-encrypt name key packet: %w", err)
	}
	nameKPB64 := base64.StdEncoding.EncodeToString(reEncNameKP)

	// Zero intermediate key material.
	clear(rawPassphrase)

	return shareKeyArmored, encPassphraseArmored, sigArmored, passphraseKPB64, nameKPB64, nil
}
