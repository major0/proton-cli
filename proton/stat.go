package proton

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// StatLink resolves a single link ID within a share into a fully-populated
// Link with decrypted name, size, and timestamps.
func (s *Session) StatLink(ctx context.Context, share *Share, parentLink *Link, linkID string) (*Link, error) {
	pLink, err := s.Client.GetLink(ctx, share.protonShare.ShareID, linkID)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", linkID, err)
	}

	return s.newLink(ctx, share, parentLink, &pLink)
}

// StatLinks resolves a batch of link IDs concurrently. Up to MaxWorkers
// goroutines run in parallel. Links that fail to resolve are logged and
// skipped. Respects context cancellation.
func (s *Session) StatLinks(ctx context.Context, share *Share, parentLink *Link, linkIDs []string) ([]Link, error) {
	if len(linkIDs) == 0 {
		return nil, nil
	}

	workers := min(s.MaxWorkers, len(linkIDs))

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		links   []Link
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
				link, err := s.StatLink(ctx, share, parentLink, id)
				if err != nil {
					slog.Error("stat", "linkID", id, "error", err)
					continue
				}
				mu.Lock()
				links = append(links, *link)
				mu.Unlock()
			}
		}()
	}

	// Feed IDs, respecting cancellation.
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
// matching the given name. Returns nil if no match is found. Workers are
// cancelled as soon as a match is found.
func (s *Session) FindLinkByName(ctx context.Context, share *Share, parentLink *Link, linkIDs []string, name string) (*Link, error) {
	if len(linkIDs) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workers := min(s.MaxWorkers, len(linkIDs))

	type result struct {
		link *Link
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
				link, err := s.StatLink(ctx, share, parentLink, id)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					slog.Error("stat", "linkID", id, "error", err)
					continue
				}
				if link.Name == name {
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
