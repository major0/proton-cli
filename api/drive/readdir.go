package drive

import (
	"context"
	"log/slog"
	"sync"
)

// DirEntry is a single entry yielded by Readdir. The types layer does
// not carry decrypted content — consumers that need names should use
// the client layer's ReaddirNamed.
type DirEntry struct {
	Link *Link
	Err  error
}

// Readdir returns a channel that yields directory entries for this folder.
// The first two entries are always . (self) and .. (parent), followed by
// children fetched via the resolver. Children are yielded without name
// decryption — the types layer constructs child Links only.
//
// The channel is closed when all entries have been yielded or the context
// is cancelled.
func (l *Link) Readdir(ctx context.Context) <-chan DirEntry {
	ch := make(chan DirEntry)

	go func() {
		defer close(ch)

		slog.Debug("link.Readdir", "linkID", l.protonLink.LinkID)

		// Emit . (self) and .. (parent) as the first two entries.
		// For share roots, both point to the same link (POSIX /.. → /).
		select {
		case ch <- DirEntry{Link: l}:
		case <-ctx.Done():
			return
		}
		select {
		case ch <- DirEntry{Link: l.Parent()}:
		case <-ctx.Done():
			return
		}

		// Respect throttle before making the API call.
		if throttle := l.resolver.Throttle(); throttle != nil {
			if err := throttle.Wait(ctx); err != nil {
				select {
				case ch <- DirEntry{Err: err}:
				case <-ctx.Done():
				}
				return
			}
		}

		pChildren, err := l.resolver.ListLinkChildren(
			ctx, l.share.protonShare.ShareID, l.protonLink.LinkID, true,
		)
		if err != nil {
			// Signal throttle on 429-like errors.
			if throttle := l.resolver.Throttle(); throttle != nil {
				throttle.Signal(0)
			}
			select {
			case ch <- DirEntry{Err: err}:
			case <-ctx.Done():
			}
			return
		}

		if throttle := l.resolver.Throttle(); throttle != nil {
			throttle.Reset()
		}

		if len(pChildren) == 0 {
			return
		}

		// Fan out child link construction across workers.
		workers := min(l.resolver.MaxWorkers(), len(pChildren))
		indexCh := make(chan int)
		var wg sync.WaitGroup

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range indexCh {
					child := l.resolver.NewChildLink(ctx, l, &pChildren[idx])

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
// Handles "." (self) and ".." (parent) directly without scanning children.
// Cancels remaining work as soon as the match is found.
func (l *Link) Lookup(ctx context.Context, name string) (*Link, error) {
	// Fast path for . and ..
	switch name {
	case ".":
		return l, nil
	case "..":
		return l.Parent(), nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	first := true
	second := true
	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			return nil, entry.Err
		}
		// Skip . and .. (first two entries by convention).
		if first {
			first = false
			continue
		}
		if second {
			second = false
			continue
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
// Excludes the synthetic . and .. entries (first two from Readdir).
// Built on Readdir — prefer Readdir for streaming or early termination.
func (l *Link) ListChildren(ctx context.Context, _ bool) ([]*Link, error) {
	links := make([]*Link, 0, 16)
	idx := 0
	for entry := range l.Readdir(ctx) {
		if entry.Err != nil {
			return nil, entry.Err
		}
		// Skip . (idx 0) and .. (idx 1).
		if idx < 2 {
			idx++
			continue
		}
		idx++
		links = append(links, entry.Link)
	}
	return links, nil
}
