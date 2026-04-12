package api

import (
	"context"
	"errors"
	"path"

	goProton "github.com/ProtonMail/go-proton-api"
)

// WalkFunc is called for each entry during a Walk. Return a non-nil error
// to stop the walk. Return SkipDir to skip descending into a directory.
type WalkFunc func(path string, link *Link, err error) error

// SkipDir can be returned by WalkFunc to skip a directory subtree.
var SkipDir = ErrSkipDir //nolint:errname // matches fs.SkipDir convention

// Walk traverses the directory tree rooted at this link, calling fn for
// each file or directory. Directories are visited before their contents.
// If fn returns SkipDir for a directory, its children are not visited.
func (l *Link) Walk(ctx context.Context, fn WalkFunc) error {
	name, err := l.Name()
	if err != nil {
		return fn("?", l, err)
	}
	return walkLink(ctx, name, l, fn)
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

	if l.Type() != goProton.LinkTypeFolder {
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
		childName, err := child.Name()
		if err != nil {
			if err := fn(path.Join(p, "?"), child, err); err != nil {
				return err
			}
			continue
		}

		if err := walkLink(ctx, path.Join(p, childName), child, fn); err != nil {
			return err
		}
	}

	return nil
}
