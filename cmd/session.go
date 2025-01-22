package cli

import (
	"context"
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/internal"
	common "github.com/major0/proton-cli/proton"
)

func SessionRestore() (*common.Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rootParams.Timeout)
	defer cancel()

	// Initialize a new session via the session store
	sessionConfig, err := sessionStore.Load()
	if err != nil {
		if err == common.ErrKeyNotFound {
			return nil, ErrNotLoggedIn
		}
		return nil, err
	}

	slog.Debug("SessionRestore config", "uid", sessionConfig.UID, "access_token", sessionConfig.AccessToken, "refresh_token", sessionConfig.RefreshToken, "keypass", sessionConfig.SaltedKeyPass)

	sessionCreds := common.SessionCredentials{}
	err = internal.CopySubStruct(sessionConfig, &sessionCreds)
	if err != nil {
		return nil, err
	}

	slog.Debug("SessionRestore creds", "uid", sessionCreds.UID, "access_token", sessionCreds.AccessToken, "refresh_token", sessionCreds.RefreshToken)

	session, err := common.SessionFromCredentials(ctx, protonOptions, &sessionCreds)
	if err != nil {
		return nil, err
	}
	session.AddAuthHandler(authHandler)
	session.AddDeauthHandler(deauthHandler)

	keypass, err := common.Base64Decode(sessionConfig.SaltedKeyPass)
	if err != nil {
		return nil, err
	}

	err = session.Unlock(string(keypass))
	if err != nil {
		return nil, err
	}

	return session, nil
}

func SessionLogin(username string, password string, mboxpass string, twoFA string) (*common.Session, error) {
	var err error

	if username == "" {
		username, err = internal.UserPrompt("Username", false)
		if err != nil {
			return nil, err
		}
	}

	if password == "" {
		password, err = internal.UserPrompt("Password", true)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootParams.Timeout)
	defer cancel()

	slog.Debug("login", "username", username, "password", "<hidden>", "mboxpasswd", "<hidden>", "2fa", twoFA)
	session, err := common.SessionFromLogin(ctx, protonOptions, username, password)
	if err != nil {
		return nil, err
	}
	session.AddAuthHandler(authHandler)
	session.AddDeauthHandler(deauthHandler)

	if session.Auth.TwoFA.Enabled&proton.HasTOTP != 0 {
		if twoFA == "" {
			twoFA, err = internal.UserPrompt("2FA code", false)
			if err != nil {
				return nil, err
			}
		}

		err = session.Client.Auth2FA(ctx, proton.Auth2FAReq{
			TwoFactorCode: twoFA,
		})
		if err != nil {
			return nil, err
		}
	}

	var keypass []byte
	if session.Auth.PasswordMode == proton.TwoPasswordMode {
		if mboxpass == "" {
			mboxpass, err = internal.UserPrompt("Mailbox password", true)
			if err != nil {
				return nil, err
			}
		}
		keypass, err = common.SaltKeyPass(ctx, session.Client, []byte(mboxpass))
	} else {
		keypass, err = common.SaltKeyPass(ctx, session.Client, []byte(password))
	}
	if err != nil {
		return nil, err
	}

	config := &common.SessionConfig{
		UID:           session.Auth.UID,
		AccessToken:   session.Auth.AccessToken,
		RefreshToken:  session.Auth.RefreshToken,
		SaltedKeyPass: common.Base64Encode(keypass),
	}

	sessionStore := internal.NewFileStore(rootParams.SessionFile, rootParams.Account)

	if err := sessionStore.Save(config); err != nil {
		return session, err
	}

	return session, err
}

func SessionRevoke(session *common.Session, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootParams.Timeout)
	defer cancel()

	err := session.Client.AuthRevoke(ctx, session.Auth.UID)
	if err != nil && !force {
		return err
	}

	sessionStore := internal.NewFileStore(rootParams.SessionFile, rootParams.Account)
	return sessionStore.Delete()
}

func SessionList() ([]string, error) {
	return sessionStore.List()
}
