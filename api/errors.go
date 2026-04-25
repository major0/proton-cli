package api

import (
	"encoding/json"
	"errors"
	"fmt"

	proton "github.com/ProtonMail/go-proton-api"
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
	Status  int             // HTTP status code
	Code    int             // Proton API error code
	Message string          // error description from the API
	Details json.RawMessage // optional error details from the API
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("api: %d/%d: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("api: %d/%d", e.Status, e.Code)
}

// IsHVError reports whether this error is a Human Verification challenge (code 9001).
func (e *Error) IsHVError() bool {
	return e.Code == 9001
}

// GetHVDetails parses the Details field into an APIHVDetails struct.
// Returns an error if this is not an HV error or if Details cannot be parsed.
func (e *Error) GetHVDetails() (*proton.APIHVDetails, error) {
	if !e.IsHVError() {
		return nil, errors.New("not an HV error")
	}
	var hv proton.APIHVDetails
	if err := json.Unmarshal(e.Details, &hv); err != nil {
		return nil, fmt.Errorf("unmarshal HV details: %w", err)
	}
	return &hv, nil
}
