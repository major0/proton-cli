package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api/lumo"
)

// masterKeyFields holds the cached master key state. These fields are
// added to Client via embedding to keep the main struct clean.
type masterKeyFields struct {
	masterKey     []byte
	masterKeyOnce sync.Once
	masterKeyErr  error
}

// GetMasterKey returns the unwrapped master key, fetching and caching
// it on first call. Subsequent calls return the cached result.
func (c *Client) GetMasterKey(ctx context.Context) ([]byte, error) {
	c.masterKeyOnce.Do(func() {
		c.masterKey, c.masterKeyErr = c.fetchMasterKey(ctx)
	})
	if c.masterKeyErr != nil {
		return nil, c.masterKeyErr
	}
	// Return a copy to prevent callers from mutating the cached key.
	out := make([]byte, len(c.masterKey))
	copy(out, c.masterKey)
	return out, nil
}

// fetchMasterKey fetches master keys from the API, selects the best one,
// and PGP-decrypts it. If no keys exist, it creates a new one.
func (c *Client) fetchMasterKey(ctx context.Context) ([]byte, error) {
	var resp lumo.ListMasterKeysResponse
	err := c.Session.DoJSON(ctx, "GET", "/api/lumo/v1/masterkeys", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("lumo: get master keys: %w", err)
	}

	if resp.Eligibility != lumo.EligibilityEligible {
		return nil, fmt.Errorf("lumo: get master keys: %w", lumo.ErrNotEligible)
	}

	if len(resp.MasterKeys) == 0 {
		return c.createMasterKey(ctx)
	}

	best, err := lumo.SelectBestMasterKey(resp.MasterKeys)
	if err != nil {
		return nil, fmt.Errorf("lumo: select master key: %w", err)
	}

	return c.unwrapMasterKey(best.MasterKey)
}

// unwrapMasterKey PGP-decrypts an armored master key using the session's
// user keyring.
func (c *Client) unwrapMasterKey(armoredKey string) ([]byte, error) {
	msg, err := pgpcrypto.NewPGPMessageFromArmored(armoredKey)
	if err != nil {
		return nil, fmt.Errorf("lumo: parse master key: %w", err)
	}

	plain, err := c.Session.UserKeyRing.Decrypt(msg, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("lumo: decrypt master key: %w", err)
	}

	return plain.GetBinary(), nil
}

// createMasterKey generates a new 32-byte AES-KW key, PGP-encrypts it
// with the user's keyring, POSTs it to the API, and returns the raw bytes.
func (c *Client) createMasterKey(ctx context.Context) ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("lumo: generate master key: %w", err)
	}

	// PGP-encrypt the raw key bytes.
	plainMsg := pgpcrypto.NewPlainMessage(key)
	encMsg, err := c.Session.UserKeyRing.Encrypt(plainMsg, nil)
	if err != nil {
		return nil, fmt.Errorf("lumo: encrypt master key: %w", err)
	}

	armored, err := encMsg.GetArmored()
	if err != nil {
		return nil, fmt.Errorf("lumo: armor master key: %w", err)
	}

	req := lumo.CreateMasterKeyReq{MasterKey: armored}
	err = c.Session.DoJSON(ctx, "POST", "/api/lumo/v1/masterkeys", req, nil)
	if err != nil {
		return nil, fmt.Errorf("lumo: create master key: %w", err)
	}

	return key, nil
}

// GenerateSpaceKey generates a new 32-byte AES-GCM key for a space.
func GenerateSpaceKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("lumo: generate space key: %w", err)
	}
	return key, nil
}

// GenerateTag generates a new random tag (base64url-encoded 16 bytes).
func GenerateTag() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
