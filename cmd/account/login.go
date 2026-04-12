package accountCmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/internal"
	"github.com/spf13/cobra"
)

var authLoginParams = struct {
	username  string
	password  string
	mboxpass  string
	twoFA     string
	noBrowser bool
}{}

// hasCaptchaMethod reports whether "captcha" is among the HV methods.
func hasCaptchaMethod(methods []string) bool {
	for _, m := range methods {
		if m == "captcha" {
			return true
		}
	}
	return false
}

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

		// Build manager hook for debug logging at verbosity >= 3.
		managerHook := cli.ManagerHook()

		session, err := common.SessionFromLogin(ctx, cli.ProtonOpts, username, password, nil, managerHook)
		if err != nil {
			// Check for HV error (code 9001).
			apiErr := new(proton.APIError)
			if !errors.As(err, &apiErr) || !apiErr.IsHVError() {
				return err
			}

			hv, hvErr := apiErr.GetHVDetails()
			if hvErr != nil {
				return fmt.Errorf("extracting HV details: %w", hvErr)
			}

			if !hasCaptchaMethod(hv.Methods) {
				return fmt.Errorf("unsupported HV methods: %v", hv.Methods)
			}

			// Solve the CAPTCHA and get the composite token.
			// The verify.proton.me page returns a composite token via
			// postMessage that must be sent in x-pm-human-verification-token.
			solvedToken, solveErr := SolveCaptcha(hv, authLoginParams.noBrowser)
			if solveErr != nil {
				return fmt.Errorf("CAPTCHA: %w", solveErr)
			}

			// Replace the token with the composite solved token.
			hv.Token = solvedToken

			fmt.Println("Authenticating ...")

			// Retry on the same session (same manager + cookie jar).
			if err := common.SessionRetryWithHV(ctx, session, username, password, hv); err != nil {
				return err
			}
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
	authLoginCmd.Flags().BoolVar(&authLoginParams.noBrowser, "no-browser", false, "Do not open browser for CAPTCHA; print URL and prompt for token")
}
