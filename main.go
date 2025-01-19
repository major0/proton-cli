package main

import (
	cli "github.com/major0/proton-cli/cmd"
	_ "github.com/major0/proton-cli/cmd/account"

	//_ "github.com/major0/proton-cli/cmd/calendar"
	//_ "github.com/major0/proton-cli/cmd/drive"
	//_ "github.com/major0/proton-cli/cmd/mail"
	//_ "github.com/major0/proton-cli/cmd/pass"
	_ "github.com/major0/proton-cli/cmd/share"
	_ "github.com/major0/proton-cli/cmd/volume"
	//_ "github.com/major0/proton-cli/cmd/wallet"
)

func main() {
	cli.Execute()
}
