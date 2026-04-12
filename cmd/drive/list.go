package driveCmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/docker/go-units"
	cli "github.com/major0/proton-cli/cmd"
	common "github.com/major0/proton-cli/proton"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// outputFormat controls how entries are displayed.
type outputFormat int

const (
	formatColumns outputFormat = iota // -C: multi-column (default if tty)
	formatLong                        // -l: long listing
	formatSingle                      // -1: one entry per line (default if pipe)
	formatAcross                      // -x: multi-column, sorted across
)

// sortMode controls how entries are ordered.
type sortMode int

const (
	sortName sortMode = iota // default
	sortSize                 // -S / --sort=size
	sortTime                 // -t / --sort=time
	sortNone                 // -U / --sort=none
)

// timeStyle controls time formatting in long mode.
type timeStyle int

const (
	timeDefault timeStyle = iota // ls-style: "Jan  2 15:04" or "Jan  2  2006"
	timeFull                     // --full-time: ISO 8601
	timeISO                      // --time-style=iso
	timeLongISO                  // --time-style=long-iso
)

// listOpts holds all resolved options after flag parsing.
type listOpts struct {
	format    outputFormat
	sortBy    sortMode
	timeStyle timeStyle
	human     bool
	all       bool
	almostAll bool
	recursive bool
	reverse   bool
}

// Raw flag values — resolved into listOpts in runList.
var listFlags struct {
	all       bool
	almostAll bool
	long      bool
	single    bool
	across    bool
	columns   bool
	human     bool
	recursive bool
	reverse   bool
	sortSize  bool
	sortTime  bool
	unsorted  bool
	fullTime  bool
	format    string
	sortWord  string
	timeStyle string
}

var driveListCmd = &cobra.Command{
	Use:     "list [options] [<path> ...]",
	Aliases: []string{"ls"},
	Short:   "List files and directories in Proton Drive",
	Long:    "List files and directories in Proton Drive",
	RunE:    runList,
}

func init() {
	driveCmd.AddCommand(driveListCmd)
	f := driveListCmd.Flags()

	// Visibility
	f.BoolVarP(&listFlags.all, "all", "a", false, "Do not ignore entries starting with '.'")
	f.BoolVarP(&listFlags.almostAll, "almost-all", "A", false, "Do not list implied '.' and '..'")

	// Output format (last one wins)
	f.BoolVarP(&listFlags.long, "long", "l", false, "Use long listing format")
	f.BoolVarP(&listFlags.single, "single-column", "1", false, "List one file per line")
	f.BoolVarP(&listFlags.across, "across", "x", false, "List entries by lines instead of columns")
	f.BoolVarP(&listFlags.columns, "columns", "C", false, "List entries in columns")
	f.StringVar(&listFlags.format, "format", "", "Output format: long, single-column, across, columns")

	// Sizes
	f.BoolVar(&listFlags.human, "human-readable", false, "Print sizes in human-readable format")

	// Sorting
	f.BoolVarP(&listFlags.sortSize, "sort-size", "S", false, "Sort by file size, largest first")
	f.BoolVarP(&listFlags.sortTime, "sort-time", "t", false, "Sort by modification time, newest first")
	f.BoolVarP(&listFlags.unsorted, "unsorted", "U", false, "Do not sort; list in directory order")
	f.BoolVarP(&listFlags.reverse, "reverse", "r", false, "Reverse sort order")
	f.StringVar(&listFlags.sortWord, "sort", "", "Sort by: name, size, time, none")

	// Time formatting
	f.BoolVar(&listFlags.fullTime, "full-time", false, "Like -l --time-style=full-iso")
	f.StringVar(&listFlags.timeStyle, "time-style", "", "Time format: full-iso, long-iso, iso")

	// Recursion
	f.BoolVarP(&listFlags.recursive, "recursive", "R", false, "List subdirectories recursively")
}

// resolveOpts converts raw flag values into a clean listOpts struct.
// Last-flag-wins semantics for format and sort, matching GNU ls.
func resolveOpts() (listOpts, error) {
	opts := listOpts{
		all:       listFlags.all,
		almostAll: listFlags.almostAll,
		human:     listFlags.human,
		recursive: listFlags.recursive,
		reverse:   listFlags.reverse,
	}

	// Default format: columns if tty, single-column if pipe.
	if term.IsTerminal(int(os.Stdout.Fd())) { //nolint:gosec // fd fits int on all platforms
		opts.format = formatColumns
	} else {
		opts.format = formatSingle
	}

	// Short flags override in order (last wins).
	if listFlags.columns {
		opts.format = formatColumns
	}
	if listFlags.single {
		opts.format = formatSingle
	}
	if listFlags.across {
		opts.format = formatAcross
	}
	if listFlags.long {
		opts.format = formatLong
	}

	// --format=WORD overrides short flags.
	switch listFlags.format {
	case "":
		// no override
	case "long", "verbose":
		opts.format = formatLong
	case "single-column":
		opts.format = formatSingle
	case "across", "horizontal":
		opts.format = formatAcross
	case "columns", "vertical":
		opts.format = formatColumns
	default:
		return opts, fmt.Errorf("invalid --format value: %q", listFlags.format)
	}

	// Sort mode.
	opts.sortBy = sortName
	if listFlags.sortSize {
		opts.sortBy = sortSize
	}
	if listFlags.sortTime {
		opts.sortBy = sortTime
	}
	if listFlags.unsorted {
		opts.sortBy = sortNone
	}

	switch listFlags.sortWord {
	case "":
		// no override
	case "name":
		opts.sortBy = sortName
	case "size":
		opts.sortBy = sortSize
	case "time":
		opts.sortBy = sortTime
	case "none":
		opts.sortBy = sortNone
	default:
		return opts, fmt.Errorf("invalid --sort value: %q", listFlags.sortWord)
	}

	// Time style.
	opts.timeStyle = timeDefault
	switch listFlags.timeStyle {
	case "":
		// no override
	case "full-iso":
		opts.timeStyle = timeFull
	case "long-iso":
		opts.timeStyle = timeLongISO
	case "iso":
		opts.timeStyle = timeISO
	default:
		return opts, fmt.Errorf("invalid --time-style value: %q", listFlags.timeStyle)
	}

	// --full-time implies -l with full-iso timestamps.
	if listFlags.fullTime {
		opts.format = formatLong
		opts.timeStyle = timeFull
	}

	return opts, nil
}

// parsePath normalizes a proton:// path, resolving . and .. components.
func parsePath(raw string) string {
	path := strings.TrimPrefix(raw, "proton://")
	path = strings.TrimPrefix(path, "/")

	trailingSlash := ""
	if strings.HasSuffix(path, "/") {
		trailingSlash = "/"
	}

	var parts []string
	for _, p := range strings.Split(path, "/") {
		switch p {
		case "", ".":
			continue
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, p)
		}
	}

	return strings.Join(parts, "/") + trailingSlash
}

// resolveLinks resolves the given path arguments to a list of links.
func resolveLinks(ctx context.Context, session *common.Session, args []string) ([]common.Link, error) {
	if len(args) == 0 {
		return rootLinks(ctx, session)
	}

	var links []common.Link
	for _, arg := range args {
		if !strings.HasPrefix(arg, "proton://") {
			return nil, fmt.Errorf("invalid path: %s (must start with proton://)", arg)
		}

		path := parsePath(arg)
		if path == "" {
			roots, err := rootLinks(ctx, session)
			if err != nil {
				return nil, err
			}
			links = append(links, roots...)
			continue
		}

		link, err := session.ResolvePath(ctx, path, true)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arg, err)
		}

		if strings.HasSuffix(path, "/") {
			if link.Type != proton.LinkTypeFolder {
				return nil, fmt.Errorf("%s: not a directory", arg)
			}
			children, err := link.ListChildren(ctx, true)
			if err != nil {
				return nil, err
			}
			links = append(links, children...)
		} else {
			links = append(links, *link)
		}
	}

	return links, nil
}

// rootLinks returns the root links from all shares.
func rootLinks(ctx context.Context, session *common.Session) ([]common.Link, error) {
	shares, err := session.ListShares(ctx, true)
	if err != nil {
		return nil, err
	}

	links := make([]common.Link, len(shares))
	for i, share := range shares {
		links[i] = *share.Link
	}
	return links, nil
}

// filterLinks removes hidden files unless -a or -A is set, and skips deleted links.
func filterLinks(links []common.Link, opts listOpts) []common.Link {
	var out []common.Link
	for _, l := range links {
		if l.State != nil && *l.State == proton.LinkStateDeleted {
			continue
		}
		if !opts.all && !opts.almostAll && strings.HasPrefix(l.Name, ".") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// doSort sorts links according to the active sort mode.
func doSort(links []common.Link, opts listOpts) {
	if opts.sortBy == sortNone {
		if opts.reverse {
			for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
				links[i], links[j] = links[j], links[i]
			}
		}
		return
	}

	sort.SliceStable(links, func(i, j int) bool {
		var less bool
		switch opts.sortBy {
		case sortSize:
			less = links[i].Size > links[j].Size
		case sortTime:
			less = links[i].ModifyTime > links[j].ModifyTime
		default:
			less = strings.ToLower(links[i].Name) < strings.ToLower(links[j].Name)
		}
		if opts.reverse {
			return !less
		}
		return less
	})
}

// formatSize returns the size as a string, optionally human-readable.
func formatSize(size int64, opts listOpts) string {
	if opts.human {
		return units.HumanSize(float64(size))
	}
	return fmt.Sprintf("%d", size)
}

// formatTimestamp formats an epoch timestamp according to the time style.
func formatTimestamp(epoch int64, style timeStyle) string {
	t := time.Unix(epoch, 0)
	switch style {
	case timeFull:
		return t.Format("2006-01-02 15:04:05.000000000 -0700")
	case timeLongISO:
		return t.Format("2006-01-02 15:04")
	case timeISO:
		return t.Format("01-02 15:04")
	default:
		sixMonthsAgo := time.Now().AddDate(0, -6, 0)
		if t.Before(sixMonthsAgo) {
			return t.Format("Jan _2  2006")
		}
		return t.Format("Jan _2 15:04")
	}
}

// typeChar returns 'd' for folders, '-' for files.
func typeChar(lt proton.LinkType) byte {
	if lt == proton.LinkTypeFolder {
		return 'd'
	}
	return '-'
}

// printLong prints a single link in long format.
func printLong(l common.Link, opts listOpts) {
	fmt.Printf("%c%-9s %8s %s %s\n",
		typeChar(l.Type),
		"rwxr-xr-x",
		formatSize(l.Size, opts),
		formatTimestamp(l.ModifyTime, opts.timeStyle),
		l.Name,
	)
}

// printLinks prints the link list in the selected format.
func printLinks(links []common.Link, opts listOpts) {
	switch opts.format {
	case formatLong:
		for _, l := range links {
			printLong(l, opts)
		}

	case formatSingle:
		for _, l := range links {
			fmt.Println(l.Name)
		}

	case formatColumns:
		printColumns(links, false)

	case formatAcross:
		printColumns(links, true)
	}
}

// printColumns prints names in multi-column format. If across is true,
// entries fill rows left-to-right (-x); otherwise they fill columns
// top-to-bottom (-C).
func printColumns(links []common.Link, across bool) {
	if len(links) == 0 {
		return
	}

	names := make([]string, len(links))
	maxLen := 0
	for i, l := range links {
		names[i] = l.Name
		if len(l.Name) > maxLen {
			maxLen = len(l.Name)
		}
	}

	// Column width: longest name + 2 spaces padding.
	colWidth := maxLen + 2

	// Terminal width, default 80.
	termWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 { //nolint:gosec // fd fits int
		termWidth = w
	}

	numCols := termWidth / colWidth
	if numCols < 1 {
		numCols = 1
	}

	numRows := (len(names) + numCols - 1) / numCols

	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			var idx int
			if across {
				idx = row*numCols + col
			} else {
				idx = col*numRows + row
			}

			if idx >= len(names) {
				continue
			}

			if col < numCols-1 {
				fmt.Printf("%-*s", colWidth, names[idx])
			} else {
				fmt.Print(names[idx])
			}
		}
		fmt.Println()
	}
}

// listRecursive prints a directory tree recursively.
func listRecursive(ctx context.Context, prefix string, links []common.Link, opts listOpts) error {
	for _, l := range links {
		if l.Type != proton.LinkTypeFolder {
			continue
		}

		path := prefix + l.Name + "/"
		children, err := l.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		children = filterLinks(children, opts)
		doSort(children, opts)

		fmt.Printf("\n%s:\n", prefix+l.Name)
		printLinks(children, opts)

		if err := listRecursive(ctx, path, children, opts); err != nil {
			return err
		}
	}
	return nil
}

func runList(_ *cobra.Command, args []string) error {
	opts, err := resolveOpts()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cli.Timeout)
	defer cancel()

	session, err := common.SessionRestore(ctx, cli.ProtonOpts, cli.SessionStoreVar, cli.ManagerHook())
	if err != nil {
		return err
	}

	session.AddAuthHandler(common.NewAuthHandler(cli.SessionStoreVar, session))
	session.AddDeauthHandler(common.NewDeauthHandler())

	slog.Debug("drive.list", "args", args)

	links, err := resolveLinks(ctx, session, args)
	if err != nil {
		return err
	}

	links = filterLinks(links, opts)
	doSort(links, opts)
	printLinks(links, opts)

	if opts.recursive {
		if err := listRecursive(ctx, "", links, opts); err != nil {
			return err
		}
	}

	return nil
}
