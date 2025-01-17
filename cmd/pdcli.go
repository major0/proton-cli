package pdcli

import (
	"log/slog"
	"os"
	"github.com/spf13/cobra"
)

var logLevel = new(slog.LevelVar)

var RootCmd = &cobra.Command{
	Use:   "protondrive [options] <command>",
	Short: "protondrive is a command line interface for ProtonDrive",
	Long:  `protondrive is a command line interface for managing and manipulating the ProtonDrive storage solution`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetCount("verbose")
		switch {
		case verbose == 1:
			logLevel.Set(slog.LevelInfo)
			slog.Info("verbosity", "verbose", verbose)
		case verbose >= 2:
			logLevel.Set(slog.LevelDebug)
			slog.Debug("verbosity", "verbose", verbose)
		default:
			logLevel.Set(slog.LevelWarn)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	//cobra.OnInitialize(initConfig)
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	RootCmd.PersistentFlags().CountP("verbose", "v", "Enable verbose output. Can be specified multiple times to increase verbosity.")
	
	//protonDriveCmd.AddCommand(pdcliCmd)
	
	// Hide the help command as it ends up sorted into the middle of the output
	RootCmd.CompletionOptions.HiddenDefaultCmd = true
}

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		slog.Any("error", err)
		os.Exit(1)
	}
}
