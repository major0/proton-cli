package accountCmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/internal"
	"github.com/spf13/cobra"
)

// sessionStatus describes the freshness of a session.
type sessionStatus string

const (
	statusFresh   sessionStatus = "fresh"
	statusWarn    sessionStatus = "warn"
	statusExpired sessionStatus = "expired"
	statusNone    sessionStatus = "none"
)

// serviceStatus holds display data for one service session.
type serviceStatus struct {
	Service     string        `json:"service"`
	Status      sessionStatus `json:"status"`
	UID         string        `json:"uid,omitempty"`
	LastRefresh time.Time     `json:"last_refresh,omitempty"`
	Age         string        `json:"age,omitempty"`
	ExpiresIn   string        `json:"expires_in,omitempty"`
}

var statusJSON bool

var accountStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show session status for all services",
	Long:  "Display the state of all Proton service sessions (account, drive, lumo, etc.)",
	RunE:  runAccountStatus,
}

func init() {
	accountCmd.AddCommand(accountStatusCmd)
	accountStatusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
}

func runAccountStatus(_ *cobra.Command, _ []string) error {
	// Read the session index directly to enumerate all services.
	sessionFile := cli.ConfigFilePath()
	if sessionFile != "" {
		// ConfigFilePath returns the config.yaml path; session file is alongside it.
		sessionFile = sessionFile[:len(sessionFile)-len("config.yaml")] + "sessions.db"
	}

	kr := internal.SystemKeyring{}
	idx := internal.NewSessionStore(sessionFile, cli.Account, "*", kr)

	accounts, err := idx.List()
	if err != nil {
		return fmt.Errorf("reading session index: %w", err)
	}

	if len(accounts) == 0 {
		fmt.Fprintln(os.Stderr, "Not logged in.")
		os.Exit(1)
	}

	// Known services to check.
	services := []string{"*", "account", "drive", "lumo", "mail", "calendar", "pass"}

	var results []serviceStatus

	for _, svc := range services {
		store := internal.NewSessionStore(sessionFile, cli.Account, svc, kr)
		cfg, err := store.Load()
		if err != nil {
			results = append(results, serviceStatus{
				Service: svc,
				Status:  statusNone,
			})
			continue
		}

		ss := serviceStatus{
			Service:     svc,
			UID:         cfg.UID,
			LastRefresh: cfg.LastRefresh,
		}

		if cfg.LastRefresh.IsZero() {
			ss.Status = statusWarn
			ss.Age = "unknown"
			ss.ExpiresIn = "unknown"
		} else {
			age := time.Since(cfg.LastRefresh)
			ss.Age = age.Truncate(time.Second).String()

			remaining := api.TokenExpireAge - age
			switch {
			case remaining < 0:
				ss.ExpiresIn = "expired"
				ss.Status = statusExpired
			case age > api.TokenWarnAge:
				ss.ExpiresIn = remaining.Truncate(time.Second).String()
				ss.Status = statusWarn
			default:
				ss.ExpiresIn = remaining.Truncate(time.Second).String()
				ss.Status = statusFresh
			}
		}

		results = append(results, ss)
	}

	if statusJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	// Human-readable output.
	fmt.Fprintf(os.Stderr, "Account: %s\n\n", cli.Account)
	fmt.Fprintf(os.Stderr, "%-12s  %-8s  %-14s  %-14s  %s\n",
		"SERVICE", "STATUS", "AGE", "EXPIRES IN", "UID")
	for _, s := range results {
		uid := s.UID
		if uid == "" {
			uid = "-"
		} else if len(uid) > 12 {
			uid = uid[:12] + "..."
		}
		age := s.Age
		if age == "" {
			age = "-"
		}
		expires := s.ExpiresIn
		if expires == "" {
			expires = "-"
		}
		fmt.Fprintf(os.Stderr, "%-12s  %-8s  %-14s  %-14s  %s\n",
			s.Service, s.Status, age, expires, uid)
	}

	return nil
}
