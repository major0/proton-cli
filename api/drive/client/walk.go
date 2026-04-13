package client

import (
	"context"
	"path"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
)

// TreeWalk walks the directory tree rooted at root and sends each entry
// to the results channel. The caller owns the channel and controls
// buffering, backpressure, and lifetime. Cancel ctx to stop the walk.
func (c *Client) TreeWalk(ctx context.Context, root *drive.Link, rootPath string, order drive.WalkOrder, results chan<- drive.WalkEntry) error {
	switch order {
	case drive.DepthFirst:
		return c.walkDepthFirst(ctx, root, rootPath, 0, results)
	default:
		return c.walkBreadthFirst(ctx, root, rootPath, results)
	}
}

type queueItem struct {
	link  *drive.Link
	path  string
	depth int
}

func (c *Client) walkBreadthFirst(ctx context.Context, root *drive.Link, rootPath string, results chan<- drive.WalkEntry) error {
	// Emit the root itself.
	select {
	case results <- drive.WalkEntry{Path: rootPath, Link: root, Depth: 0}:
	case <-ctx.Done():
		return ctx.Err()
	}

	queue := []queueItem{{link: root, path: rootPath, depth: 0}}

	for len(queue) > 0 {
		var next []queueItem

		for _, item := range queue {
			if item.link.Type() != proton.LinkTypeFolder {
				continue
			}

			idx := 0
			for entry := range item.link.Readdir(ctx) {
				if entry.Err != nil {
					continue
				}

				// Skip . and .. (first two entries).
				if idx < 2 {
					idx++
					continue
				}
				idx++

				childName, err := entry.Link.Name()
				if err != nil {
					continue
				}

				childPath := path.Join(item.path, childName)
				childDepth := item.depth + 1

				select {
				case results <- drive.WalkEntry{Path: childPath, Link: entry.Link, Depth: childDepth}:
				case <-ctx.Done():
					return ctx.Err()
				}

				if entry.Link.Type() == proton.LinkTypeFolder {
					next = append(next, queueItem{link: entry.Link, path: childPath, depth: childDepth})
				}
			}
		}

		queue = next
	}

	return nil
}

func (c *Client) walkDepthFirst(ctx context.Context, link *drive.Link, linkPath string, depth int, results chan<- drive.WalkEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// If folder, recurse into children first (post-order).
	if link.Type() == proton.LinkTypeFolder {
		idx := 0
		for entry := range link.Readdir(ctx) {
			if entry.Err != nil {
				continue
			}

			// Skip . and .. (first two entries).
			if idx < 2 {
				idx++
				continue
			}
			idx++

			childName, err := entry.Link.Name()
			if err != nil {
				continue
			}

			childPath := path.Join(linkPath, childName)
			if err := c.walkDepthFirst(ctx, entry.Link, childPath, depth+1, results); err != nil {
				return err
			}
		}
	}

	// Emit this entry after all descendants.
	select {
	case results <- drive.WalkEntry{Path: linkPath, Link: link, Depth: depth}:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
