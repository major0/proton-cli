package driveCmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	driveClient "github.com/major0/proton-cli/api/drive/client"
)

// parseProtonURI parses a proton:// URI into its share and path components.
//
// URI format: proton://<share>/<path>
//
//   - proton://Drive/Documents/file.txt → share="Drive", path="Documents/file.txt"
//   - proton:///path/to/file            → share="" (empty → root share), path="path/to/file"
//   - proton://                         → error: no share specified
//   - proton://{id}/path                → share="{id}", path="path"
//
// The proton:// prefix is a cmd/ concern — the api/ layer never sees it.
func parseProtonURI(rawPath string) (sharePart, pathPart string, err error) {
	if !strings.HasPrefix(rawPath, "proton://") {
		return "", "", fmt.Errorf("invalid path: %s (must start with proton://)", rawPath)
	}

	// Strip the "proton://" prefix.
	remainder := strings.TrimPrefix(rawPath, "proton://")

	// Bare "proton://" with nothing after it → error.
	if remainder == "" {
		return "", "", fmt.Errorf("no share specified (use proton://<share>/<path> or proton:///<path> for root share)")
	}

	// Triple-slash: proton:///path → empty share (root), path starts after.
	if strings.HasPrefix(remainder, "/") {
		pathPart = strings.TrimPrefix(remainder, "/")
		normalized, err := drive.NormalizePath(pathPart)
		if err != nil {
			return "", "", err
		}
		return "", normalized, nil
	}

	// Split on first "/" to separate share from path.
	parts := strings.SplitN(remainder, "/", 2)
	sharePart = parts[0]

	if len(parts) == 1 || parts[1] == "" {
		// proton://Drive or proton://Drive/ → share root, no sub-path.
		return sharePart, "", nil
	}

	normalized, err := drive.NormalizePath(parts[1])
	if err != nil {
		// Path normalized to empty — treat as share root.
		return sharePart, "", nil
	}

	return sharePart, normalized, nil
}

// parsePath strips the proton:// prefix and returns the normalized
// share/path string. Retained for command handlers that need the
// combined string for display or splitting.
func parsePath(rawPath string) string {
	sharePart, pathPart, err := parseProtonURI(rawPath)
	if err != nil {
		return ""
	}
	if sharePart == "" && pathPart == "" {
		return ""
	}
	if pathPart == "" {
		return sharePart
	}
	if sharePart == "" {
		return pathPart
	}
	return sharePart + "/" + pathPart
}

// resolveShareComponent resolves the share part of a proton:// URI.
//
// Resolution priority:
//  1. Empty string → root share (main volume share)
//  2. {id} brackets → resolve by share ID directly
//  3. "Drive" → main volume share (ShareTypeMain)
//  4. "Photos" → photos share (ShareTypePhotos)
//  5. Otherwise → resolve by decrypted share root link name
func resolveShareComponent(ctx context.Context, dc *driveClient.Client, sharePart string) (*drive.Share, error) {
	// Empty share → root share (triple-slash case).
	if sharePart == "" {
		return dc.ResolveShareByType(ctx, proton.ShareTypeMain)
	}

	// Direct share ID: {ABC123DEF-456}
	if strings.HasPrefix(sharePart, "{") && strings.HasSuffix(sharePart, "}") {
		id := sharePart[1 : len(sharePart)-1]
		return dc.GetShare(ctx, id)
	}

	// Well-known aliases (case-sensitive).
	switch sharePart {
	case "Drive":
		return dc.ResolveShareByType(ctx, proton.ShareTypeMain)
	case "Photos":
		return dc.ResolveShareByType(ctx, drive.ShareTypePhotos)
	}

	// Resolve by decrypted share root link name.
	return dc.ResolveShare(ctx, sharePart, true)
}

// resolveProtonPath parses a proton:// URI and resolves it to a Link and Share.
func resolveProtonPath(ctx context.Context, dc *driveClient.Client, rawPath string) (*drive.Link, *drive.Share, error) {
	sharePart, pathPart, err := parseProtonURI(rawPath)
	if err != nil {
		return nil, nil, err
	}

	share, err := resolveShareComponent(ctx, dc, sharePart)
	if err != nil {
		return nil, nil, err
	}

	if pathPart == "" {
		return share.Link, share, nil
	}

	link, err := share.Link.ResolvePath(ctx, pathPart, true)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %s: %w", rawPath, err)
	}

	return link, share, nil
}
