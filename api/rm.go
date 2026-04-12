package api

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

// Rm moves a link (file or folder) to the trash. For folders, if recursive
// is false and the folder is not empty, returns ErrNotEmpty.
func (s *Session) Rm(ctx context.Context, share *Share, link *Link, recursive bool) error {
	if link.parentLink == nil {
		return fmt.Errorf("rm: cannot remove share root")
	}

	if link.Type() == proton.LinkTypeFolder && !recursive {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rm: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rm: %s: %w", name, ErrNotEmpty)
		}
	}

	return s.Client.TrashChildren(
		ctx,
		share.protonShare.ShareID,
		link.parentLink.protonLink.LinkID,
		link.protonLink.LinkID,
	)
}

// RmPermanent permanently deletes a link. Not recoverable.
func (s *Session) RmPermanent(ctx context.Context, share *Share, link *Link, recursive bool) error {
	if link.parentLink == nil {
		return fmt.Errorf("rm: cannot remove share root")
	}

	if link.Type() == proton.LinkTypeFolder && !recursive {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rm: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rm: %s: %w", name, ErrNotEmpty)
		}
	}

	return s.Client.DeleteChildren(
		ctx,
		share.protonShare.ShareID,
		link.parentLink.protonLink.LinkID,
		link.protonLink.LinkID,
	)
}

// EmptyTrash permanently deletes all items in the trash for the given share.
func (s *Session) EmptyTrash(ctx context.Context, share *Share) error {
	return s.Client.EmptyTrash(ctx, share.protonShare.ShareID)
}
