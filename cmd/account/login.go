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

// userPromptFn is the function used to prompt the user for input.
// It is a variable so tests can replace it without reading stdin.
var userPromptFn = internal.UserPrompt

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
		username, password, err := promptCredentials()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		session, err := attemptLogin(ctx, username, password)
		if err != nil {
			return err
		}

		session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
		session.AddDeauthHandler(common.NewDeauthHandler())

		if err := handleTwoFA(ctx, session); err != nil {
			return err
		}

		return deriveAndSave(ctx, session, password, authLoginParams.mboxpass)
	},
}

// promptCredentials prompts for username/password if not provided via flags.
func promptCredentials() (username, password string, err error) {
	username = authLoginParams.username
	password = authLoginParams.password

	if username == "" {
		username, err = userPromptFn("Username", false)
		if err != nil {
			return "", "", err
		}
	}

	if password == "" {
		password, err = userPromptFn("Password", true)
		if err != nil {
			return "", "", err
		}
	}

	return username, password, nil
}

// sessionFromLoginFn is the function used to create a session from login credentials.
// It is a variable so tests can replace it without making real API calls.
var sessionFromLoginFn = common.SessionFromLogin

// sessionRetryWithHVFn is the function used to retry login after HV.
// It is a variable so tests can replace it without making real API calls.
var sessionRetryWithHVFn = common.SessionRetryWithHV

// attemptLogin performs the initial login, handling HV/CAPTCHA if needed.
func attemptLogin(ctx context.Context, username, password string) (*common.Session, error) {
	session, err := sessionFromLoginFn(ctx, cli.ProtonOpts, username, password, nil, nil)
	if err != nil {
		// Check for HV error (code 9001).
		apiErr := new(proton.APIError)
		if !errors.As(err, &apiErr) || !apiErr.IsHVError() {
			return nil, err
		}

		hv, hvErr := apiErr.GetHVDetails()
		if hvErr != nil {
			return nil, fmt.Errorf("extracting HV details: %w", hvErr)
		}

		if !hasCaptchaMethod(hv.Methods) {
			return nil, fmt.Errorf("unsupported HV methods: %v", hv.Methods)
		}

		solvedToken, solveErr := SolveCaptcha(hv, authLoginParams.noBrowser)
		if solveErr != nil {
			return nil, fmt.Errorf("CAPTCHA: %w", solveErr)
		}

		hv.Token = solvedToken
		fmt.Println("Authenticating ...")

		if err := sessionRetryWithHVFn(ctx, session, username, password, hv); err != nil {
			return nil, err
		}
	}

	return session, nil
}

// handleTwoFA prompts for and submits the 2FA code if TOTP is enabled.
func handleTwoFA(ctx context.Context, session *common.Session) error {
	if session.Auth.TwoFA.Enabled&proton.HasTOTP == 0 {
		return nil
	}

	twoFA := authLoginParams.twoFA
	if twoFA == "" {
		var err error
		twoFA, err = userPromptFn("2FA code", false)
		if err != nil {
			return err
		}
	}

	return session.Client.Auth2FA(ctx, proton.Auth2FAReq{
		TwoFactorCode: twoFA,
	})
}

// deriveAndSave derives the key passphrase and saves the session.
// selectKeyPassword determines which password to use for key derivation
// based on the password mode. Returns the password bytes to salt.
func selectKeyPassword(passwordMode proton.PasswordMode, password, mboxpass string) ([]byte, error) {
	if passwordMode == proton.TwoPasswordMode {
		if mboxpass == "" {
			var err error
			mboxpass, err = userPromptFn("Mailbox password", true)
			if err != nil {
				return nil, err
			}
		}
		return []byte(mboxpass), nil
	}
	return []byte(password), nil
}

// saltKeyPassFn is the function used to salt the key password.
// It is a variable so tests can replace it without making real API calls.
var saltKeyPassFn = func(ctx context.Context, session *common.Session, password []byte) ([]byte, error) {
	return common.SaltKeyPass(ctx, session.Client, password)
}

// sessionSaveFn is the function used to save the session.
// It is a variable so tests can replace it without real persistence.
var sessionSaveFn = func(session *common.Session, keypass []byte) error {
	return common.SessionSave(cli.SessionStoreVar, session, keypass)
}

func deriveAndSave(ctx context.Context, session *common.Session, password, mboxpass string) error {
	passBytes, err := selectKeyPassword(session.Auth.PasswordMode, password, mboxpass)
	if err != nil {
		return err
	}

	keypass, err := saltKeyPassFn(ctx, session, passBytes)
	if err != nil {
		return err
	}

	return sessionSaveFn(session, keypass)
}

func init() {
	accountCmd.AddCommand(authLoginCmd)
	authLoginCmd.Flags().StringVarP(&authLoginParams.username, "username", "u", "", "Proton username")
	authLoginCmd.Flags().StringVarP(&authLoginParams.password, "password", "p", "", "Proton password")
	authLoginCmd.Flags().StringVarP(&authLoginParams.mboxpass, "mboxpass", "m", "", "Required of 2 password mode is enabled.")
	authLoginCmd.Flags().StringVarP(&authLoginParams.twoFA, "2fa", "2", "", "2FA code")
	cli.BoolFlag(authLoginCmd.Flags(), &authLoginParams.noBrowser, "no-browser", false, "Do not open browser for CAPTCHA; print URL and prompt for token")
}
