package proton

import (
	"context"
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type Session struct {
	Client  *proton.Client
	Auth    proton.Auth
	manager *proton.Manager

	address        []proton.Address
	AddressKeyRing map[string]*crypto.KeyRing

	user        proton.User
	UserKeyRing *crypto.KeyRing
}

/* Initialize a new session frmo the provided credentials. The session is
 * not fully usable until it has been Unlock()'ed using the user-provided
 * keypass */
func SessionFromCredentials(ctx context.Context, options []proton.Option, creds *SessionCredentials) (*Session, error) {
	var err error

	// Initialize the client from our cahced credentials
	if creds.UID == "" {
		return nil, ErrorMissingUID
	}

	if creds.AccessToken == "" {
		return nil, ErrorMissingAccessToken
	}

	if creds.RefreshToken == "" {
		return nil, ErrorMissingRefreshToken
	}

	var session Session

	slog.Debug("refresh client")

	session.manager = proton.New(options...)

	slog.Debug("config", "uid", creds.UID, "access_token", creds.AccessToken, "refresh_token", creds.RefreshToken)
	session.Client = session.manager.NewClient(creds.UID, creds.AccessToken, creds.RefreshToken)

	slog.Debug("GetUser")
	session.user, err = session.Client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug("GetAddresses")
	session.address, err = session.Client.GetAddresses(ctx)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

/* Initialize a new session from the provided login/password. The returned
 * session may have extra authentication requirements, such as 2FA.
 * Once all authentication challenges have been met, the session will still
 * need to be Unlock()'ed to gain access to the User and Address
 * keyrings. */
func SessionFromLogin(ctx context.Context, options []proton.Option, username string, password string) (*Session, error) {
	var err error
	session := &Session{}
	session.manager = proton.New(options...)
	slog.Debug("login", "username", username, "password", "<hidden>")
	session.Client, session.Auth, err = session.manager.NewClientWithLogin(ctx, username, []byte(password))
	if err != nil {
		return nil, err
	}

	return session, nil
}

/* Unlock the user's account keyring, as well as all keyring's associated
 * with alternate addresses. */
func (s *Session) Unlock(keypass string) error {
	var err error
	s.UserKeyRing, s.AddressKeyRing, err = proton.Unlock(s.user, s.address, []byte(keypass), nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) AddAuthHandler(handler proton.AuthHandler) {
	s.Client.AddAuthHandler(handler)
}

func (s *Session) AddDeauthHandler(handler proton.Handler) {
	s.Client.AddDeauthHandler(handler)
}

func (s *Session) Stop() {
	s.manager.Close()
}
