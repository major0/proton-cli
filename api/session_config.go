package api

import "time"

// SessionConfig represents the minimum data to restore and unlock a session.
// It contains the credentials (UID, AccessToken, RefreshToken) plus the
// SaltedKeyPass required for the Unlock operation.
type SessionConfig struct {
	UID           string         `json:"uid"`
	AccessToken   string         `json:"access_token"`
	RefreshToken  string         `json:"refresh_token"`
	SaltedKeyPass string         `json:"salted_key_pass"`
	Cookies       []serialCookie `json:"cookies,omitempty"`
	LastRefresh   time.Time      `json:"last_refresh,omitempty"`
	Service       string         `json:"service,omitempty"`
}
