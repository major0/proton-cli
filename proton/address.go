package proton

import (
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

// AddressType represents the type of a Proton email address.
type AddressType proton.AddressType

func (a AddressType) String() string {
	statusStrings := [...]string{"original", "alias", "custom", "premium", "external"}
	t := proton.AddressType(a)
	if t < proton.AddressTypeOriginal || t > proton.AddressTypeExternal {
		return fmt.Sprintf("Unknown Status (%d)", a)
	}

	// Proton API defines the start type as `iota + 1`
	return statusStrings[a-1]
}

// AddressStatus represents the status of a Proton email address.
type AddressStatus proton.AddressStatus

func (s AddressStatus) String() string {
	statusStrings := [...]string{"disabled", "enabled", "deleting"}
	t := proton.AddressStatus(s)
	if t < proton.AddressStatusDisabled || t > proton.AddressStatusDeleting {
		return fmt.Sprintf("Unknown Status (%d)", s)
	}
	return statusStrings[t]
}
