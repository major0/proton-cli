package volumeCmd

import (
	"fmt"
	"os"

	"github.com/ProtonMail/go-proton-api"
	"github.com/docker/go-units"
	"github.com/jedib0t/go-pretty/v6/table"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var VolumeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List volumes",
	Long:  "List volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := cli.SessionRestore()
		if err != nil {
			return err
		}

		volumes, err := session.Client.ListVolumes(cli.Ctx)
		if err != nil {
			return err
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Volume ID", "State", "Size", "Used"})
		for _, v := range volumes {
			t.AppendRow(table.Row{
				v.VolumeID,
				getVolState(v.State),
				getVolSpace(v.MaxSpace),
				units.BytesSize(float64(v.UsedSpace)),
			})
		}
		t.Render()

		return nil
	},
}

func getVolSpace(bytes *int64) string {
	if bytes == nil {
		return "unlimited"
	}

	return units.BytesSize(float64(*bytes))
}

func getVolState(state proton.VolumeState) string {
	switch proton.VolumeState(state) {
	case proton.VolumeStateActive:
		return "Active"
	case proton.VolumeStateLocked:
		return "Locked"
	default:
		return fmt.Sprintf("Unknown (%d)", state)
	}
}

func init() {
	volumeCmd.AddCommand(VolumeListCmd)
}
