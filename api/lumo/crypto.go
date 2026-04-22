package lumo

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/google/uuid"
)

const (
	// aesKeySize is the required key length for AES-GCM-256.
	aesKeySize = 32
	// aesNonceSize is the standard GCM nonce length.
	aesNonceSize = 12
)

// GenerateRequestKey returns 32 random bytes for AES-GCM-256.
func GenerateRequestKey() ([]byte, error) {
	key := make([]byte, aesKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("lumo: generate request key: %w", err)
	}
	return key, nil
}

// GenerateRequestID returns a new UUID v4 string.
func GenerateRequestID() string {
	return uuid.New().String()
}

// RequestAD returns the associated data string for encrypting request turns.
func RequestAD(requestID string) string {
	return "lumo.request." + requestID + ".turn"
}

// ResponseAD returns the associated data string for decrypting response chunks.
func ResponseAD(requestID string) string {
	return "lumo.response." + requestID + ".chunk"
}

// encryptAESGCM encrypts plaintext with AES-GCM-256 and returns
// base64-encoded nonce || ciphertext || tag.
func encryptAESGCM(plaintext, key, ad []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("lumo: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("lumo: gcm: %w", err)
	}
	nonce := make([]byte, aesNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("lumo: nonce: %w", err)
	}
	// Seal appends ciphertext+tag after nonce.
	sealed := gcm.Seal(nonce, nonce, plaintext, ad)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// decryptAESGCM decodes base64, splits nonce || ciphertext || tag,
// and decrypts with AES-GCM-256.
func decryptAESGCM(encoded string, key, ad []byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("lumo: base64 decode: %w", err)
	}
	if len(data) < aesNonceSize {
		return nil, fmt.Errorf("lumo: ciphertext too short: %w", ErrDecryptionFailed)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("lumo: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("lumo: gcm: %w", err)
	}
	nonce := data[:aesNonceSize]
	ciphertext := data[aesNonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, ad)
	if err != nil {
		return nil, fmt.Errorf("lumo: decrypt: %w", ErrDecryptionFailed)
	}
	return plaintext, nil
}

// EncryptTurns encrypts each turn's Content and Images with AES-GCM
// using the request key and request AD. Sets Encrypted=true on each turn.
func EncryptTurns(turns []Turn, key []byte, requestID string) ([]Turn, error) {
	ad := []byte(RequestAD(requestID))
	for i := range turns {
		content := turns[i].Content
		enc, err := encryptAESGCM([]byte(content), key, ad)
		if err != nil {
			return nil, fmt.Errorf("lumo: encrypt turn content: %w", err)
		}
		turns[i].Content = enc
		turns[i].Encrypted = true

		for j := range turns[i].Images {
			raw, err := base64.StdEncoding.DecodeString(turns[i].Images[j].Data)
			if err != nil {
				return nil, fmt.Errorf("lumo: decode image data: %w", err)
			}
			encImg, err := encryptAESGCM(raw, key, ad)
			if err != nil {
				return nil, fmt.Errorf("lumo: encrypt image data: %w", err)
			}
			turns[i].Images[j].Data = encImg
			turns[i].Images[j].Encrypted = true
		}
	}
	return turns, nil
}

// EncryptRequestKey PGP-encrypts the raw key bytes with the Lumo public
// key and returns the base64-encoded binary result.
func EncryptRequestKey(key []byte, armoredPubKey string) (string, error) {
	pgpKey, err := crypto.NewKeyFromArmored(armoredPubKey)
	if err != nil {
		return "", fmt.Errorf("lumo: parse lumo pubkey: %w", err)
	}
	keyRing, err := crypto.NewKeyRing(pgpKey)
	if err != nil {
		return "", fmt.Errorf("lumo: create keyring: %w", err)
	}
	msg, err := keyRing.Encrypt(crypto.NewPlainMessage(key), nil)
	if err != nil {
		return "", fmt.Errorf("lumo: encrypt request key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(msg.GetBinary()), nil
}

// DecryptTokenData decrypts the Content field of a token_data message
// when Encrypted is true. Modifies msg in place.
func DecryptTokenData(msg *GenerationResponseMessage, key []byte, requestID string) error {
	if !msg.Encrypted {
		return nil
	}
	ad := []byte(ResponseAD(requestID))
	plaintext, err := decryptAESGCM(msg.Content, key, ad)
	if err != nil {
		return fmt.Errorf("lumo: decrypt token_data: %w", err)
	}
	msg.Content = string(plaintext)
	msg.Encrypted = false
	return nil
}

// DecryptImageData decrypts the Data field of an image_data message
// when Encrypted is true. The decrypted bytes are base64-encoded back
// into the Data field. Modifies msg in place.
func DecryptImageData(msg *GenerationResponseMessage, key []byte, requestID string) error {
	if !msg.Encrypted {
		return nil
	}
	ad := []byte(ResponseAD(requestID))
	plaintext, err := decryptAESGCM(msg.Data, key, ad)
	if err != nil {
		return fmt.Errorf("lumo: decrypt image_data: %w", err)
	}
	msg.Data = base64.StdEncoding.EncodeToString(plaintext)
	msg.Encrypted = false
	return nil
}

// ZeroKey overwrites key bytes with zeros.
func ZeroKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}
