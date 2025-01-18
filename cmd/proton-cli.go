package cli

import (
	"context"
	"encoding/base64"
	"log/slog"
	"os"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	cacheFileName  = "protondrive/cache.yaml"
	configFileName = "protondrive/config.yaml"
	Version        = "0.0.1"
	AppVersion     = "Other"
	UserAgent      = "proton-cli" + "/" + Version + " (ProtonDrive CLI v" + Version + ")"
)

type SavedConfig struct {
	UID          string `yaml:"uid"`
	Username     string `yaml:"username"`
	AccessToken  string `yaml:"access_token"`
	RefreshToken string `yaml:"refresh_token"`
	KeyPass      string `yaml:"keypass"`
}

var (
	Ctx            = context.Background()
	Salts          proton.Salts
	Client         *proton.Client
	Manager        *proton.Manager
	Config         SavedConfig
	KeyPass        []byte
	UserKeyRing    *crypto.KeyRing
	AddressKeyRing map[string]*crypto.KeyRing
	logLevel       = new(slog.LevelVar)
	RootCmd        = &cobra.Command{
		Use:   "protondrive [options] <command>",
		Short: "protondrive is a command line interface for ProtonDrive",
		Long:  `protondrive is a command line interface for managing and manipulating the ProtonDrive storage solution`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

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

			opts := []proton.Option{
				proton.WithAppVersion(AppVersion),
				proton.WithUserAgent(UserAgent),
			}
			Manager = proton.New(opts...)

			configFilePath, _ := xdg.SearchCacheFile(cacheFileName)
			if configFilePath != "" {
				slog.Debug("config", "path", configFilePath)
				configFile, err := os.ReadFile(configFilePath)
				if err != nil {
					return err
				}

				err = yaml.Unmarshal(configFile, &Config)
				if err != nil {
					return err
				}
			}
			KeyPass, err = base64.StdEncoding.DecodeString(Config.KeyPass)
			if err != nil {
				return err
			}

			// Initialize the client from our cahced credentials
			slog.Debug("refresh client")
			if Config.UID != "" && Config.AccessToken != "" && Config.RefreshToken != "" {
				slog.Debug("config", "uid", Config.UID, "access_token", Config.AccessToken, "refresh_token", Config.RefreshToken)
				Client = Manager.NewClient(Config.UID, Config.AccessToken, Config.RefreshToken)
				Client.AddAuthHandler(AuthHandler)
				Client.AddDeauthHandler(DeauthHandler)

				slog.Debug("GetUser")
				user, err := Client.GetUser(Ctx)
				if err != nil {
					return err
				}

				slog.Debug("GetAddresses")
				addrs, err := Client.GetAddresses(Ctx)
				if err != nil {
					return err
				}

				slog.Debug("Unlock")
				UserKeyRing, AddressKeyRing, err = proton.Unlock(user, addrs, KeyPass, nil)
				if err != nil {
					return err
				}
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
)

func init() {
	//cobra.OnInitialize(initConfig)
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	RootCmd.PersistentFlags().CountP("verbose", "v", "Enable verbose output. Can be specified multiple times to increase verbosity.")

	// Hide the help flags as it ends up sorted into everything, which is a bit confusing.
	RootCmd.CompletionOptions.HiddenDefaultCmd = true
	RootCmd.SilenceUsage = true
	RootCmd.PersistentFlags().BoolP("help", "h", false, "Help for proton-cli")
	RootCmd.PersistentFlags().Lookup("help").Hidden = true
}

func SaveConfig() error {
	yamlFile, err := yaml.Marshal(&Config)
	if err != nil {
		return err
	}

	configFilePath, err := xdg.CacheFile(cacheFileName)
	if err != nil {
		return err
	}

	f, err := os.Create(configFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(yamlFile)
	if err != nil {
		return err
	}

	return nil
}

func PurgeConfig() error {
	configFilePath, _ := xdg.SearchCacheFile(cacheFileName)

	if configFilePath != "" {
		return os.Remove(configFilePath)
	}

	return nil
}

func AuthHandler(auth proton.Auth) {
	// Save the login credentials into our app cache
	slog.Debug("auth", "uid", auth.UID, "access_token", auth.AccessToken, "refresh_token", auth.RefreshToken)
	Config.UID = auth.UID
	Config.AccessToken = auth.AccessToken
	Config.RefreshToken = auth.RefreshToken
	_ = SaveConfig()
}

func DeauthHandler() {
	// Currently do nothing
	slog.Debug("deauth")
}

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		slog.Any("error", err)
		os.Exit(1)
	}
}
