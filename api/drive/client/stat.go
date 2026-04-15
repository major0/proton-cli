package client

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/major0/proton-cli/api/drive"
)

// StatLink resolves a single link ID within a share into a Link.
// The returned Link is lazily decrypted — call Name(), KeyRing(), etc.
// to trigger decryption on demand.
func (c *Client) StatLink(ctx context.Context, share *drive.Share, parentLink *drive.Link, linkID string) (*drive.Link, error) {
	pLink, err := c.Session.Client.GetLink(ctx, share.ProtonShare().ShareID, linkID)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", linkID, err)
	}

	return drive.NewLink(&pLink, parentLink, share, c), nil
}

// StatLinks resolves a batch of link IDs concurrently. Up to MaxWorkers
// goroutines run in parallel. Links that fail to resolve are logged and
// skipped. Respects context cancellation.
func (c *Client) StatLinks(ctx context.Context, share *drive.Share, parentLink *drive.Link, linkIDs []string) ([]*drive.Link, error) {
	if len(linkIDs) == 0 {
		return nil, nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(c.Session.MaxWorkers)

	var mu sync.Mutex
	links := make([]*drive.Link, 0, len(linkIDs))

	for _, id := range linkIDs {
		g.Go(func() error {
			link, err := c.StatLink(ctx, share, parentLink, id)
			if err != nil {
				slog.Error("stat", "linkID", id, "error", err)
				return nil // log and skip, don't fail the batch
			}
			mu.Lock()
			links = append(links, link)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return links, err
	}
	return links, nil
}

// FindLinkByName resolves link IDs concurrently and returns the first one
// whose decrypted name matches. Returns nil if no match is found.
//
// This function keeps the manual channel + WaitGroup pattern instead of
// errgroup because it needs early cancellation: when a match is found,
// remaining workers must stop immediately. errgroup.WithContext would
// cancel on error, but here we cancel on success — a semantic mismatch
// that would require returning a sentinel error to trigger cancellation,
// making the code less clear than the explicit cancel() call.
func (c *Client) FindLinkByName(ctx context.Context, share *drive.Share, parentLink *drive.Link, linkIDs []string, name string) (*drive.Link, error) {
	if len(linkIDs) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workers := min(c.Session.MaxWorkers, len(linkIDs))

	type result struct {
		link *drive.Link
		err  error
	}

	idQueue := make(chan string)
	resultCh := make(chan result, 1)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range idQueue {
				link, err := c.StatLink(ctx, share, parentLink, id)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					slog.Error("stat", "linkID", id, "error", err)
					continue
				}
				linkName, err := link.Name()
				if err != nil {
					slog.Error("stat", "linkID", id, "error", err)
					continue
				}
				if linkName == name {
					select {
					case resultCh <- result{link: link}:
						cancel()
					default:
					}
					return
				}
			}
		}()
	}

	go func() {
		defer close(idQueue)
		for _, id := range linkIDs {
			select {
			case idQueue <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	if r, ok := <-resultCh; ok {
		return r.link, r.err
	}

	return nil, nil
}
