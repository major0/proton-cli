package api

import (
	"errors"
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
