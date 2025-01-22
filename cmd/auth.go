package cli

import (
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/internal"
)

/* The authHandler is periodically called by the underyling Proton Client
 * whenever an authentication refresh has been performed. Whenever this
 * happens we need to update our in-memory session config as well as
 * update our session cache.
 *
 * FIXME should this be protected by a mutex? */
func authHandler(auth proton.Auth) {
	// Save the login credentials into our app cache
	slog.Debug("auth", "uid", auth.UID, "access_token", auth.AccessToken, "refresh_token", auth.RefreshToken)

	sessionStore := internal.NewFileStore(rootParams.SessionFile, rootParams.Account)

	// Read the previous session store so we don't lose the SaltedKeyPass
	sessionConfig, err := sessionStore.Load()
	if err != nil {
		slog.Any("error", err)
		return
	}
	sessionConfig.UID = auth.UID
	sessionConfig.AccessToken = auth.AccessToken
	sessionConfig.RefreshToken = auth.RefreshToken
	_ = sessionStore.Save(sessionConfig)
}

/* Similar to the authHandler, the deauthHandler is called by the Proton
 * Client. It is not entirely clear what we should be doing here? Possibly
 * we should purge the current session cache. For now we only log that
 * a deauth call was made. */
func deauthHandler() {
	// Currently do nothing
	slog.Debug("deauth")
}
