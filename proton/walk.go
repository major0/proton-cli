package proton

import (
	"context"
	"errors"
	"path"

	goProton "github.com/ProtonMail/go-proton-api"
)

// WalkEntry is a single entry yielded by Walk.
type WalkEntry struct {
	Path string
	Link Link
	Err  error
}

// WalkFunc is called for each entry during a Walk. Return a non-nil error
// to stop the walk. Return SkipDir to skip descending into a directory.
type WalkFunc func(path string, link *Link, err error) error

// SkipDir can be returned by WalkFunc to skip a directory subtree.
// It is an alias for ErrSkipDir.
var SkipDir = ErrSkipDir //nolint:errname // matches fs.SkipDir convention

// Walk traverses the directory tree rooted at this link, calling fn for
// each file or directory. Directories are visited before their contents.
// If fn returns SkipDir for a directory, its children are not visited.
func (l *Link) Walk(ctx context.Context, fn WalkFunc) error {
	return walkLink(ctx, l.Name, l, fn)
}

func walkLink(ctx context.Context, p string, l *Link, fn WalkFunc) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := fn(p, l, nil); err != nil {
		if errors.Is(err, SkipDir) {
			return nil
		}
		return err
	}

	if l.Type != goProton.LinkTypeFolder {
		return nil
	}

	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			if err := fn(path.Join(p, "?"), nil, entry.Err); err != nil {
				return err
			}
			continue
		}

		child := entry.Link
		if err := walkLink(ctx, path.Join(p, child.Name), &child, fn); err != nil {
			return err
		}
	}

	return nil
}
