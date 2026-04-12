package volumeCmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/docker/go-units"
	"github.com/jedib0t/go-pretty/v6/table"
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var volumeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List volumes",
	Long:    "List volumes with share type, usage, and traffic stats",
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

	// Build a share type index keyed by ShareID for the volume join.
	shares, err := session.Client.ListShares(ctx, true)
	if err != nil {
		return err
	}

	shareIndex := make(map[string]proton.ShareMetadata, len(shares))
	for _, s := range shares {
		shareIndex[s.ShareID] = s
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"Volume ID", "State", "Type", "Size", "Used",
		"Down", "Up", "Created", "Restore",
	})

	for _, v := range volumes {
		shareType := "?"
		if s, ok := shareIndex[v.Share.ShareID]; ok {
			shareType = fmtShareType(s.Type)
		}

		t.AppendRow(table.Row{
			v.VolumeID,
			fmtVolState(v.State),
			shareType,
			fmtSpace(v.MaxSpace),
			units.BytesSize(float64(v.UsedSpace)),
			units.BytesSize(float64(v.DownloadedBytes)),
			units.BytesSize(float64(v.UploadedBytes)),
			fmtTime(v.CreationTime),
			fmtRestore(v.RestoreStatus),
		})
	}

	t.Render()
	return nil
}

func fmtSpace(bytes *int64) string {
	if bytes == nil {
		return "unlimited"
	}
	return units.BytesSize(float64(*bytes))
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
	default:
		return fmt.Sprintf("unknown(%d)", st)
	}
}

func fmtTime(epoch int64) string {
	if epoch == 0 {
		return "-"
	}
	return time.Unix(epoch, 0).Format("2006-01-02")
}

func fmtRestore(status *proton.VolumeRestoreStatus) string {
	if status == nil {
		return "-"
	}
	switch *status {
	case proton.RestoreStatusDone:
		return "done"
	case proton.RestoreStatusInProgress:
		return "in-progress"
	case proton.RestoreStatusFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", *status)
	}
}
