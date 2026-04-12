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
	common "github.com/major0/proton-cli/api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// outputFormat controls how entries are displayed.
type outputFormat int

const (
	formatColumns outputFormat = iota
	formatLong
	formatSingle
	formatAcross
)

// sortMode controls how entries are ordered.
type sortMode int

const (
	sortName sortMode = iota
	sortSize
	sortTime
	sortNone
)

// timeStyle controls time formatting in long mode.
type timeStyle int

const (
	timeDefault timeStyle = iota
	timeFull
	timeISO
	timeLongISO
)

type listOpts struct {
	format    outputFormat
	sortBy    sortMode
	timeStyle timeStyle
	human     bool
	all       bool
	almostAll bool
	recursive bool
	reverse   bool
	color     bool
}

var listFlags struct {
	all, almostAll, long, single, across, columns bool
	human, recursive, reverse                     bool
	sortSize, sortTime, unsorted                  bool
	fullTime                                      bool
	format, sortWord, timeStyle, color            string
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
	f.BoolVarP(&listFlags.all, "all", "a", false, "Do not ignore entries starting with '.'")
	f.BoolVarP(&listFlags.almostAll, "almost-all", "A", false, "Do not list implied '.' and '..'")
	f.BoolVarP(&listFlags.long, "long", "l", false, "Use long listing format")
	f.BoolVarP(&listFlags.single, "single-column", "1", false, "List one file per line")
	f.BoolVarP(&listFlags.across, "across", "x", false, "List entries by lines instead of columns")
	f.BoolVarP(&listFlags.columns, "columns", "C", false, "List entries in columns")
	f.StringVar(&listFlags.format, "format", "", "Output format: long, single-column, across, columns")
	f.BoolVar(&listFlags.human, "human-readable", false, "Print sizes in human-readable format")
	f.BoolVarP(&listFlags.sortSize, "sort-size", "S", false, "Sort by file size, largest first")
	f.BoolVarP(&listFlags.sortTime, "sort-time", "t", false, "Sort by modification time, newest first")
	f.BoolVarP(&listFlags.unsorted, "unsorted", "U", false, "Do not sort; list in directory order")
	f.BoolVarP(&listFlags.reverse, "reverse", "r", false, "Reverse sort order")
	f.StringVar(&listFlags.sortWord, "sort", "", "Sort by: name, size, time, none")
	f.BoolVar(&listFlags.fullTime, "full-time", false, "Like -l --time-style=full-iso")
	f.StringVar(&listFlags.timeStyle, "time-style", "", "Time format: full-iso, long-iso, iso")
	f.BoolVarP(&listFlags.recursive, "recursive", "R", false, "List subdirectories recursively")
	f.StringVar(&listFlags.color, "color", "auto", "Colorize output: auto, always, never")
}

func resolveOpts() (listOpts, error) {
	opts := listOpts{
		all: listFlags.all, almostAll: listFlags.almostAll,
		human: listFlags.human, recursive: listFlags.recursive,
		reverse: listFlags.reverse,
	}

	if term.IsTerminal(int(os.Stdout.Fd())) { //nolint:gosec
		opts.format = formatColumns
	} else {
		opts.format = formatSingle
	}

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

	switch listFlags.format {
	case "":
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

	opts.timeStyle = timeDefault
	switch listFlags.timeStyle {
	case "":
	case "full-iso":
		opts.timeStyle = timeFull
	case "long-iso":
		opts.timeStyle = timeLongISO
	case "iso":
		opts.timeStyle = timeISO
	default:
		return opts, fmt.Errorf("invalid --time-style value: %q", listFlags.timeStyle)
	}

	if listFlags.fullTime {
		opts.format = formatLong
		opts.timeStyle = timeFull
	}

	switch listFlags.color {
	case "always":
		opts.color = true
	case "never":
		opts.color = false
	case "auto", "":
		opts.color = term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec
	default:
		return opts, fmt.Errorf("invalid --color value: %q (use auto, always, or never)", listFlags.color)
	}

	return opts, nil
}

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

func resolveLinks(ctx context.Context, session *common.Session, args []string) ([]*common.Link, error) {
	if len(args) == 0 {
		return rootLinks(ctx, session)
	}

	var links []*common.Link
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
			if link.Type() != proton.LinkTypeFolder {
				return nil, fmt.Errorf("%s: not a directory", arg)
			}
			children, err := link.ListChildren(ctx, true)
			if err != nil {
				return nil, err
			}
			links = append(links, children...)
		} else {
			links = append(links, link)
		}
	}

	return links, nil
}

func rootLinks(ctx context.Context, session *common.Session) ([]*common.Link, error) {
	shares, err := session.ListShares(ctx, true)
	if err != nil {
		return nil, err
	}

	links := make([]*common.Link, len(shares))
	for i := range shares {
		links[i] = shares[i].Link
	}
	return links, nil
}

func filterLinks(links []*common.Link, opts listOpts) []*common.Link {
	var out []*common.Link
	for _, l := range links {
		if l.State() == proton.LinkStateDeleted {
			continue
		}
		if !opts.all && !opts.almostAll {
			name, err := l.Name()
			if err == nil && strings.HasPrefix(name, ".") {
				continue
			}
		}
		out = append(out, l)
	}
	return out
}

func doSort(links []*common.Link, opts listOpts) {
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
			less = links[i].Size() > links[j].Size()
		case sortTime:
			less = links[i].ModifyTime() > links[j].ModifyTime()
		default:
			ni, _ := links[i].Name()
			nj, _ := links[j].Name()
			less = strings.ToLower(ni) < strings.ToLower(nj)
		}
		if opts.reverse {
			return !less
		}
		return less
	})
}

func formatSize(size int64, opts listOpts) string {
	if opts.human {
		return units.HumanSize(float64(size))
	}
	return fmt.Sprintf("%d", size)
}

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

func typeChar(lt proton.LinkType) byte {
	if lt == proton.LinkTypeFolder {
		return 'd'
	}
	return '-'
}

// ANSI color codes for ls-style output.
const (
	colorReset    = "\033[0m"
	colorBoldBlue = "\033[1;34m" // directories
)

// colorName returns the name with ANSI color codes if color is enabled.
func colorName(name string, lt proton.LinkType, useColor bool) string {
	if !useColor {
		return name
	}
	if lt == proton.LinkTypeFolder {
		return colorBoldBlue + name + colorReset
	}
	return name
}

func printLong(l *common.Link, opts listOpts) {
	name, _ := l.Name()
	fmt.Printf("%c%-9s %8s %s %s\n",
		typeChar(l.Type()),
		"rwxr-xr-x",
		formatSize(l.Size(), opts),
		formatTimestamp(l.ModifyTime(), opts.timeStyle),
		colorName(name, l.Type(), opts.color),
	)
}

func printLinks(links []*common.Link, opts listOpts) {
	switch opts.format {
	case formatLong:
		for _, l := range links {
			printLong(l, opts)
		}
	case formatSingle:
		for _, l := range links {
			name, _ := l.Name()
			fmt.Println(colorName(name, l.Type(), opts.color))
		}
	case formatColumns:
		printColumns(links, false, opts.color)
	case formatAcross:
		printColumns(links, true, opts.color)
	}
}

func printColumns(links []*common.Link, across bool, useColor bool) {
	if len(links) == 0 {
		return
	}

	type entry struct {
		name    string
		display string
	}

	entries := make([]entry, len(links))
	maxLen := 0
	for i, l := range links {
		n, _ := l.Name()
		entries[i] = entry{
			name:    n,
			display: colorName(n, l.Type(), useColor),
		}
		if len(n) > maxLen {
			maxLen = len(n)
		}
	}

	colWidth := maxLen + 2
	termWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 { //nolint:gosec
		termWidth = w
	}

	numCols := termWidth / colWidth
	if numCols < 1 {
		numCols = 1
	}
	numRows := (len(entries) + numCols - 1) / numCols

	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			var idx int
			if across {
				idx = row*numCols + col
			} else {
				idx = col*numRows + row
			}
			if idx >= len(entries) {
				continue
			}
			e := entries[idx]
			if col < numCols-1 {
				// Pad based on raw name length, not display length (ANSI codes).
				padding := colWidth - len(e.name)
				if padding < 0 {
					padding = 0
				}
				fmt.Print(e.display)
				for i := 0; i < padding; i++ {
					fmt.Print(" ")
				}
			} else {
				fmt.Print(e.display)
			}
		}
		fmt.Println()
	}
}

func listRecursive(ctx context.Context, prefix string, links []*common.Link, opts listOpts) error {
	for _, l := range links {
		if l.Type() != proton.LinkTypeFolder {
			continue
		}

		name, _ := l.Name()
		path := prefix + name + "/"
		children, err := l.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		children = filterLinks(children, opts)
		doSort(children, opts)

		fmt.Printf("\n%s:\n", prefix+name)
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
