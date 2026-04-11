package proton

import (
	"errors"
)

var (
	ErrorMissingUID          = errors.New("missing UID")
	ErrMissingAccessToken    = errors.New("missing access token")
	ErrMissingRefreshToken   = errors.New("missing refresh token")
	ErrKeyNotFound           = errors.New("key not found")
	ErrNotLoggedIn           = errors.New("not logged in")
	ErrFileNotFound          = errors.New("file not found")
	ErrNotAFolder            = errors.New("not a folder")
	ErrInvalidPath           = errors.New("invalid path")
)
