package proton

import (
	"errors"
)

var (
	ErrorMissingUID          = errors.New("missing UID")
	ErrorMissingAccessToken  = errors.New("missing access token")
	ErrorMissingRefreshToken = errors.New("missing refresh token")
	ErrKeyNotFound           = errors.New("key not found")
	ErrFileNotFound          = errors.New("file not found")
	ErrNotAFolder            = errors.New("not a folder")
	ErrInvalidPath           = errors.New("invalid path")
)
