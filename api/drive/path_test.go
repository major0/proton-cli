package drive

import (
	"errors"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestNormalizePath_Property verifies NormalizePath output invariants.
// **Validates: Requirements 1.5**
// **Property 1: NormalizePath Round-Trip**
func TestNormalizePath_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random path segments including ".", "..", normal names,
		// empty strings (to simulate consecutive "/"), and leading "/".
		numSegments := rapid.IntRange(0, 10).Draw(t, "numSegments")
		segments := make([]string, numSegments)
		for i := range segments {
			segments[i] = rapid.OneOf(
				rapid.Just("."),
				rapid.Just(".."),
				rapid.Just(""),
				rapid.StringMatching(`[a-zA-Z0-9_-]{1,10}`),
			).Draw(t, "segment")
		}

		leadingSlash := rapid.Bool().Draw(t, "leadingSlash")
		trailingSlash := rapid.Bool().Draw(t, "trailingSlash")

		raw := strings.Join(segments, "/")
		if leadingSlash {
			raw = "/" + raw
		}
		if trailingSlash {
			raw += "/"
		}

		result, err := NormalizePath(raw)

		if err != nil {
			// If error, it must be ErrInvalidPath.
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("expected ErrInvalidPath, got %v", err)
			}
			return
		}

		// Valid result: check invariants.

		// No leading "/".
		if strings.HasPrefix(result, "/") {
			t.Fatalf("result has leading /: %q", result)
		}

		// No "." or ".." segments.
		parts := strings.Split(result, "/")
		for _, p := range parts {
			if p == "." || p == ".." {
				t.Fatalf("result contains . or .. segment: %q", result)
			}
		}

		// No consecutive "/" (no empty segments except possibly trailing).
		trimmed := strings.TrimSuffix(result, "/")
		if trimmed != "" {
			for _, p := range strings.Split(trimmed, "/") {
				if p == "" {
					t.Fatalf("result contains consecutive /: %q", result)
				}
			}
		}

		// Result is non-empty.
		if result == "" {
			t.Fatal("result is empty but no error returned")
		}
	})
}

// TestNormalizePath_EmptyInput verifies empty input returns ErrInvalidPath.
func TestNormalizePath_EmptyInput(t *testing.T) {
	_, err := NormalizePath("")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath for empty input, got %v", err)
	}
}

// TestNormalizePath_SlashOnly verifies "/" returns ErrInvalidPath.
func TestNormalizePath_SlashOnly(t *testing.T) {
	_, err := NormalizePath("/")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath for /, got %v", err)
	}
}

// TestNormalizePath_DotOnly verifies "." returns ErrInvalidPath.
func TestNormalizePath_DotOnly(t *testing.T) {
	_, err := NormalizePath(".")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath for ., got %v", err)
	}
}

// TestNormalizePath_Basic verifies basic normalization cases.
func TestNormalizePath_Basic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo/bar", "foo/bar"},
		{"/foo/bar", "foo/bar"},
		{"foo//bar", "foo/bar"},
		{"foo/./bar", "foo/bar"},
		{"foo/../bar", "bar"},
		{"foo/bar/", "foo/bar/"},
		{"a/b/../c/./d", "a/c/d"},
	}
	for _, tt := range tests {
		got, err := NormalizePath(tt.input)
		if err != nil {
			t.Errorf("NormalizePath(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
