package api

import (
	"log/slog"
	"time"

	"github.com/ProtonMail/go-proton-api"
)

// NewAuthHandler returns a proton.AuthHandler that persists updated tokens
// and cookies to the session store. Uses the session's own cookie jar.
func NewAuthHandler(store SessionStore, session *Session) proton.AuthHandler {
	return func(auth proton.Auth) {
		slog.Debug("auth", "uid", auth.UID, "access_token", auth.AccessToken, "refresh_token", auth.RefreshToken)

		sessionConfig, err := store.Load()
		if err != nil {
			slog.Error("auth handler: loading session config", "error", err)
			return
		}

		sessionConfig.UID = auth.UID
		sessionConfig.AccessToken = auth.AccessToken
		sessionConfig.RefreshToken = auth.RefreshToken
		sessionConfig.Cookies = serializeCookies(session.cookieJar, apiCookieURL())
		sessionConfig.LastRefresh = time.Now()

		if err := store.Save(sessionConfig); err != nil {
			slog.Error("auth handler: saving session config", "error", err)
		}
	}
}

// deauthHandler logs a deauth event. Matches the current behavior from
// cmd/auth.go — no recovery action is taken.
func deauthHandler() {
	slog.Debug("deauth")
}

// NewDeauthHandler returns a proton.Handler that logs deauth events.
func NewDeauthHandler() proton.Handler {
	return deauthHandler
}
