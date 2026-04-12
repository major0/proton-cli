package driveCmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ProtonMail/go-proton-api"
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
)

var findFlags struct {
	name     string
	iname    string
	findType string
	minSize  int64
	maxSize  int64
	mtime    int
	newer    string
	maxDepth int
	print0   bool
}

var driveFindCmd = &cobra.Command{
	Use:   "find [options] [<path>]",
	Short: "Search for files and directories in Proton Drive",
	Long:  "Search for files and directories in Proton Drive, compatible with Unix find",
	RunE:  runFind,
}

func init() {
	driveCmd.AddCommand(driveFindCmd)
	f := driveFindCmd.Flags()
	f.SetLongOnly(true)
	f.StringVar(&findFlags.name, "name", "", "Match file name (glob pattern, case-sensitive)")
	f.StringVar(&findFlags.iname, "iname", "", "Match file name (glob pattern, case-insensitive)")
	f.StringVar(&findFlags.findType, "type", "", "File type: f (file), d (directory)")
	f.Int64Var(&findFlags.minSize, "minsize", 0, "Minimum file size in bytes")
	f.Int64Var(&findFlags.maxSize, "maxsize", 0, "Maximum file size in bytes")
	f.IntVar(&findFlags.mtime, "mtime", 0, "Modified time in days (negative=within N days, positive=older than N days)")
	f.StringVar(&findFlags.newer, "newer", "", "Match files newer than this ISO date (YYYY-MM-DD)")
	f.IntVar(&findFlags.maxDepth, "maxdepth", -1, "Maximum directory depth (-1 for unlimited)")
	cli.BoolFlag(f, &findFlags.print0, "print0", false, "Separate output with NUL instead of newline")
}

type findPredicate func(p string, l *common.Link, depth int) bool

func buildPredicates() []findPredicate {
	var preds []findPredicate

	if findFlags.findType != "" {
		preds = append(preds, func(_ string, l *common.Link, _ int) bool {
			switch findFlags.findType {
			case "f":
				return l.Type() == proton.LinkTypeFile
			case "d":
				return l.Type() == proton.LinkTypeFolder
			default:
				return true
			}
		})
	}

	if findFlags.name != "" {
		preds = append(preds, func(_ string, l *common.Link, _ int) bool {
			name, err := l.Name()
			if err != nil {
				return false
			}
			matched, _ := path.Match(findFlags.name, name)
			return matched
		})
	}

	if findFlags.iname != "" {
		pattern := strings.ToLower(findFlags.iname)
		preds = append(preds, func(_ string, l *common.Link, _ int) bool {
			name, err := l.Name()
			if err != nil {
				return false
			}
			matched, _ := path.Match(pattern, strings.ToLower(name))
			return matched
		})
	}

	if findFlags.minSize > 0 {
		preds = append(preds, func(_ string, l *common.Link, _ int) bool {
			return l.Size() >= findFlags.minSize
		})
	}

	if findFlags.maxSize > 0 {
		preds = append(preds, func(_ string, l *common.Link, _ int) bool {
			return l.Size() <= findFlags.maxSize
		})
	}

	if findFlags.mtime != 0 {
		preds = append(preds, func(_ string, l *common.Link, _ int) bool {
			mt := time.Unix(l.ModifyTime(), 0)
			days := time.Since(mt).Hours() / 24
			if findFlags.mtime < 0 {
				return days <= float64(-findFlags.mtime)
			}
			return days >= float64(findFlags.mtime)
		})
	}

	if findFlags.newer != "" {
		t, err := time.Parse("2006-01-02", findFlags.newer)
		if err == nil {
			preds = append(preds, func(_ string, l *common.Link, _ int) bool {
				return time.Unix(l.ModifyTime(), 0).After(t)
			})
		}
	}

	if findFlags.maxDepth >= 0 {
		preds = append(preds, func(_ string, _ *common.Link, depth int) bool {
			return depth <= findFlags.maxDepth
		})
	}

	return preds
}

func matchAll(preds []findPredicate, p string, l *common.Link, depth int) bool {
	for _, pred := range preds {
		if !pred(p, l, depth) {
			return false
		}
	}
	return true
}

func runFind(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
	if err != nil {
		return err
	}

	session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
	session.AddDeauthHandler(common.NewDeauthHandler())

	// Default to all shares if no path given.
	var roots []*common.Link
	var rootPaths []string

	if len(args) == 0 {
		shares, err := session.ListShares(ctx, true)
		if err != nil {
			return err
		}
		for i := range shares {
			name, _ := shares[i].GetName(ctx)
			roots = append(roots, shares[i].Link)
			rootPaths = append(rootPaths, name)
		}
	} else {
		for _, arg := range args {
			link, _, err := resolveProtonPath(ctx, session, arg)
			if err != nil {
				return fmt.Errorf("find: %s: %w", arg, err)
			}
			name, _ := link.Name()
			roots = append(roots, link)
			rootPaths = append(rootPaths, name)
		}
	}

	preds := buildPredicates()
	sep := "\n"
	if findFlags.print0 {
		sep = "\x00"
	}

	for i, root := range roots {
		if err := findWalk(ctx, rootPaths[i], root, 0, preds, sep); err != nil {
			return err
		}
	}

	return nil
}

func findWalk(ctx context.Context, prefix string, l *common.Link, depth int, preds []findPredicate, sep string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Check maxdepth for traversal (not just matching).
	if findFlags.maxDepth >= 0 && depth > findFlags.maxDepth {
		return nil
	}

	if matchAll(preds, prefix, l, depth) {
		fmt.Print(prefix + sep)
	}

	if l.Type() != proton.LinkTypeFolder {
		return nil
	}

	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			fmt.Fprintf(os.Stderr, "find: %s: %v\n", prefix, entry.Err)
			continue
		}

		childName, err := entry.Link.Name()
		if err != nil {
			continue
		}

		childPath := prefix + "/" + childName
		if err := findWalk(ctx, childPath, entry.Link, depth+1, preds, sep); err != nil {
			return err
		}
	}

	return nil
}
