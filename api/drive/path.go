package drive

import "strings"

// NormalizePath normalizes a drive path: collapses "." and ".." segments,
// removes consecutive "/" separators, and trims leading "/".
// Returns ErrInvalidPath for empty input.
// Does NOT handle proton:// prefix — that is a cmd/ concern.
func NormalizePath(raw string) (string, error) {
	raw = strings.TrimPrefix(raw, "/")
	trailingSlash := ""
	if strings.HasSuffix(raw, "/") {
		trailingSlash = "/"
	}
	var parts []string
	for _, p := range strings.Split(raw, "/") {
		switch p {
		case "", ".":
			continue
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, p)
		}
	}
	result := strings.Join(parts, "/") + trailingSlash
	if result == "" || result == "/" {
		return "", ErrInvalidPath
	}
	return result, nil
}
