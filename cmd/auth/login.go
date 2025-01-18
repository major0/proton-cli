package authCmd

import (
	"context"
	"encoding/base64"
	"log/slog"

	"github.com/ProtonMail/go-proton-api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/internal"
	"github.com/spf13/cobra"
)

var authLoginCmd = &cobra.Command{
	Use:   "login [options]",
	Short: "login to ProtonDrive",
	Long:  `login to ProtonDrive`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		ctx := context.Background()

		username, _ := cmd.Flags().GetString("username")
		if username == "" {
			username, err = internal.UserPrompt("Username", false)
			if err != nil {
				return err
			}
		}

		password, _ := cmd.Flags().GetString("password")
		if password == "" {
			password, err = internal.UserPrompt("Password", true)
			if err != nil {
				return err
			}
		}

		slog.Debug("login", "username", username, "password", password)
		client, auth, err := cli.Manager.NewClientWithLogin(ctx, username, []byte(password))
		if err != nil {
			return err
		}
		cli.Client = client
		cli.Client.AddAuthHandler(cli.AuthHandler)
		cli.Client.AddDeauthHandler(cli.DeauthHandler)

		if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
			twoFA, _ := cmd.Flags().GetString("2fa")
			if twoFA == "" {
				twoFA, err = internal.UserPrompt("2FA code", false)
				if err != nil {
					return err
				}
			}

			err = client.Auth2FA(ctx, proton.Auth2FAReq{
				TwoFactorCode: twoFA,
			})
			if err != nil {
				return err
			}
		}
		cli.Config.UID = auth.UID
		cli.Config.Username = username
		cli.Config.AccessToken = auth.AccessToken
		cli.Config.RefreshToken = auth.RefreshToken

		var keypass []byte
		if auth.PasswordMode == proton.TwoPasswordMode {
			mboxPasswd, _ := cmd.Flags().GetString("mailbox-password")
			if mboxPasswd == "" {
				mboxPasswd, err = internal.UserPrompt("Mailbox password", true)
				if err != nil {
					return err
				}
			}
			keypass, err = getKeypass(ctx, []byte(mboxPasswd))
		} else {
			keypass, err = getKeypass(ctx, []byte(password))
		}
		if err != nil {
			return err
		}

		cli.Config.KeyPass = base64.StdEncoding.EncodeToString(keypass)
		if err := cli.SaveConfig(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authLoginCmd.Flags().StringP("username", "u", "", "ProtonDrive username")
	authLoginCmd.Flags().StringP("password", "p", "", "ProtonDrive password")
	authLoginCmd.Flags().StringP("mailbox-password", "m", "", "Required of 2 password mode is enabled.")
	authLoginCmd.Flags().StringP("2fa", "a", "", "2FA code")
}

func getKeypass(ctx context.Context, password []byte) ([]byte, error) {
	user, err := cli.Client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	salts, err := cli.Client.GetSalts(ctx)
	if err != nil {
		return nil, err
	}

	saltedKeypass, err := salts.SaltForKey(password, user.Keys.Primary().ID)
	if err != nil {
		return nil, err
	}

	return saltedKeypass, nil
}
