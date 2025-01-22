package proton

import (
	"context"
	"log/slog"

	p "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type Session struct {
	Client *p.Client
	Auth   p.Auth

	Address        []p.Address
	AddressKeyRing map[string]*crypto.KeyRing
	manager        *p.Manager
	User           p.User
	UserKeyRing    *crypto.KeyRing
}

func SessionFromCredentials(ctx context.Context, options []p.Option, creds *SessionCredentials) (*Session, error) {
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

	session.manager = p.New(options...)

	slog.Debug("config", "uid", creds.UID, "access_token", creds.AccessToken, "refresh_token", creds.RefreshToken)
	session.Client = session.manager.NewClient(creds.UID, creds.AccessToken, creds.RefreshToken)

	slog.Debug("GetUser")
	session.User, err = session.Client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug("GetAddresses")
	session.Address, err = session.Client.GetAddresses(ctx)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func SessionFromLogin(ctx context.Context, options []p.Option, username string, password string) (*Session, error) {
	var err error
	session := &Session{}
	session.manager = p.New(options...)
	slog.Debug("login", "username", username, "password", "<hidden>")
	session.Client, session.Auth, err = session.manager.NewClientWithLogin(ctx, username, []byte(password))
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Session) Unlock(keypass string) error {
	var err error
	s.UserKeyRing, s.AddressKeyRing, err = p.Unlock(s.User, s.Address, []byte(keypass), nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) AddAuthHandler(handler p.AuthHandler) {
	s.Client.AddAuthHandler(handler)
}

func (s *Session) AddDeauthHandler(handler p.Handler) {
	s.Client.AddDeauthHandler(handler)
}

func (s *Session) Stop() {
	s.manager.Close()
}
