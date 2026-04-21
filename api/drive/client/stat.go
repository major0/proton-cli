package client

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/api/drive"
	"github.com/major0/proton-cli/api/pool"
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

// StatLinks resolves a batch of link IDs concurrently using the
// session's worker pool. Links that fail to resolve are logged and
// skipped. Respects context cancellation.
func (c *Client) StatLinks(_ context.Context, share *drive.Share, parentLink *drive.Link, linkIDs []string) ([]*drive.Link, error) {
	if len(linkIDs) == 0 {
		return nil, nil
	}

	var mu sync.Mutex
	links := make([]*drive.Link, 0, len(linkIDs))

	for _, id := range linkIDs {
		c.Session.Pool.Go(func(ctx context.Context) error {
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

	if err := c.Session.Pool.Wait(); err != nil {
		return links, err
	}
	return links, nil
}

// FindLinkByName resolves link IDs concurrently and returns the first one
// whose decrypted name matches. Returns nil if no match is found.
//
// Uses a short-lived pool with a child context for early cancellation:
// when a match is found, the context is cancelled to stop remaining workers.
func (c *Client) FindLinkByName(ctx context.Context, share *drive.Share, parentLink *drive.Link, linkIDs []string, name string) (*drive.Link, error) {
	if len(linkIDs) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workers := min(api.DefaultMaxWorkers, len(linkIDs))
	p := pool.New(ctx, workers)

	var (
		found   *drive.Link
		foundMu sync.Mutex
	)

	for _, id := range linkIDs {
		p.Go(func(ctx context.Context) error {
			link, err := c.StatLink(ctx, share, parentLink, id)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				slog.Error("stat", "linkID", id, "error", err)
				return nil
			}
			linkName, err := link.Name()
			if err != nil {
				slog.Error("stat", "linkID", id, "error", err)
				return nil
			}
			if linkName == name {
				foundMu.Lock()
				if found == nil {
					found = link
				}
				foundMu.Unlock()
				cancel()
			}
			return nil
		})
	}

	// Context cancellation from cancel() causes Wait to return ctx.Err();
	// that's expected when we found a match — not a real error.
	_ = p.Wait()

	return found, nil
}
