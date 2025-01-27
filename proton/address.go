package proton

import (
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

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

type AddressStatus proton.AddressStatus

func (s AddressStatus) String() string {
	statusStrings := [...]string{"disabled", "enabled", "deleting"}
	t := proton.AddressStatus(s)
	if t < proton.AddressStatusDisabled || t > proton.AddressStatusDeleting {
		return fmt.Sprintf("Unknown Status (%d)", s)
	}
	return statusStrings[t]
}
