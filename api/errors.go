package api

import (
	"errors"
	"fmt"
)

var (
	// ErrMissingUID indicates that the session credentials are missing a UID.
	ErrMissingUID = errors.New("missing UID")
	// ErrMissingAccessToken indicates that the session credentials are missing an access token.
	ErrMissingAccessToken = errors.New("missing access token")
	// ErrMissingRefreshToken indicates that the session credentials are missing a refresh token.
	ErrMissingRefreshToken = errors.New("missing refresh token")
	// ErrKeyNotFound indicates that the requested key was not found.
	ErrKeyNotFound = errors.New("key not found")
	// ErrNotLoggedIn indicates that no active session exists.
	ErrNotLoggedIn = errors.New("not logged in")
)

// Error represents a non-success response from the Proton API.
type Error struct {
	Status  int    // HTTP status code
	Code    int    // Proton API error code
	Message string // error description from the API
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("api: %d/%d: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("api: %d/%d", e.Status, e.Code)
}
