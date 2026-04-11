package smoke

// Compile-time type assertions verifying the cmd package's public API surface.
// If any exported symbol is missing or has the wrong signature, this file fails
// to compile.
//
// Validates: Requirements 10.1, 10.2, 10.3, 10.4

import (
	"time"

	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

// Execute() — no args, no return.
var _ func() = cli.Execute

// AddCommand(*cobra.Command) — single arg, no return.
var _ func(*cobra.Command) = cli.AddCommand

// SessionRestore() (*Session, error)
var _ func() (*proton.Session, error) = cli.SessionRestore

// SessionLogin(username, password, mboxpass, twoFA string) (*Session, error)
var _ func(string, string, string, string) (*proton.Session, error) = cli.SessionLogin

// SessionRevoke(*Session, bool) error
var _ func(*proton.Session, bool) error = cli.SessionRevoke

// SessionList() ([]string, error)
var _ func() ([]string, error) = cli.SessionList

// Timeout is an exported time.Duration variable.
var _ time.Duration = cli.Timeout
