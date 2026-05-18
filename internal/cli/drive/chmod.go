package driveCmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	cli "github.com/major0/proton-utils/internal/cli"
	"github.com/spf13/cobra"
)

var chmodFlags struct {
	verbose bool
}

var driveChmodCmd = &cobra.Command{
	Use:   "chmod <mode> <path>",
	Short: "Change file mode bits on Proton Drive",
	Long:  "Update Unix permission bits stored in the file's revision XAttr",
	Args:  cobra.ExactArgs(2),
	RunE:  runChmod,
}

func init() {
	driveCmd.AddCommand(driveChmodCmd)
	cli.BoolFlagP(driveChmodCmd.Flags(), &chmodFlags.verbose, "verbose", "v", false, "Print each file as its mode is changed")
}

func runChmod(cmd *cobra.Command, args []string) error {
	modeStr := args[0]
	rawPath := args[1]

	// Parse octal mode.
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return fmt.Errorf("chmod: invalid mode %q: %w", modeStr, err)
	}
	if mode > 0o7777 {
		return fmt.Errorf("chmod: mode %q exceeds maximum (7777)", modeStr)
	}

	ctx := context.Background()

	session, err := cli.SetupSession(ctx, cmd)
	if err != nil {
		return err
	}

	dc, err := cli.NewDriveClient(ctx, session)
	if err != nil {
		return err
	}

	sharePart, pathPart, err := parseProtonURI(rawPath)
	if err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if pathPart == "" {
		return fmt.Errorf("chmod: missing file path")
	}

	share, err := dc.ResolveShareComponent(ctx, sharePart)
	if err != nil {
		return fmt.Errorf("chmod: %s: %w", sharePart, err)
	}

	pathPart = strings.TrimSuffix(pathPart, "/")
	link, err := share.Link.ResolvePath(ctx, pathPart, true)
	if err != nil {
		return fmt.Errorf("chmod: %s: %w", pathPart, err)
	}

	if link.Type() != proton.LinkTypeFile {
		return fmt.Errorf("chmod: %s: not a file (directories not supported)", pathPart)
	}

	if err := dc.Chmod(ctx, share, link, uint32(mode)); err != nil {
		return err
	}

	if chmodFlags.verbose {
		name, _ := link.Name()
		fmt.Printf("mode of '%s' changed to %04o\n", name, mode)
	}

	return nil
}
