package main

import (
  "github.com/major0/protondrive-cli/cmd"
  _ "github.com/major0/protondrive-cli/cmd/auth"
  //_ "github.com/major0/protondrive-cli/cmd/copy"
  //_ "github.com/major0/protondrive-cli/cmd/find"
  //_ "github.com/major0/protondrive-cli/cmd/mkdir"
  //_ "github.com/major0/protondrive-cli/cmd/move"
  //_ "github.com/major0/protondrive-cli/cmd/purge"
  //_ "github.com/major0/protondrive-cli/cmd/rename"
  //_ "github.com/major0/protondrive-cli/cmd/remove"
  //_ "github.com/major0/protondrive-cli/cmd/revisions"
  //_ "github.com/major0/protondrive-cli/cmd/shares"
  //_ "github.com/major0/protondrive-cli/cmd/volumes"
)

func main() {
  pdcli.Execute()
}
