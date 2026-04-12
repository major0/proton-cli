package client

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/major0/proton-cli/api/drive"
)

// ListSharesMetadata returns metadata for all shares visible to this session.
func (c *Client) ListSharesMetadata(ctx context.Context, all bool) ([]drive.ShareMetadata, error) {
	pShares, err := c.Session.Client.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	shares := make([]drive.ShareMetadata, len(pShares))
	for i := range pShares {
		shares[i] = drive.ShareMetadata(pShares[i])
	}
	return shares, nil
}

// GetShareMetadata returns the metadata for the share with the given ID.
func (c *Client) GetShareMetadata(ctx context.Context, id string) (drive.ShareMetadata, error) {
	shares, err := c.Session.Client.ListShares(ctx, true)
	if err != nil {
		return drive.ShareMetadata{}, err
	}

	for _, share := range shares {
		if share.ShareID == id {
			return drive.ShareMetadata(share), nil
		}
	}

	return drive.ShareMetadata{}, nil
}

// ListShares returns all fully-resolved shares visible to this session.
func (c *Client) ListShares(ctx context.Context, all bool) ([]drive.Share, error) {
	return c.listShares(ctx, "", all)
}

func (c *Client) listShares(ctx context.Context, volumeID string, all bool) ([]drive.Share, error) {
	pshares, err := c.Session.Client.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	slog.Debug("client.ListShares", "shares", len(pshares))
	slog.Debug("client.ListShares", "volumeID", volumeID)

	var wg sync.WaitGroup
	idQueue := make(chan string)
	shareQueue := make(chan *drive.Share)
	for i := 0; i < min(c.Session.MaxWorkers, len(pshares)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range idQueue {
				share, err := c.GetShare(ctx, id)
				if err != nil {
					slog.Error("worker", "shareID", id, "error", err)
					continue
				}
				shareQueue <- share
			}
		}()
	}

	// Spawn a producer to feed the idQueue, respecting cancellation.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(idQueue)
		for _, s := range pshares {
			if volumeID != "" && volumeID != s.VolumeID {
				continue
			}
			select {
			case idQueue <- s.ShareID:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to finish, then close the shareQueue to
	// signal the main goroutine.
	go func() {
		wg.Wait()
		close(shareQueue)
	}()

	var shares []drive.Share
	for share := range shareQueue {
		shares = append(shares, *share)
	}

	return shares, nil
}

// GetShare returns the fully-resolved share with the given ID.
func (c *Client) GetShare(ctx context.Context, id string) (*drive.Share, error) {
	pShare, err := c.Session.Client.GetShare(ctx, id)
	if err != nil {
		return nil, err
	}

	shareAddrKR, ok := c.addressKeyRings[pShare.AddressID]
	if !ok {
		return nil, fmt.Errorf("GetShare %s: address keyring not found for %s", id, pShare.AddressID)
	}

	shareKR, err := pShare.GetKeyRing(shareAddrKR)
	if err != nil {
		return nil, err
	}

	pLink, err := c.Session.Client.GetLink(ctx, pShare.ShareID, pShare.LinkID)
	if err != nil {
		return nil, err
	}

	share := drive.NewShare(&pShare, shareKR, nil, c)
	link := drive.NewLink(&pLink, nil, share, c)
	// Set the link on the share after construction to break the circular reference.
	share.Link = link

	return share, nil
}

// ResolveShare finds a share by its root link name.
func (c *Client) ResolveShare(ctx context.Context, name string, all bool) (*drive.Share, error) {
	shares, err := c.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	for i := range shares {
		shareName, err := shares[i].Link.Name()
		if err != nil {
			continue
		}
		if shareName == name {
			return &shares[i], nil
		}
	}

	return nil, drive.ErrFileNotFound
}

// ResolvePath resolves a slash-separated path to a link across all shares.
func (c *Client) ResolvePath(ctx context.Context, path string, all bool) (*drive.Link, error) {
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		return nil, drive.ErrInvalidPath
	}

	share, err := c.ResolveShare(ctx, parts[0], all)
	if err != nil {
		return nil, err
	}

	return share.Link.ResolvePath(ctx, strings.Join(parts[1:], "/"), all)
}
