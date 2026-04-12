package api

import (
	"context"
	"log/slog"
	"sync"
)

// DirEntry is a single entry yielded by Readdir.
type DirEntry struct {
	Link *Link
	Err  error
}

// Readdir returns a channel that yields child links of this folder.
// Children are fetched in one API call. Name decryption is performed
// concurrently across workers. The channel is closed when all children
// have been yielded or the context is cancelled.
//
// The returned Links are lazily decrypted — only the name is decrypted
// during Readdir (needed for the caller to identify entries). Keyrings
// and other fields are decrypted on demand.
func (l *Link) Readdir(ctx context.Context) <-chan DirEntry {
	ch := make(chan DirEntry)

	go func() {
		defer close(ch)

		slog.Debug("link.Readdir", "linkID", l.protonLink.LinkID)

		// Respect throttle before making the API call.
		if l.session.Throttle != nil {
			if err := l.session.Throttle.Wait(ctx); err != nil {
				select {
				case ch <- DirEntry{Err: err}:
				case <-ctx.Done():
				}
				return
			}
		}

		pChildren, err := l.session.Client.ListChildren(
			ctx, l.share.protonShare.ShareID, l.protonLink.LinkID, true,
		)
		if err != nil {
			// Signal throttle on 429-like errors.
			if l.session.Throttle != nil {
				l.session.Throttle.Signal(0)
			}
			select {
			case ch <- DirEntry{Err: err}:
			case <-ctx.Done():
			}
			return
		}

		if l.session.Throttle != nil {
			l.session.Throttle.Reset()
		}

		if len(pChildren) == 0 {
			return
		}

		// Fan out name decryption across workers.
		workers := min(l.session.MaxWorkers, len(pChildren))
		indexCh := make(chan int)
		var wg sync.WaitGroup

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range indexCh {
					child := l.newChildLink(ctx, &pChildren[idx])

					// Trigger name decryption so the caller can
					// identify the entry. Other fields stay encrypted.
					if err := child.decrypt(); err != nil {
						select {
						case ch <- DirEntry{Err: err}:
						case <-ctx.Done():
							return
						}
						continue
					}

					select {
					case ch <- DirEntry{Link: child}:
					case <-ctx.Done():
						return
					}
				}
			}()
		}

		// Feed indices, respecting cancellation.
		go func() {
			defer close(indexCh)
			for i := range pChildren {
				select {
				case indexCh <- i:
				case <-ctx.Done():
					return
				}
			}
		}()

		wg.Wait()
	}()

	return ch
}

// Lookup finds a child by name in this folder. Returns nil if not found.
// Cancels remaining decryption work as soon as the match is found.
func (l *Link) Lookup(ctx context.Context, name string) (*Link, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			return nil, entry.Err
		}
		entryName, err := entry.Link.Name()
		if err != nil {
			return nil, err
		}
		if entryName == name {
			return entry.Link, nil
		}
	}
	return nil, nil
}

// ListChildren returns all child links of this folder as a slice.
// Built on Readdir — prefer Readdir for streaming or early termination.
func (l *Link) ListChildren(ctx context.Context, _ bool) ([]*Link, error) {
	var links []*Link
	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			return nil, entry.Err
		}
		links = append(links, entry.Link)
	}
	return links, nil
}
