package proton

import (
	"context"
	"log/slog"
)

// DirEntry is a single entry yielded by Readdir.
type DirEntry struct {
	Link Link
	Err  error
}

// Readdir returns a channel that yields child links of this folder
// concurrently. The channel is closed when all children have been
// yielded or the context is cancelled. Errors are delivered inline
// as DirEntry values with Err set.
func (l *Link) Readdir(ctx context.Context) <-chan DirEntry {
	ch := make(chan DirEntry)

	go func() {
		defer close(ch)

		slog.Debug("link.Readdir", "linkID", l.protonLink.LinkID)

		pChildren, err := l.session.Client.ListChildren(
			ctx, l.share.protonShare.ShareID, l.protonLink.LinkID, true,
		)
		if err != nil {
			select {
			case ch <- DirEntry{Err: err}:
			case <-ctx.Done():
			}
			return
		}

		for i := range pChildren {
			link, err := l.newLink(ctx, &pChildren[i])
			if err != nil {
				select {
				case ch <- DirEntry{Err: err}:
				case <-ctx.Done():
					return
				}
				continue
			}

			select {
			case ch <- DirEntry{Link: *link}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// Lookup finds a child by name in this folder. Returns nil if not found.
// Uses Readdir internally so it can be cancelled via context.
func (l *Link) Lookup(ctx context.Context, name string) (*Link, error) {
	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			return nil, entry.Err
		}
		if entry.Link.Name == name {
			return &entry.Link, nil
		}
	}
	return nil, nil
}
