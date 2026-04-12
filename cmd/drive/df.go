package driveCmd

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/docker/go-units"
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var driveDfCmd = &cobra.Command{
	Use:   "df",
	Short: "Show volume disk usage",
	Long:  "Show Proton Drive volume usage in df-style output",
	RunE:  runDf,
}

func init() {
	driveCmd.AddCommand(driveDfCmd)
}

func runDf(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
	if err != nil {
		return err
	}

	session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
	session.AddDeauthHandler(common.NewDeauthHandler())

	volumes, err := session.Client.ListVolumes(ctx)
	if err != nil {
		return err
	}

	shares, err := session.Client.ListShares(ctx, true)
	if err != nil {
		return err
	}

	shareIndex := make(map[string]proton.ShareMetadata, len(shares))
	for _, s := range shares {
		shareIndex[s.ShareID] = s
	}

	nameIndex := make(map[string]string)
	for _, v := range volumes {
		share, err := session.GetShare(ctx, v.Share.ShareID)
		if err != nil {
			continue
		}
		name, err := share.GetName(ctx)
		if err != nil {
			continue
		}
		nameIndex[v.VolumeID] = name
	}

	fmt.Printf("%-20s %10s %10s %10s %5s %10s %10s %s\n",
		"Volume", "Size", "Used", "Avail", "Use%", "Down", "Up", "State")

	for _, v := range volumes {
		label := nameIndex[v.VolumeID]
		if label == "" {
			if s, ok := shareIndex[v.Share.ShareID]; ok {
				label = dfShareType(s.Type)
			} else {
				label = v.VolumeID[:12] + "..."
			}
		}

		used := v.UsedSpace
		size := "unlimited"
		avail := "-"
		usePct := "-"

		if v.MaxSpace != nil {
			total := *v.MaxSpace
			size = units.BytesSize(float64(total))
			free := total - used
			if free < 0 {
				free = 0
			}
			avail = units.BytesSize(float64(free))
			if total > 0 {
				usePct = fmt.Sprintf("%.0f%%", float64(used)/float64(total)*100)
			}
		}

		fmt.Printf("%-20s %10s %10s %10s %5s %10s %10s %s\n",
			label,
			size,
			units.BytesSize(float64(used)),
			avail,
			usePct,
			units.BytesSize(float64(v.DownloadedBytes)),
			units.BytesSize(float64(v.UploadedBytes)),
			dfVolState(v.State),
		)
	}

	return nil
}

func dfVolState(state proton.VolumeState) string {
	switch state {
	case proton.VolumeStateActive:
		return "active"
	case proton.VolumeStateLocked:
		return "locked"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

func dfShareType(st proton.ShareType) string {
	switch st {
	case proton.ShareTypeMain:
		return "main"
	case proton.ShareTypeStandard:
		return "shared"
	case proton.ShareTypeDevice:
		return "device"
	case 4:
		return "photos"
	default:
		return fmt.Sprintf("unknown(%d)", st)
	}
}
