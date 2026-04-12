// Package drive provides Proton Drive resource types and interfaces.
package drive

import "errors"

var (
	// ErrFileNotFound indicates that the requested file or link was not found.
	ErrFileNotFound = errors.New("file not found")
	// ErrNotAFolder indicates that the target link is not a folder.
	ErrNotAFolder = errors.New("not a folder")
	// ErrNotEmpty indicates that the directory is not empty.
	ErrNotEmpty = errors.New("directory not empty")
	// ErrInvalidPath indicates that the provided path is malformed.
	ErrInvalidPath = errors.New("invalid path")
	// ErrSkipDir is returned by WalkFunc to skip a directory subtree.
	ErrSkipDir = errors.New("skip directory")
)
