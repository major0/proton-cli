package client

import (
	"context"

	"github.com/major0/proton-cli/api/drive"
)

// NamedDirEntry is a directory entry with a resolved name. Lives in the
// client layer because the types layer (api/drive/) does not carry
// decrypted content.
type NamedDirEntry struct {
	Link      *drive.Link
	EntryName string // ".", "..", or decrypted child name
	Err       error
}

// ReaddirNamed wraps link.Readdir and resolves names for each entry.
// The first two entries are . and .. (detected by pointer identity).
// Child names are resolved via resolveName (with optional caching).
func (c *Client) ReaddirNamed(ctx context.Context, dir *drive.Link) <-chan NamedDirEntry {
	ch := make(chan NamedDirEntry)
	go func() {
		defer close(ch)
		idx := 0
		for entry := range dir.Readdir(ctx) {
			if entry.Err != nil {
				select {
				case ch <- NamedDirEntry{Err: entry.Err}:
				case <-ctx.Done():
					return
				}
				idx++
				continue
			}

			var name string
			var err error

			switch {
			case idx == 0:
				name = "."
			case idx == 1:
				name = ".."
			default:
				name, err = c.resolveName(entry.Link)
				if err != nil {
					select {
					case ch <- NamedDirEntry{Err: err}:
					case <-ctx.Done():
						return
					}
					idx++
					continue
				}
			}

			select {
			case ch <- NamedDirEntry{Link: entry.Link, EntryName: name}:
			case <-ctx.Done():
				return
			}
			idx++
		}
	}()
	return ch
}

// resolveName resolves the decrypted name for a link. Currently always
// calls Link.Name() — dirent cache integration deferred to config-store spec.
func (c *Client) resolveName(link *drive.Link) (string, error) {
	return link.Name()
}
