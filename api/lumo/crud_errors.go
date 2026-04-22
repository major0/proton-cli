package lumo

import "errors"

var (
	// ErrNotEligible indicates the account is not eligible for Lumo.
	ErrNotEligible = errors.New("lumo: account not eligible")
	// ErrNotFound indicates the requested resource was deleted (422/2501).
	ErrNotFound = errors.New("lumo: resource not found")
	// ErrConflict indicates a duplicate resource (409).
	ErrConflict = errors.New("lumo: resource conflict")
)
