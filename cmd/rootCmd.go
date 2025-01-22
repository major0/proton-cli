package cli

import (
	"log/slog"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/internal"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var (
	Timeout time.Duration

	// Private variables below this point

	logLevel = new(slog.LevelVar)

	// Handles loading/saving session data
	sessionStore common.SessionStore

	protonOptions = []proton.Option{
		proton.WithAppVersion(AppVersion),
		proton.WithUserAgent(UserAgent),
	}

	// rootCmd parameter store. Only the results of Flags and our preRun
	// flag cleanups should be stored here.
	rootParams = struct {
		Account     string
		ConfigFile  string
		SessionFile string
		Verbose     int
		Timeout     time.Duration
	}{}

	rootCmd = &cobra.Command{
		Use:   "protondrive [options] <command>",
		Short: "protondrive is a command line interface for ProtonDrive",
		Long:  `protondrive is a command line interface for managing and manipulating the ProtonDrive storage solution`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
				rootParams.ConfigFile = "proton/config.yaml"
			}

			if rootParams.SessionFile == "" {
				rootParams.SessionFile = "proton/sessions.db"
			}

			Timeout = rootParams.Timeout * time.Second

			sessionStore = internal.NewFileStore(rootParams.SessionFile, rootParams.Account)

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
)

// A AddCommand() wrapper to make it a little easier for subcmds
func AddCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}

// Needed for the main() entrypoint
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		slog.Any("error", err)
		os.Exit(1)
	}
}

func init() {
	//cobra.OnInitialize(initConfig) // TODO
	logopts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, logopts))
	slog.SetDefault(logger)

	protonOptions = []proton.Option{
		proton.WithAppVersion(AppVersion),
		proton.WithUserAgent(UserAgent),
	}

	rootCmd.PersistentFlags().CountVarP(&rootParams.Verbose, "verbose", "v", "Enable verbose output. Can be specified multiple times to increase verbosity.")
	rootCmd.PersistentFlags().StringVarP(&rootParams.Account, "account", "a", "default", "Nickname of the account to use. This can be any string the user desires.")
	rootCmd.PersistentFlags().StringVar(&rootParams.ConfigFile, "config-file", "", "Config file to use. Defaults to value XDG_CONFIG_FILE")
	rootCmd.PersistentFlags().StringVar(&rootParams.SessionFile, "session-file", "", "Session file to use. Defaults to value XDG_CACHE_FILE")
	rootCmd.PersistentFlags().DurationVarP(&rootParams.Timeout, "timeout", "t", 60*time.Second, "Timeout for requests.")

	// Hide the help flags as it ends up sorted into everything, which is a bit confusing.
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Help for proton-cli")
	rootCmd.PersistentFlags().Lookup("help").Hidden = true
}
