package accountCmd

import (
	"context"
	"errors"
	"log/slog"

	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

// logoutAccountStoreFn loads the account config to check CookieAuth.
// It is a variable so tests can replace it.
var logoutAccountStoreFn = func() (*common.SessionConfig, error) {
	return cli.AccountStoreVar.Load()
}

// logoutCookieDeleteFn deletes the cookie store entry.
// It is a variable so tests can replace it.
var logoutCookieDeleteFn = func() error {
	return cli.CookieStoreVar.Delete()
}

var authLogoutForce = false
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout of Proton",
	Long:  `Logout of Proton`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		session, err := cli.RestoreSession(ctx)
		if err != nil && !errors.Is(err, common.ErrNotLoggedIn) && !authLogoutForce {
			return err
		}

		if err := common.SessionRevoke(ctx, session, cli.SessionStoreVar, authLogoutForce); err != nil {
			return err
		}

		// Clean up cookie store. Log warning on failure — don't fail the logout.
		if err := logoutCookieDeleteFn(); err != nil {
			slog.Warn("logout: cookie store delete failed", "error", err)
		}

		return nil
	},
}

func init() {
	accountCmd.AddCommand(authLogoutCmd)
	cli.BoolFlagP(authLogoutCmd.Flags(), &authLogoutForce, "force", "f", false, "Force logout of Proton")
}
