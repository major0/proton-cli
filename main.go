// Package main is the entry point for the proton-cli application.
package main

import (
	cli "github.com/major0/proton-cli/cmd"
	_ "github.com/major0/proton-cli/cmd/account"

	// _ "github.com/major0/proton-cli/cmd/calendar"
	_ "github.com/major0/proton-cli/cmd/drive"
	// _ "github.com/major0/proton-cli/cmd/mail"
	// _ "github.com/major0/proton-cli/cmd/pass"
	_ "github.com/major0/proton-cli/cmd/drive/share"
	// _ "github.com/major0/proton-cli/cmd/wallet"
)

func main() {
	cli.Execute()
}
