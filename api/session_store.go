// Package api provides the core API client library for proton-cli.
package api

// SessionStore defines the interface for persisting and retrieving session data.
type SessionStore interface {
	Load() (*SessionConfig, error)
	Save(session *SessionConfig) error
	Delete() error
	List() ([]string, error)
	Switch(account string) error
}
