package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/internal"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

const (
	cacheFileName  = "protondrive/cache.yaml"
	configFileName = "protondrive/config.yaml"
	Version        = "0.0.1"
	AppVersion     = "Other"
	UserAgent      = "proton-cli" + "/" + Version + " (Proton CLI v" + Version + ")"
)

var (
	ErrNoAccount   = fmt.Errorf("no account specified")
	ErrNoConfig    = fmt.Errorf("no config file specified")
	ErrNoSession   = fmt.Errorf("no session file specified")
	ErrNoTimeout   = fmt.Errorf("no timeout specified")
	ErrNotLoggedIn = fmt.Errorf("not logged in")
)

// Global Variables
var (
	Ctx       = context.Background()
	AccountID = "default"
)

// Local variables
var (
	cancel        context.CancelFunc
	manager       *proton.Manager
	sessionConfig *common.SessionConfig
	sessionStore  *internal.SessionStore
	logLevel      = new(slog.LevelVar)

	rootParams = struct {
		account     string
		configFile  string
		sessionFile string
		verbose     int
		timeout     time.Duration
	}{}

	rootCmd = &cobra.Command{
		Use:   "protondrive [options] <command>",
		Short: "protondrive is a command line interface for ProtonDrive",
		Long:  `protondrive is a command line interface for managing and manipulating the ProtonDrive storage solution`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case rootParams.verbose == 1:
				logLevel.Set(slog.LevelInfo)
				slog.Info("verbosity", "verbose", rootParams.verbose)
			case rootParams.verbose > 1:
				logLevel.Set(slog.LevelDebug)
				slog.Debug("verbosity", "verbose", rootParams.verbose)
			default:
				logLevel.Set(slog.LevelWarn)
			}

			if rootParams.configFile == "" {
				rootParams.configFile = "proton/config.yaml"
			}

			if rootParams.sessionFile == "" {
				rootParams.sessionFile = "proton/sessions.db"
			}

			rootParams.timeout = rootParams.timeout * time.Second

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
)

func AddCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}

/* The authHandler is periodically called by the underyling Proton Client
 * whenever an authentication refresh has been performed. Whenever this
 * happens we need to update our in-memory session config as well as
 * update our session cache.
 *
 * FIXME should this be protected by a mutex? */
func authHandler(auth proton.Auth) {
	// Save the login credentials into our app cache
	slog.Debug("auth", "uid", auth.UID, "access_token", auth.AccessToken, "refresh_token", auth.RefreshToken)
	sessionConfig.UID = auth.UID
	sessionConfig.AccessToken = auth.AccessToken
	sessionConfig.RefreshToken = auth.RefreshToken
	_ = sessionStore.Save(sessionConfig)
}

/* Similar to the authHandler, the deauthHandler is called by the Proton
 * Client. It is not entirely clear what we should be doing here? Possibly
 * we should purge the current session cache. For now we only log that
 * a deauth call was made. */
func deauthHandler() {
	// Currently do nothing
	slog.Debug("deauth")
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		slog.Any("error", err)
		os.Exit(1)
	}
}

func GetKeyPass() ([]byte, error) {
	if sessionConfig.SaltedKeyPass == "" {
		return nil, nil
	}

	keypass, err := common.Base64Decode(sessionConfig.SaltedKeyPass)
	if err != nil {
		return nil, err
	}

	return keypass, nil
}

func GetUID() string {
	return sessionConfig.UID
}

func init() {
	//cobra.OnInitialize(initConfig) // TODO
	logopts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, logopts))
	slog.SetDefault(logger)

	popts := []proton.Option{
		proton.WithAppVersion(AppVersion),
		proton.WithUserAgent(UserAgent),
	}
	manager = proton.New(popts...)

	rootCmd.PersistentFlags().CountVarP(&rootParams.verbose, "verbose", "v", "Enable verbose output. Can be specified multiple times to increase verbosity.")
	rootCmd.PersistentFlags().StringVarP(&rootParams.account, "account", "a", "default", "Nickname of the account to use. This can be any string the user desires.")
	rootCmd.PersistentFlags().StringVar(&rootParams.configFile, "config-file", "", "Config file to use. Defaults to value XDG_CONFIG_FILE")
	rootCmd.PersistentFlags().StringVar(&rootParams.sessionFile, "session-file", "", "Session file to use. Defaults to value XDG_CACHE_FILE")
	rootCmd.PersistentFlags().DurationVarP(&rootParams.timeout, "timeout", "t", 60, "Timeout for requests. Defaults to 60 seconds.")

	// Hide the help flags as it ends up sorted into everything, which is a bit confusing.
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Help for proton-cli")
	rootCmd.PersistentFlags().Lookup("help").Hidden = true
}

func SessionRestore() (*common.Session, error) {
	var err error

	Ctx, cancel = context.WithTimeout(context.Background(), rootParams.timeout)

	// Initialize a new session via the session store
	fileCache := internal.NewFileCache(rootParams.sessionFile)
	sessionStore = internal.NewSessionStore(fileCache, rootParams.account)
	sessionConfig, err = sessionStore.Load()
	if err != nil {
		if err == internal.ErrKeyNotFound {
			return nil, ErrNotLoggedIn
		}
		return nil, err
	}

	session, err := common.SessionFromConfig(Ctx, manager, sessionConfig)
	if err != nil {
		return nil, err
	}
	session.Client.AddAuthHandler(authHandler)
	session.Client.AddDeauthHandler(deauthHandler)

	keypass, err := GetKeyPass()
	if err != nil {
		return nil, err
	}
	slog.Debug("Unlock")
	session.UserKeyRing, session.AddressKeyRing, err = proton.Unlock(session.User, session.Address, keypass, nil)
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

	Ctx, cancel = context.WithTimeout(context.Background(), rootParams.timeout)

	session := &common.Session{}

	slog.Debug("login", "username", username, "password", "<hidden>", "mboxpasswd", "<hidden>", "2fa", twoFA)
	client, auth, err := manager.NewClientWithLogin(Ctx, username, []byte(password))
	if err != nil {
		return nil, err
	}
	session.Client = client
	session.Client.AddAuthHandler(authHandler)
	session.Client.AddDeauthHandler(deauthHandler)

	if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
		if twoFA == "" {
			twoFA, err = internal.UserPrompt("2FA code", false)
			if err != nil {
				return nil, err
			}
		}

		err = session.Client.Auth2FA(Ctx, proton.Auth2FAReq{
			TwoFactorCode: twoFA,
		})
		if err != nil {
			return nil, err
		}
	}
	sessionConfig := &common.SessionConfig{}
	sessionConfig.UID = auth.UID
	sessionConfig.AccessToken = auth.AccessToken
	sessionConfig.RefreshToken = auth.RefreshToken

	var keypass []byte
	if auth.PasswordMode == proton.TwoPasswordMode {
		if mboxpass == "" {
			mboxpass, err = internal.UserPrompt("Mailbox password", true)
			if err != nil {
				return nil, err
			}
		}
		keypass, err = common.SaltKeyPass(Ctx, session.Client, []byte(mboxpass))
	} else {
		keypass, err = common.SaltKeyPass(Ctx, session.Client, []byte(password))
	}
	if err != nil {
		return nil, err
	}

	sessionConfig.SaltedKeyPass = common.Base64Encode(keypass)
	fileCache := internal.NewFileCache(rootParams.sessionFile)
	sessionStore = internal.NewSessionStore(fileCache, rootParams.account)

	if err := sessionStore.Save(sessionConfig); err != nil {
		return session, err
	}

	return session, err
}

func SessionRevoke(session *common.Session, force bool) error {
	err := session.Client.AuthRevoke(Ctx, sessionConfig.UID)
	if err != nil && !force {
		return err
	}

	fileCache := internal.NewFileCache(rootParams.sessionFile)
	sessionStore = internal.NewSessionStore(fileCache, rootParams.account)
	return sessionStore.Delete()
}

func SessionStop(sesion *common.Session) {
	manager.Close()
	cancel()
}
