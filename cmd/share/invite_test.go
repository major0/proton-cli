package shareCmd

import (
	"testing"

	"github.com/major0/proton-cli/api/share"
)

func TestParsePermissions(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"read", share.PermViewer, false},
		{"viewer", share.PermViewer, false},
		{"write", share.PermEditor, false},
		{"editor", share.PermEditor, false},
		{"admin", 0, true},
		{"", 0, true},
		{"rw", 0, true},
	}
	for _, tt := range tests {
		got, err := parsePermissions(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parsePermissions(%q) = %d, want error", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePermissions(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parsePermissions(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
