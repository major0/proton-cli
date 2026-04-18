package api

import (
	"testing"

	"github.com/ProtonMail/go-proton-api"
)

// TestAddressTypeString verifies AddressType.String() for all known types
// and an unknown value.
func TestAddressTypeString(t *testing.T) {
	tests := []struct {
		name string
		typ  AddressType
		want string
	}{
		{"original", AddressType(proton.AddressTypeOriginal), "original"},
		{"alias", AddressType(proton.AddressTypeAlias), "alias"},
		{"custom", AddressType(proton.AddressTypeCustom), "custom"},
		{"premium", AddressType(proton.AddressTypePremium), "premium"},
		{"external", AddressType(proton.AddressTypeExternal), "external"},
		{"unknown", AddressType(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.String()
			if got != tt.want {
				t.Fatalf("AddressType(%d).String() = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}

// TestAddressStatusString verifies AddressStatus.String() for all known
// statuses and an out-of-range value.
func TestAddressStatusString(t *testing.T) {
	tests := []struct {
		name   string
		status AddressStatus
		want   string
	}{
		{"disabled", AddressStatus(proton.AddressStatusDisabled), "disabled"},
		{"enabled", AddressStatus(proton.AddressStatusEnabled), "enabled"},
		{"deleting", AddressStatus(proton.AddressStatusDeleting), "deleting"},
		{"unknown", AddressStatus(99), "Unknown Status (99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.want {
				t.Fatalf("AddressStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
