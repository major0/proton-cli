package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/major0/proton-cli/api/drive"
)

// ResolveDrivePath resolves a normalized drive path to its Link and Share.
// The path format is "sharename/relative/path". The proton:// prefix must
// be stripped by the caller before calling this method.
func (c *Client) ResolveDrivePath(ctx context.Context, rawPath string) (*drive.Link, *drive.Share, error) {
	path, err := drive.NormalizePath(rawPath)
	if err != nil {
		return nil, nil, err
	}

	parts := strings.SplitN(path, "/", 2)
	shareName := parts[0]

	share, err := c.ResolveShare(ctx, shareName, true)
	if err != nil {
		return nil, nil, err
	}

	if len(parts) == 1 || parts[1] == "" {
		return share.Link, share, nil
	}

	link, err := share.Link.ResolvePath(ctx, parts[1], true)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %s: %w", rawPath, err)
	}

	return link, share, nil
}
