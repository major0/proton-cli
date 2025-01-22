package cli

import (
	"fmt"
)

var (
	ErrNoAccount   = fmt.Errorf("no account specified")
	ErrNoConfig    = fmt.Errorf("no config file specified")
	ErrNoSession   = fmt.Errorf("no session file specified")
	ErrNoTimeout   = fmt.Errorf("no timeout specified")
	ErrNotLoggedIn = fmt.Errorf("not logged in")
)
