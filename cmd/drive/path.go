package driveCmd

import (
	"context"
	"fmt"
	"strings"

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

// resolveProtonPath strips the proton:// prefix and resolves via the client.
func resolveProtonPath(ctx context.Context, dc *driveClient.Client, rawPath string) (*drive.Link, *drive.Share, error) {
	stripped, err := stripProtonPrefix(rawPath)
	if err != nil {
		return nil, nil, err
	}
	return dc.ResolveDrivePath(ctx, stripped)
}
