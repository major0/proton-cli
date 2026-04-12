package accountCmd

import (
	"context"
	"fmt"

	"github.com/docker/go-units"
	"github.com/major0/proton-cli/api/account"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var accountInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "report account information",
	Long:  `report information about currently logged in user`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		session, err := cli.RestoreSession(ctx)
		if err != nil {
			return err
		}

		acct := account.NewClient(session)
		user, err := acct.GetUser(ctx)
		if err != nil {
			return err
		}

		fmt.Println("ID: " + user.ID)
		fmt.Println("Display Name: " + user.DisplayName)
		fmt.Println("Username: " + user.Name)
		fmt.Println("Email: " + user.Email)
		fmt.Println("")

		total := units.BytesSize(float64(user.MaxSpace))
		used := units.BytesSize(float64(user.UsedSpace))
		avail := "-"
		pct := "-"
		if user.MaxSpace > 0 {
			free := user.MaxSpace - user.UsedSpace
			avail = units.BytesSize(float64(free))
			pct = fmt.Sprintf("%.1f%%", float64(user.UsedSpace)/float64(user.MaxSpace)*100)
		}

		fmt.Printf("Storage: %s / %s (%s free, %s used)\n", used, total, avail, pct)
		fmt.Println("")
		fmt.Printf("%-12s %10s\n", "Service", "Used")
		fmt.Printf("%-12s %10s\n", "Mail", units.BytesSize(float64(user.ProductUsedSpace.Mail)))
		fmt.Printf("%-12s %10s\n", "Drive", units.BytesSize(float64(user.ProductUsedSpace.Drive)))
		fmt.Printf("%-12s %10s\n", "Calendar", units.BytesSize(float64(user.ProductUsedSpace.Calendar)))
		fmt.Printf("%-12s %10s\n", "Pass", units.BytesSize(float64(user.ProductUsedSpace.Pass)))
		fmt.Printf("%-12s %10s\n", "Contacts", units.BytesSize(float64(user.ProductUsedSpace.Contact)))

		return nil
	},
}

func init() {
	accountCmd.AddCommand(accountInfoCmd)
}
