package driveCmd

import (
	"strings"
	"testing"

	driveClient "github.com/major0/proton-cli/api/drive/client"
)

func TestClassifyPath(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want driveClient.PathType
	}{
		{"proton triple-slash", "proton:///Documents/file.txt", driveClient.PathProton},
		{"proton double-slash", "proton://Photos/vacation.jpg", driveClient.PathProton},
		{"proton bare prefix", "proton://", driveClient.PathProton},
		{"absolute local", "/home/user/file.txt", driveClient.PathLocal},
		{"relative local", "./relative/path", driveClient.PathLocal},
		{"bare filename", "file.txt", driveClient.PathLocal},
		{"empty string", "", driveClient.PathLocal},
		{"uppercase prefix", "PROTON://uppercase", driveClient.PathLocal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyPath(tt.arg); got != tt.want {
				t.Errorf("classifyPath(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestArgSplitting(t *testing.T) {
	// resetFlags zeroes cpFlags so tests are independent.
	resetFlags := func() {
		cpFlags = struct {
			recursive   bool
			archive     bool
			dereference bool
			noDeref     bool
			verbose     bool
			progress    bool
			preserve    string
			workers     int
			targetDir   string
			removeDest  bool
			backup      bool
		}{}
	}

	tests := []struct {
		name    string
		args    []string
		setup   func() // optional flag setup before calling runCp
		wantErr string // substring expected in error
	}{
		{
			name:    "default mode valid args",
			args:    []string{"src.txt", "dst.txt"},
			wantErr: "not yet implemented",
		},
		{
			name:    "default mode multiple sources",
			args:    []string{"a.txt", "b.txt", "destdir"},
			wantErr: "not yet implemented",
		},
		{
			name: "target-directory mode",
			args: []string{"a.txt", "b.txt"},
			setup: func() {
				cpFlags.targetDir = "/tmp/dest"
			},
			wantErr: "not yet implemented",
		},
		{
			name:    "fewer than 2 args without -t",
			args:    []string{"only-one"},
			wantErr: "missing destination operand",
		},
		{
			name: "remove-destination and backup mutually exclusive",
			args: []string{"src.txt", "dst.txt"},
			setup: func() {
				cpFlags.removeDest = true
				cpFlags.backup = true
			},
			wantErr: "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			if tt.setup != nil {
				tt.setup()
			}

			err := runCp(nil, tt.args)
			if err == nil {
				t.Fatal("runCp() returned nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("runCp() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
