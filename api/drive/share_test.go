package drive

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"pgregory.net/rapid"
)

// TestFormatShareType_Property verifies FormatShareType completeness.
// **Property 2: FormatShareType Completeness**
// **Validates: Requirement 2.2**
func TestFormatShareType_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		st := proton.ShareType(rapid.IntRange(0, 255).Draw(t, "shareType"))
		result := FormatShareType(st)

		// Must return a non-empty string for any input.
		if result == "" {
			t.Fatalf("FormatShareType(%d) returned empty string", st)
		}

		// Known types must return expected labels.
		switch st {
		case proton.ShareTypeMain:
			if result != "main" {
				t.Fatalf("FormatShareType(Main) = %q, want %q", result, "main")
			}
		case proton.ShareTypeStandard:
			if result != "shared" {
				t.Fatalf("FormatShareType(Standard) = %q, want %q", result, "shared")
			}
		case proton.ShareTypeDevice:
			if result != "device" {
				t.Fatalf("FormatShareType(Device) = %q, want %q", result, "device")
			}
		case ShareTypePhotos:
			if result != "photos" {
				t.Fatalf("FormatShareType(Photos) = %q, want %q", result, "photos")
			}
		default:
			expected := fmt.Sprintf("unknown(%d)", st)
			if result != expected {
				t.Fatalf("FormatShareType(%d) = %q, want %q", st, result, expected)
			}
		}
	})
}

// TestFormatShareType_KnownTypes verifies known share type labels.
func TestFormatShareType_KnownTypes(t *testing.T) {
	tests := []struct {
		input proton.ShareType
		want  string
	}{
		{proton.ShareTypeMain, "main"},
		{proton.ShareTypeStandard, "shared"},
		{proton.ShareTypeDevice, "device"},
		{ShareTypePhotos, "photos"},
	}
	for _, tt := range tests {
		got := FormatShareType(tt.input)
		if got != tt.want {
			t.Errorf("FormatShareType(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestFormatShareType_Unknown verifies unknown types match the pattern.
func TestFormatShareType_Unknown(t *testing.T) {
	got := FormatShareType(99)
	if !strings.HasPrefix(got, "unknown(") {
		t.Errorf("FormatShareType(99) = %q, want unknown(N) pattern", got)
	}
}
