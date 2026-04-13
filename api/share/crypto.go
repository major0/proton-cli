package share

import (
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
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
