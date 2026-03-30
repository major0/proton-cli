package driveCmd

import (
	"context"
	"fmt"
	"path/filepath"

	cli "github.com/major0/proton-cli/cmd"
	proton "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var driveUploadParams = struct {
	filePath   string
	remoteName string
	shareID    string
}{
	shareID: "primary",
}

var driveUploadCmd = &cobra.Command{
	Use:   "upload [options]",
	Short: "Upload a file to Proton Drive",
	Long:  "Upload a file to Proton Drive (currently to the root of the selected share)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if driveUploadParams.filePath == "" {
			return fmt.Errorf("--file is required")
		}

		session, err := cli.SessionRestore()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()

		remoteName := driveUploadParams.remoteName
		if remoteName == "" {
			remoteName = filepath.Base(driveUploadParams.filePath)
		}

		var upload *proton.UploadResult
		if driveUploadParams.shareID == "" || driveUploadParams.shareID == "primary" {
			upload, err = session.UploadFileToPrimaryShareRoot(ctx, driveUploadParams.filePath, remoteName)
		} else {
			upload, err = session.UploadFileToShareRoot(ctx, driveUploadParams.shareID, driveUploadParams.filePath, remoteName)
		}
		if err != nil {
			return err
		}

		fmt.Printf("uploaded: %s\n", upload.FileName)
		fmt.Printf("size: %d bytes\n", upload.Size)
		fmt.Printf("share_id: %s\n", upload.ShareID)
		fmt.Printf("link_id: %s\n", upload.LinkID)
		fmt.Printf("revision_id: %s\n", upload.RevisionID)
		return nil
	},
}

func init() {
	driveCmd.AddCommand(driveUploadCmd)
	driveUploadCmd.Flags().StringVarP(&driveUploadParams.filePath, "file", "f", "", "Local file path to upload")
	driveUploadCmd.Flags().StringVarP(&driveUploadParams.remoteName, "name", "n", "", "Remote file name (defaults to basename of --file)")
	driveUploadCmd.Flags().StringVar(&driveUploadParams.shareID, "share-id", "primary", "Share ID to upload into, or 'primary'")
}
