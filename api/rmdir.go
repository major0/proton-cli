package api

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

// RmDir moves a folder to the trash. The link must be a folder and must
// have a parent (cannot trash a share root). Returns ErrNotEmpty if the
// folder has children and force is false.
func (s *Session) RmDir(ctx context.Context, share *Share, link *Link, force bool) error {
	if link.Type() != proton.LinkTypeFolder {
		return ErrNotAFolder
	}

	if link.parentLink == nil {
		return fmt.Errorf("rmdir: cannot remove share root")
	}

	if !force {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rmdir: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rmdir: %s: %w", name, ErrNotEmpty)
		}
	}

	return s.Client.TrashChildren(
		ctx,
		share.protonShare.ShareID,
		link.parentLink.protonLink.LinkID,
		link.protonLink.LinkID,
	)
}

// RmDirPermanent permanently deletes a folder. This is not recoverable.
func (s *Session) RmDirPermanent(ctx context.Context, share *Share, link *Link, force bool) error {
	if link.Type() != proton.LinkTypeFolder {
		return ErrNotAFolder
	}

	if link.parentLink == nil {
		return fmt.Errorf("rmdir: cannot remove share root")
	}

	if !force {
		children, err := link.ListChildren(ctx, true)
		if err != nil {
			return fmt.Errorf("rmdir: listing children: %w", err)
		}
		if len(children) > 0 {
			name, _ := link.Name()
			return fmt.Errorf("rmdir: %s: %w", name, ErrNotEmpty)
		}
	}

	return s.Client.DeleteChildren(
		ctx,
		share.protonShare.ShareID,
		link.parentLink.protonLink.LinkID,
		link.protonLink.LinkID,
	)
}
