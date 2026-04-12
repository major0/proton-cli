package client

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
)

// Rm moves a link (file or folder) to the trash. For folders, if recursive
// is false and the folder is not empty, returns ErrNotEmpty.
func (c *Client) Rm(ctx context.Context, share *drive.Share, link *drive.Link, recursive bool) error {
	if link.ParentLink() == nil {
		return fmt.Errorf("rm: cannot remove share root")
	}

	if link.Type() == proton.LinkTypeFolder && !recursive {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rm: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rm: %s: %w", name, drive.ErrNotEmpty)
		}
	}

	return c.Session.Client.TrashChildren(
		ctx,
		share.ProtonShare().ShareID,
		link.ParentLink().ProtonLink().LinkID,
		link.ProtonLink().LinkID,
	)
}

// RmPermanent permanently deletes a link. Not recoverable.
func (c *Client) RmPermanent(ctx context.Context, share *drive.Share, link *drive.Link, recursive bool) error {
	if link.ParentLink() == nil {
		return fmt.Errorf("rm: cannot remove share root")
	}

	if link.Type() == proton.LinkTypeFolder && !recursive {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rm: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rm: %s: %w", name, drive.ErrNotEmpty)
		}
	}

	return c.Session.Client.DeleteChildren(
		ctx,
		share.ProtonShare().ShareID,
		link.ParentLink().ProtonLink().LinkID,
		link.ProtonLink().LinkID,
	)
}

// RmDir moves a folder to the trash. The link must be a folder and must
// have a parent (cannot trash a share root). Returns ErrNotEmpty if the
// folder has children and force is false.
func (c *Client) RmDir(ctx context.Context, share *drive.Share, link *drive.Link, force bool) error {
	if link.Type() != proton.LinkTypeFolder {
		return drive.ErrNotAFolder
	}

	if link.ParentLink() == nil {
		return fmt.Errorf("rmdir: cannot remove share root")
	}

	if !force {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rmdir: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rmdir: %s: %w", name, drive.ErrNotEmpty)
		}
	}

	return c.Session.Client.TrashChildren(
		ctx,
		share.ProtonShare().ShareID,
		link.ParentLink().ProtonLink().LinkID,
		link.ProtonLink().LinkID,
	)
}

// RmDirPermanent permanently deletes a folder. This is not recoverable.
func (c *Client) RmDirPermanent(ctx context.Context, share *drive.Share, link *drive.Link, force bool) error {
	if link.Type() != proton.LinkTypeFolder {
		return drive.ErrNotAFolder
	}

	if link.ParentLink() == nil {
		return fmt.Errorf("rmdir: cannot remove share root")
	}

	if !force {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rmdir: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rmdir: %s: %w", name, drive.ErrNotEmpty)
		}
	}

	return c.Session.Client.DeleteChildren(
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
