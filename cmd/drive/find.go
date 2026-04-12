package driveCmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	driveClient "github.com/major0/proton-cli/api/drive/client"
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
	print    bool // -print is default behavior, explicit flag for compatibility
	depth    bool // -depth: process directory contents before the directory itself
	trashed  bool // include trashed items
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
	cli.BoolFlag(f, &findFlags.print, "print", false, "Print matching paths (default action)")
	cli.BoolFlag(f, &findFlags.depth, "depth", false, "Process directory contents before the directory itself")
	cli.BoolFlag(f, &findFlags.trashed, "trashed", false, "Include trashed items in results")
}

type findPredicate func(p string, l *drive.Link, depth int) bool

func buildPredicates() []findPredicate {
	var preds []findPredicate

	if findFlags.findType != "" {
		preds = append(preds, func(_ string, l *drive.Link, _ int) bool {
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
		preds = append(preds, func(_ string, l *drive.Link, _ int) bool {
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
		preds = append(preds, func(_ string, l *drive.Link, _ int) bool {
			name, err := l.Name()
			if err != nil {
				return false
			}
			matched, _ := path.Match(pattern, strings.ToLower(name))
			return matched
		})
	}

	if findFlags.minSize > 0 {
		preds = append(preds, func(_ string, l *drive.Link, _ int) bool {
			return l.Size() >= findFlags.minSize
		})
	}

	if findFlags.maxSize > 0 {
		preds = append(preds, func(_ string, l *drive.Link, _ int) bool {
			return l.Size() <= findFlags.maxSize
		})
	}

	if findFlags.mtime != 0 {
		preds = append(preds, func(_ string, l *drive.Link, _ int) bool {
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
			preds = append(preds, func(_ string, l *drive.Link, _ int) bool {
				return time.Unix(l.ModifyTime(), 0).After(t)
			})
		}
	}

	return preds
}

func matchAll(preds []findPredicate, p string, l *drive.Link, depth int) bool {
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

	session, err := cli.RestoreSession(ctx)
	if err != nil {
		return err
	}

	dc, err := driveClient.NewClient(ctx, session)
	if err != nil {
		return err
	}

	// Default to all shares if no path given, or if proton:// alone.
	var roots []*drive.Link
	var rootPaths []string

	// Normalize: no args or bare "proton://" both mean "all shares".
	searchAll := len(args) == 0
	if len(args) == 1 {
		p := parsePath(args[0])
		if p == "" {
			searchAll = true
		}
	}

	if searchAll {
		shares, err := dc.ListShares(ctx, true)
		if err != nil {
			return err
		}
		for i := range shares {
			name, _ := shares[i].GetName(ctx)
			roots = append(roots, shares[i].Link)
			rootPaths = append(rootPaths, "proton://"+name+"/")
		}
	} else {
		for _, arg := range args {
			link, _, err := resolveProtonPath(ctx, dc, arg)
			if err != nil {
				return fmt.Errorf("find: %s: %w", arg, err)
			}
			p := strings.TrimSuffix(arg, "/")
			if link.Type() == proton.LinkTypeFolder {
				p += "/"
			}
			roots = append(roots, link)
			rootPaths = append(rootPaths, p)
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

func findWalk(ctx context.Context, prefix string, l *drive.Link, depth int, preds []findPredicate, sep string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if findFlags.maxDepth >= 0 && depth > findFlags.maxDepth {
		return nil
	}

	// Skip trashed/deleted items unless explicitly searching for them.
	state := l.State()
	if state == proton.LinkStateDeleted {
		return nil
	}
	if state == proton.LinkStateTrashed && !findFlags.trashed {
		return nil
	}

	// Breadth-first (default): print before descending.
	if !findFlags.depth {
		if matchAll(preds, prefix, l, depth) {
			fmt.Print(prefix + sep)
		}
	}

	if l.Type() == proton.LinkTypeFolder {
		if findFlags.maxDepth < 0 || depth < findFlags.maxDepth {
			if err := findChildren(ctx, prefix, l, depth, preds, sep); err != nil {
				return err
			}
		}
	}

	// Depth-first: print after descending.
	if findFlags.depth {
		if matchAll(preds, prefix, l, depth) {
			fmt.Print(prefix + sep)
		}
	}

	return nil
}

// findChildren collects children, prints matches at this level, then
// descends into child directories. This ensures breadth-first ordering:
// all entries at depth N are printed before any at depth N+1.
func findChildren(ctx context.Context, prefix string, l *drive.Link, depth int, preds []findPredicate, sep string) error {
	type childWork struct {
		path string
		link *drive.Link
	}

	// Collect all children from Readdir.
	var children []childWork
	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			fmt.Fprintf(os.Stderr, "find: %s: %v\n", prefix, entry.Err)
			continue
		}

		childName, err := entry.Link.Name()
		if err != nil {
			continue
		}

		// Skip trashed/deleted unless searching for them.
		state := entry.Link.State()
		if state == proton.LinkStateDeleted {
			continue
		}
		if state == proton.LinkStateTrashed && !findFlags.trashed {
			continue
		}

		childPath := prefix + childName
		if entry.Link.Type() == proton.LinkTypeFolder {
			childPath += "/"
		}
		children = append(children, childWork{path: childPath, link: entry.Link})
	}

	if len(children) == 0 {
		return nil
	}

	// Phase 1: print all matches at this level.
	if !findFlags.depth {
		for _, c := range children {
			if matchAll(preds, c.path, c.link, depth+1) {
				fmt.Print(c.path + sep)
			}
		}
	}

	// Phase 2: descend into child directories concurrently.
	var dirs []childWork
	for _, c := range children {
		if c.link.Type() == proton.LinkTypeFolder {
			if findFlags.maxDepth < 0 || depth+1 < findFlags.maxDepth {
				dirs = append(dirs, c)
			}
		}
	}

	if len(dirs) > 0 {
		errCh := make(chan error, len(dirs))
		sem := make(chan struct{}, 3)

		var wg sync.WaitGroup
		for _, d := range dirs {
			wg.Add(1)
			go func(c childWork) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				if err := findDescend(ctx, c.path, c.link, depth+1, preds, sep); err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}(d)
		}

		wg.Wait()
		close(errCh)

		if err, ok := <-errCh; ok {
			return err
		}
	}

	// Phase 3 (depth-first only): print matches after descending.
	if findFlags.depth {
		for _, c := range children {
			if matchAll(preds, c.path, c.link, depth+1) {
				fmt.Print(c.path + sep)
			}
		}
	}

	return nil
}

// findDescend is the recursive entry point for child directories.
func findDescend(ctx context.Context, prefix string, l *drive.Link, depth int, preds []findPredicate, sep string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if findFlags.maxDepth >= 0 && depth > findFlags.maxDepth {
		return nil
	}

	return findChildren(ctx, prefix, l, depth, preds, sep)
}
