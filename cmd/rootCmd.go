package cli

import (
	"log/slog"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	common "github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/internal"
	"github.com/spf13/cobra"
)

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

	// Private variables below this point

	logLevel = new(slog.LevelVar)

	// rootCmd parameter store. Only the results of Flags and our preRun
	// flag cleanups should be stored here.
	rootParams = struct {
		Account     string
		ConfigFile  string
		MaxWorkers  int
		SessionFile string
		Verbose     int
		Timeout     time.Duration
	}{}

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

// ManagerHook returns the debug hook callback when DebugHTTP is enabled,
// or nil otherwise. Pass the result to SessionFromLogin, SessionRestore, etc.
func ManagerHook() func(*proton.Manager) {
	// Temporarily disabled — WithDebug(true) provides resty-level logging.
	// Our custom hooks may conflict with resty's debug mode.
	return nil
}

// Execute runs the root command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// cobra.OnInitialize(initConfig) // TODO
	logopts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, logopts))
	slog.SetDefault(logger)
	// proton.WithLogger(common.Logger)

	ProtonOpts = []proton.Option{
		proton.WithHostURL(APIHost),
		proton.WithAppVersion(AppVersion),
		proton.WithUserAgent(UserAgent),
	}

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
