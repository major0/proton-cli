package proton

import (
	"context"
	"errors"
	"log/slog"

	p "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

/* The SessionConfig is the minimum data required for restarting a
 * session later. With the exception of the SaltedKeyPass, all of this
 * data is returned by the Client.Login() call, w/ the SaltedKeyPass
 * being a salt of the password+UID.
 *
 * After a succesful login there is a small time window in which the
 * application must call proton.Unlock() to unlock the account.
 * Failure to do so will timeout the authentication process and a new
 * login session will need to be established.
 *
 * WARNING: This information is sensitive and should not be stored in the
 *          clear text anywhere!
 *          See: https://github.com/major0/proton-cli/issues/7 */
type SessionConfig struct {
	UID           string `json:"uid"`
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	SaltedKeyPass string `json:"saltedkeypass"`
}

var (
	ErrorMissingUID          = errors.New("missing UID")
	ErrorMissingAccessToken  = errors.New("missing access token")
	ErrorMissingRefreshToken = errors.New("missing refresh token")
)

type Session struct {
	Client         *p.Client
	User           p.User
	UserKeyRing    *crypto.KeyRing
	Address        []p.Address
	AddressKeyRing map[string]*crypto.KeyRing
}

func SessionFromConfig(ctx context.Context, manager *p.Manager, config *SessionConfig) (*Session, error) {
	var err error

	// Initialize the client from our cahced credentials
	if config.UID == "" {
		return nil, ErrorMissingUID
	}

	if config.AccessToken == "" {
		return nil, ErrorMissingAccessToken
	}

	if config.RefreshToken == "" {
		return nil, ErrorMissingRefreshToken
	}

	var session Session

	slog.Debug("refresh client")

	slog.Debug("config", "uid", config.UID, "access_token", config.AccessToken, "refresh_token", config.RefreshToken)
	session.Client = manager.NewClient(config.UID, config.AccessToken, config.RefreshToken)

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
