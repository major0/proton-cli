package proton

import (
	"context"

	"github.com/ProtonMail/go-proton-api"
)

// SaltKeyPass salts the provided keypass using the user's primary key salt.
// The salted keypass is used to Unlock the account.
func SaltKeyPass(ctx context.Context, client *proton.Client, password []byte) ([]byte, error) {
	user, err := client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	salts, err := client.GetSalts(ctx)
	if err != nil {
		return nil, err
	}

	saltedKeypass, err := salts.SaltForKey(password, user.Keys.Primary().ID)
	if err != nil {
		return nil, err
	}

	return saltedKeypass, nil
}
