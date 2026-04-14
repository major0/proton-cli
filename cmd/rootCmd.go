package cli

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	common "github.com/major0/proton-cli/api"
	driveClient "github.com/major0/proton-cli/api/drive/client"
	"github.com/major0/proton-cli/internal"
	"github.com/spf13/cobra"
)

// rootParamsType holds the parsed root command flags.
type rootParamsType struct {
	Account     string
	ConfigFile  string
	MaxWorkers  int
	SessionFile string
	Verbose     int
	Timeout     time.Duration
}

var (
	// Timeout holds the global request timeout duration.
	Timeout time.Duration

	// DebugHTTP is true when verbosity >= 3, enabling HTTP debug logging.
	DebugHTTP bool

	// ProtonOpts holds the base Proton API options (app version, user agent).
	ProtonOpts []proton.Option

	// SessionStoreVar handles loading/saving session data.
	SessionStoreVar common.SessionStore

	// Account holds the current --account flag value.
	Account string

	// ConfigVar holds the loaded application config. Available to all subcommands.
	ConfigVar *common.Config

	// Private variables below this point

	logLevel = new(slog.LevelVar)

	// rootCmd parameter store. Only the results of Flags and our preRun
	// flag cleanups should be stored here.
	rootParams rootParamsType

	rootCmd = &cobra.Command{
		Use:   "proton [options] <command>",
		Short: "proton is a command line interface for Proton services",
		Long:  `proton is a command line interface for managing Proton services (Drive, Mail, etc.)`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			switch {
			case rootParams.Verbose == 1:
				logLevel.Set(slog.LevelInfo)
				slog.Info("verbosity", "verbose", rootParams.Verbose)
			case rootParams.Verbose > 1:
				logLevel.Set(slog.LevelDebug)
				slog.Debug("verbosity", "verbose", rootParams.Verbose)
			default:
				logLevel.Set(slog.LevelWarn)
			}

			if rootParams.ConfigFile == "" {
				rootParams.ConfigFile = xdgConfigPath("config.yaml")
			}

			if rootParams.SessionFile == "" {
				rootParams.SessionFile = xdgConfigPath("sessions.db")
			}

			Timeout = rootParams.Timeout
			DebugHTTP = rootParams.Verbose >= 3
			Account = rootParams.Account

			// Rebuild proton options based on verbosity.
			ProtonOpts = []proton.Option{
				proton.WithHostURL(APIHost),
				proton.WithAppVersion(AppVersion),
				proton.WithUserAgent(UserAgent),
			}

			if DebugHTTP {
				ProtonOpts = append(ProtonOpts, proton.WithDebug(true))
			}

			SessionStoreVar = internal.NewSessionStore(rootParams.SessionFile, rootParams.Account, "*", internal.SystemKeyring{})

			// Load application config.
			cfg, err := common.LoadConfig(rootParams.ConfigFile)
			if err != nil {
				slog.Warn("config load failed, using defaults", "error", err)
				cfg = common.DefaultConfig()
			}
			ConfigVar = cfg

			return nil
		},
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}
)

// AddCommand registers a subcommand with the root command.
func AddCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}

// RestoreSession returns a fully initialized, ready-to-use session using
// the package-level ProtonOpts and SessionStoreVar.
func RestoreSession(ctx context.Context) (*common.Session, error) {
	session, err := common.ReadySession(ctx, ProtonOpts, SessionStoreVar, nil)
	if err != nil {
		return nil, err
	}
	session.AppVersion = AppVersion
	session.UserAgent = UserAgent
	return session, nil
}

// NewDriveClient creates a drive client with the loaded config applied.
func NewDriveClient(ctx context.Context, session *common.Session) (*driveClient.Client, error) {
	dc, err := driveClient.NewClient(ctx, session)
	if err != nil {
		return nil, err
	}
	dc.Config = ConfigVar
	return dc, nil
}

// Execute runs the root command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ConfigFilePath returns the resolved config file path.
func ConfigFilePath() string {
	return rootParams.ConfigFile
}

func init() {
	// cobra.OnInitialize(initConfig) // TODO
	logopts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, logopts))
	slog.SetDefault(logger)
	// proton.WithLogger(common.Logger)

	rootCmd.PersistentFlags().CountVarP(&rootParams.Verbose, "verbose", "v", "Enable verbose output. Can be specified multiple times to increase verbosity.")
	rootCmd.PersistentFlags().StringVarP(&rootParams.Account, "account", "a", "default", "Nickname of the account to use. This can be any string the user desires.")
	rootCmd.PersistentFlags().StringVar(&rootParams.ConfigFile, "config-file", "", "Config file to use. Defaults to value XDG_CONFIG_FILE")
	rootCmd.PersistentFlags().StringVar(&rootParams.SessionFile, "session-file", "", "Session file to use. Defaults to value XDG_CACHE_FILE")
	rootCmd.PersistentFlags().DurationVarP(&rootParams.Timeout, "timeout", "t", 60*time.Second, "Timeout for requests.")
	rootCmd.PersistentFlags().IntVarP(&rootParams.MaxWorkers, "max-jobs", "j", 10, "Maximum number of jobs to run in parallel.")

	// Hide the help flags as it ends up sorted into everything, which is a bit confusing.
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Help for proton-cli")
	rootCmd.PersistentFlags().Lookup("help").Hidden = true
}
