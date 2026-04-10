package driveCmd

import (
	"context"
	"fmt"

	"log/slog"
	"strings"
	"time"

	"github.com/ProtonMail/go-proton-api"
	//"github.com/jedib0t/go-pretty/v6/table"
	cli "github.com/major0/proton-cli/cmd"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
)

var driveListCmd = &cobra.Command{
	Use:   "list [options] [<path> ...]",
	Short: "List drive information",
	Long:  "List Proton Drive information",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse/Validate the path.
		var path string

		// The path is in the form of proton://<share>/<path>/...
		// FIXME handle multiple paths?
		if len(args) > 0 {
			if !strings.HasPrefix(args[0], "proton://") {
				return fmt.Errorf("invalid path: %s", args[0])
			}
			path = strings.TrimPrefix(args[0], "proton://")
			path = strings.TrimPrefix(path, "/")
			trailingSlash := ""
			if strings.HasSuffix(path, "/") {
				trailingSlash = "/"
			}

			oldParts := strings.Split(path, "/")
			newParts := make([]string, 0)
			for _, part := range oldParts {
				switch part {
				case ".":
					continue
				case "":
					continue
				case "..":
					if len(newParts) > 0 {
						newParts = newParts[:len(newParts)-1]
					}
				default:
					newParts = append(newParts, part)
				}
			}
			path = strings.Join(newParts, "/") + trailingSlash
		}

		session, err := cli.SessionRestore()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
		defer cancel()


		// If no path is specified then simply list all shares.
		// If a path is specified then reolve it to the last link.
		var links []common.Link
		if len(args) == 0 || path == "" {
			shares, err := session.ListShares(ctx, true)
			if err != nil {
				return err
			}

			for _, share := range shares {
				links = append(links, *share.Link)
			}
		} else {
			link, err := session.ResolvePath(ctx, path, true)
			if err != nil {
				return err
			}

			// If the path ends in a `/` then list the contents of the folder.
			if strings.HasSuffix(path, "/") {
				if link.Type != proton.LinkTypeFolder {
					return fmt.Errorf("not a folder: %s", path)
				}

				slog.Debug("cmd.ListChildren", "path", path)
				links, err = link.ListChildren(ctx, true)
				slog.Debug("cmd.ListChildren", "links", len(links))
				if err != nil {
					return err
				}
			} else {
				links = append(links, *link)
			}
		}

		// FIXME pass to a function that handles different formatting options
		for _, link := range links {
			if proton.LinkState(*link.State) == proton.LinkStateDeleted {
				// FIXME add a flag for selecting this?
				continue
			}

			name := link.Name

			ctime := time.Unix(link.CreateTime, 0)
			//xtime := time.Unix(link.ExpirationTime, 0)

			var linkType byte
			if link.Type == proton.LinkTypeFolder {
				linkType = 'd'
			} else {

				linkType = '-'
			}

			fmt.Printf("%crwxr-xr-x %-20s %s\n", linkType, name, ctime)
		}

		return nil
	},
}

var driveListParam struct {
	all       bool
	almostAll bool
}

func init() {
	driveCmd.AddCommand(driveListCmd)
	driveListCmd.Flags().BoolVarP(&driveListParam.all, "all", "a", false, "Do not ignore entries starting with '.'")
	driveListCmd.Flags().BoolVarP(&driveListParam.almostAll, "almost-all", "A", false, "Do not list implied '.' and '..' entries")
}
