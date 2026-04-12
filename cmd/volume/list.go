package volumeCmd

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/docker/go-units"
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var volumeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List volumes",
	Long:    "List volumes with usage stats",
	RunE:    runVolumeList,
}

func init() {
	volumeCmd.AddCommand(volumeListCmd)
}

func runVolumeList(_ *cobra.Command, _ []string) error {
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

	// Build share index for type lookup.
	shares, err := session.Client.ListShares(ctx, true)
	if err != nil {
		return err
	}

	shareIndex := make(map[string]proton.ShareMetadata, len(shares))
	for _, s := range shares {
		shareIndex[s.ShareID] = s
	}

	// Resolve root link names by getting the share + decrypting the root link.
	nameIndex := make(map[string]string) // volumeID → decrypted root name
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

	// Print df-style output.
	fmt.Printf("%-20s %10s %10s %10s %5s %10s %10s %s\n",
		"Volume", "Size", "Used", "Avail", "Use%", "Down", "Up", "State")

	for _, v := range volumes {
		label := nameIndex[v.VolumeID]
		if label == "" {
			if s, ok := shareIndex[v.Share.ShareID]; ok {
				label = fmtShareType(s.Type)
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
			fmtVolState(v.State),
		)
	}

	return nil
}

func fmtVolState(state proton.VolumeState) string {
	switch state {
	case proton.VolumeStateActive:
		return "active"
	case proton.VolumeStateLocked:
		return "locked"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

func fmtShareType(st proton.ShareType) string {
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
