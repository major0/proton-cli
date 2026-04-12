package proton

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
	// ErrFileNotFound indicates that the requested file or link was not found.
	ErrFileNotFound = errors.New("file not found")
	// ErrNotAFolder indicates that the target link is not a folder.
	ErrNotAFolder = errors.New("not a folder")
	// ErrInvalidPath indicates that the provided path is malformed.
	ErrInvalidPath = errors.New("invalid path")
	// ErrSkipDir is returned by WalkFunc to skip a directory subtree.
	ErrSkipDir = errors.New("skip directory")
)
