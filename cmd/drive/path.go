package driveCmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	driveClient "github.com/major0/proton-cli/api/drive/client"
)

// stripProtonPrefix removes the "proton://" prefix from a raw CLI path.
// This is a cmd/ concern — the api/ layer never sees the prefix.
func stripProtonPrefix(rawPath string) (string, error) {
	if !strings.HasPrefix(rawPath, "proton://") {
		return "", fmt.Errorf("invalid path: %s (must start with proton://)", rawPath)
	}
	return strings.TrimPrefix(rawPath, "proton://"), nil
}

// parsePath strips the proton:// prefix and normalizes the path.
// Retained as a convenience for command handlers that only need the
// normalized path string (e.g., for display or splitting).
func parsePath(rawPath string) string {
	stripped, err := stripProtonPrefix(rawPath)
	if err != nil {
		return ""
	}
	normalized, err := drive.NormalizePath(stripped)
	if err != nil {
		return ""
	}
	return normalized
}

// resolveShareComponent resolves the share part of a proton:// URI.
// Priority: {id} brackets → well-known alias → share name.
func resolveShareComponent(ctx context.Context, dc *driveClient.Client, sharePart string) (*drive.Share, error) {
	// 1. Direct share ID: {ABC123DEF-456}
	if strings.HasPrefix(sharePart, "{") && strings.HasSuffix(sharePart, "}") {
		id := sharePart[1 : len(sharePart)-1]
		return dc.GetShare(ctx, id)
	}

	// 2. Well-known aliases (case-sensitive).
	switch sharePart {
	case "Drive":
		return dc.ResolveShareByType(ctx, proton.ShareTypeMain)
	case "Photos":
		return dc.ResolveShareByType(ctx, drive.ShareTypePhotos)
	}

	// 3. Resolve by decrypted share root link name.
	return dc.ResolveShare(ctx, sharePart, true)
}

// resolveProtonPath strips the proton:// prefix and resolves via the client.
// Uses resolveShareComponent for the share part to support {id}, Drive/Photos
// aliases, and share name resolution.
func resolveProtonPath(ctx context.Context, dc *driveClient.Client, rawPath string) (*drive.Link, *drive.Share, error) {
	stripped, err := stripProtonPrefix(rawPath)
	if err != nil {
		return nil, nil, err
	}

	path, err := drive.NormalizePath(stripped)
	if err != nil {
		return nil, nil, err
	}

	parts := strings.SplitN(path, "/", 2)
	sharePart := parts[0]

	share, err := resolveShareComponent(ctx, dc, sharePart)
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
