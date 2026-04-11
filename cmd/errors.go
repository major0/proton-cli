package cli

import (
	"fmt"
)

var (
	// ErrNoAccount indicates that no account was specified.
	ErrNoAccount = fmt.Errorf("no account specified")
	// ErrNoConfig indicates that no config file was specified.
	ErrNoConfig = fmt.Errorf("no config file specified")
	// ErrNoSession indicates that no session file was specified.
	ErrNoSession = fmt.Errorf("no session file specified")
	// ErrNoTimeout indicates that no timeout was specified.
	ErrNoTimeout = fmt.Errorf("no timeout specified")
)
