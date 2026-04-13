package api

import (
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

// AddressType represents the type of a Proton email address.
type AddressType proton.AddressType

func (a AddressType) String() string {
	switch proton.AddressType(a) {
	case proton.AddressTypeOriginal:
		return "original"
	case proton.AddressTypeAlias:
		return "alias"
	case proton.AddressTypeCustom:
		return "custom"
	case proton.AddressTypePremium:
		return "premium"
	case proton.AddressTypeExternal:
		return "external"
	default:
		return fmt.Sprintf("unknown(%d)", a)
	}
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
