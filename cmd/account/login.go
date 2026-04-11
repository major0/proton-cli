package accountCmd

import (
	"context"

	"github.com/ProtonMail/go-proton-api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/internal"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var authLoginParams = struct {
	username string
	password string
	mboxpass string
	twoFA    string
}{}

var authLoginCmd = &cobra.Command{
	Use:   "login [options]",
	Short: "login to Proton",
	Long:  `login to Proton`,
	RunE: func(_ *cobra.Command, _ []string) error {
		username := authLoginParams.username
		password := authLoginParams.password
		mboxpass := authLoginParams.mboxpass
		twoFA := authLoginParams.twoFA
		var err error

		if username == "" {
			username, err = internal.UserPrompt("Username", false)
			if err != nil {
				return err
			}
		}

		if password == "" {
			password, err = internal.UserPrompt("Password", true)
			if err != nil {
				return err
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		session, err := common.SessionFromLogin(ctx, cli.ProtonOpts, username, password, nil)
		if err != nil {
			return err
		}

		session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
		session.AddDeauthHandler(common.NewDeauthHandler())

		if session.Auth.TwoFA.Enabled&proton.HasTOTP != 0 {
			if twoFA == "" {
				twoFA, err = internal.UserPrompt("2FA code", false)
				if err != nil {
					return err
				}
			}

			err = session.Client.Auth2FA(ctx, proton.Auth2FAReq{
				TwoFactorCode: twoFA,
			})
			if err != nil {
				return err
			}
		}

		var keypass []byte
		if session.Auth.PasswordMode == proton.TwoPasswordMode {
			if mboxpass == "" {
				mboxpass, err = internal.UserPrompt("Mailbox password", true)
				if err != nil {
					return err
				}
			}
			keypass, err = common.SaltKeyPass(ctx, session.Client, []byte(mboxpass))
		} else {
			keypass, err = common.SaltKeyPass(ctx, session.Client, []byte(password))
		}
		if err != nil {
			return err
		}

		return common.SessionSave(cli.SessionStoreVar, session, keypass)
	},
}

func init() {
	accountCmd.AddCommand(authLoginCmd)
	authLoginCmd.Flags().StringVarP(&authLoginParams.username, "username", "u", "", "Proton username")
	authLoginCmd.Flags().StringVarP(&authLoginParams.password, "password", "p", "", "Proton password")
	authLoginCmd.Flags().StringVarP(&authLoginParams.mboxpass, "mboxpass", "m", "", "Required of 2 password mode is enabled.")
	authLoginCmd.Flags().StringVarP(&authLoginParams.twoFA, "2fa", "2", "", "2FA code")
}
