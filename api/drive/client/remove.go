package client

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
)

// Remove moves a link to trash or permanently deletes it.
// Returns an error if the link is a share root. For non-empty folders,
// returns ErrNotEmpty unless opts.Recursive is true.
func (c *Client) Remove(ctx context.Context, share *drive.Share, link *drive.Link, opts drive.RemoveOpts) error {
	if link.ParentLink() == nil {
		return fmt.Errorf("remove: cannot remove share root")
	}

	if link.Type() == proton.LinkTypeFolder && !opts.Recursive {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("remove: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("remove: %s: %w", name, drive.ErrNotEmpty)
		}
	}

	if opts.Permanent {
		return c.Session.Client.DeleteChildren(
			ctx,
			share.ProtonShare().ShareID,
			link.ParentLink().ProtonLink().LinkID,
			link.ProtonLink().LinkID,
		)
	}

	return c.Session.Client.TrashChildren(
		ctx,
		share.ProtonShare().ShareID,
		link.ParentLink().ProtonLink().LinkID,
		link.ProtonLink().LinkID,
	)
}

// EmptyTrash permanently deletes all items in the trash for the given share.
func (c *Client) EmptyTrash(ctx context.Context, share *drive.Share) error {
	return c.Session.Client.EmptyTrash(ctx, share.ProtonShare().ShareID)
}
