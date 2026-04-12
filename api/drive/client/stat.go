package client

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

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

	workers := min(c.Session.MaxWorkers, len(linkIDs))

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		links   []*drive.Link
		idQueue = make(chan string)
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range idQueue {
				if ctx.Err() != nil {
					return
				}
				link, err := c.StatLink(ctx, share, parentLink, id)
				if err != nil {
					slog.Error("stat", "linkID", id, "error", err)
					continue
				}
				mu.Lock()
				links = append(links, link)
				mu.Unlock()
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

	wg.Wait()
	return links, ctx.Err()
}

// FindLinkByName resolves link IDs concurrently and returns the first one
// whose decrypted name matches. Returns nil if no match is found.
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
