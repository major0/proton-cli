package driveCmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	driveClient "github.com/major0/proton-cli/api/drive/client"
	cli "github.com/major0/proton-cli/cmd"
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

	// Create temp files/dirs so path resolution succeeds and dispatch
	// reaches cpSingle → "not yet implemented".
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "src.txt")
	if err := os.WriteFile(srcFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	srcA := filepath.Join(tmp, "a.txt")
	if err := os.WriteFile(srcA, []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	srcB := filepath.Join(tmp, "b.txt")
	if err := os.WriteFile(srcB, []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	destDir := filepath.Join(tmp, "destdir")
	if err := os.Mkdir(destDir, 0700); err != nil {
		t.Fatal(err)
	}
	// dstFile is a non-existent path whose parent exists.
	dstFile := filepath.Join(tmp, "dst.txt")

	tests := []struct {
		name    string
		args    []string
		setup   func() // optional flag setup before calling runCp
		wantErr string // substring expected in error; empty means expect success
	}{
		{
			name:    "default mode valid args",
			args:    []string{srcFile, dstFile},
			wantErr: "",
		},
		{
			name:    "default mode multiple sources",
			args:    []string{srcA, srcB, destDir},
			wantErr: "",
		},
		{
			name: "target-directory mode",
			args: []string{srcA, srcB},
			setup: func() {
				cpFlags.targetDir = destDir
			},
			wantErr: "",
		},
		{
			name:    "fewer than 2 args without -t",
			args:    []string{"only-one"},
			wantErr: "missing destination operand",
		},
		{
			name: "remove-destination and backup mutually exclusive",
			args: []string{srcFile, dstFile},
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
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("runCp() returned error %q, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("runCp() returned nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("runCp() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// resetFlags zeroes cpFlags so tests are independent.
func resetFlags() {
	cli.Timeout = 30 * time.Second
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

func TestDestSemantics(t *testing.T) {
	tmp := t.TempDir()

	// Fixtures: source files.
	srcFile := filepath.Join(tmp, "src.txt")
	if err := os.WriteFile(srcFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	srcA := filepath.Join(tmp, "a.txt")
	if err := os.WriteFile(srcA, []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	srcB := filepath.Join(tmp, "b.txt")
	if err := os.WriteFile(srcB, []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}

	// Fixtures: destination directory.
	destDir := filepath.Join(tmp, "destdir")
	if err := os.Mkdir(destDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Fixtures: destination file (existing).
	destFile := filepath.Join(tmp, "existing.txt")
	if err := os.WriteFile(destFile, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}

	// Non-existent path whose parent exists.
	newDst := filepath.Join(tmp, "newfile.txt")

	// Non-existent path whose parent does NOT exist.
	deepDst := filepath.Join(tmp, "no", "such", "parent", "file.txt")

	// Non-existent directory path (for multi-source).
	missingDir := filepath.Join(tmp, "missing-dir")

	// Non-existent source.
	missingSrc := filepath.Join(tmp, "ghost.txt")

	tests := []struct {
		name    string
		args    []string
		wantErr string // empty means expect success
	}{
		{
			name:    "single source to existing directory",
			args:    []string{srcFile, destDir},
			wantErr: "",
		},
		{
			name:    "single source to non-existent path (parent exists)",
			args:    []string{srcFile, newDst},
			wantErr: "",
		},
		{
			name:    "multi-source to existing directory",
			args:    []string{srcA, srcB, destDir},
			wantErr: "",
		},
		{
			name:    "multi-source to non-existent path",
			args:    []string{srcA, srcB, missingDir},
			wantErr: "no such file or directory",
		},
		{
			name:    "multi-source to existing file",
			args:    []string{srcA, srcB, destFile},
			wantErr: "not a directory",
		},
		{
			name:    "single source to non-existent path (parent doesn't exist)",
			args:    []string{srcFile, deepDst},
			wantErr: "no such file or directory",
		},
		{
			name:    "source doesn't exist",
			args:    []string{missingSrc, destDir},
			wantErr: "no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			err := runCp(nil, tt.args)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("runCp() returned error %q, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("runCp() returned nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("runCp() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestResolvedEndpointIsDir(t *testing.T) {
	tmp := t.TempDir()

	file := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(tmp)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		ep   resolvedEndpoint
		want bool
	}{
		{
			name: "file endpoint",
			ep: resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				localPath: file,
				localInfo: fileInfo,
			},
			want: false,
		},
		{
			name: "dir endpoint",
			ep: resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				localPath: tmp,
				localInfo: dirInfo,
			},
			want: true,
		},
		{
			name: "nil localInfo (non-existent dest)",
			ep: resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				localPath: filepath.Join(tmp, "nope"),
				localInfo: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ep.isDir(); got != tt.want {
				t.Errorf("isDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvedEndpointBasename(t *testing.T) {
	tmp := t.TempDir()

	file := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(tmp, "mydir")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(sub)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		ep   resolvedEndpoint
		want string
	}{
		{
			name: "file endpoint",
			ep: resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				localPath: file,
				localInfo: fileInfo,
			},
			want: "hello.txt",
		},
		{
			name: "dir endpoint",
			ep: resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				localPath: sub,
				localInfo: dirInfo,
			},
			want: "mydir",
		},
		{
			name: "nil localInfo with localPath set",
			ep: resolvedEndpoint{
				pathType:  driveClient.PathLocal,
				localPath: "/some/path/newfile.dat",
				localInfo: nil,
			},
			want: "newfile.dat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ep.basename(); got != tt.want {
				t.Errorf("basename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConflictHandling(t *testing.T) {
	t.Run("default overwrites existing file", func(t *testing.T) {
		resetFlags()
		tmp := t.TempDir()
		src := filepath.Join(tmp, "src.txt")
		dst := filepath.Join(tmp, "dst.txt")
		if err := os.WriteFile(src, []byte("new-data"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte("old-data-longer"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := runCp(nil, []string{src, dst}); err != nil {
			t.Fatalf("runCp: %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new-data" {
			t.Errorf("dst content = %q, want %q", got, "new-data")
		}
	})

	t.Run("--remove-destination removes before copy", func(t *testing.T) {
		resetFlags()
		cpFlags.removeDest = true
		tmp := t.TempDir()
		src := filepath.Join(tmp, "src.txt")
		dst := filepath.Join(tmp, "dst.txt")
		if err := os.WriteFile(src, []byte("new"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte("old-longer-content"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := runCp(nil, []string{src, dst}); err != nil {
			t.Fatalf("runCp: %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new" {
			t.Errorf("dst content = %q, want %q", got, "new")
		}
	})

	t.Run("--backup renames existing to tilde suffix", func(t *testing.T) {
		resetFlags()
		cpFlags.backup = true
		tmp := t.TempDir()
		src := filepath.Join(tmp, "src.txt")
		dst := filepath.Join(tmp, "dst.txt")
		if err := os.WriteFile(src, []byte("new"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte("old"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := runCp(nil, []string{src, dst}); err != nil {
			t.Fatalf("runCp: %v", err)
		}

		// New content at dst.
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new" {
			t.Errorf("dst content = %q, want %q", got, "new")
		}

		// Old content at dst~.
		backup, err := os.ReadFile(dst + "~")
		if err != nil {
			t.Fatalf("backup file missing: %v", err)
		}
		if string(backup) != "old" {
			t.Errorf("backup content = %q, want %q", backup, "old")
		}
	})

	t.Run("copy to non-existent dest (no conflict)", func(t *testing.T) {
		resetFlags()
		tmp := t.TempDir()
		src := filepath.Join(tmp, "src.txt")
		dst := filepath.Join(tmp, "new.txt")
		if err := os.WriteFile(src, []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := runCp(nil, []string{src, dst}); err != nil {
			t.Fatalf("runCp: %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "data" {
			t.Errorf("dst content = %q, want %q", got, "data")
		}
	})

	t.Run("copy into existing directory preserves basename", func(t *testing.T) {
		resetFlags()
		tmp := t.TempDir()
		src := filepath.Join(tmp, "src.txt")
		dstDir := filepath.Join(tmp, "destdir")
		if err := os.WriteFile(src, []byte("hello"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dstDir, 0700); err != nil {
			t.Fatal(err)
		}

		if err := runCp(nil, []string{src, dstDir}); err != nil {
			t.Fatalf("runCp: %v", err)
		}

		got, err := os.ReadFile(filepath.Join(dstDir, "src.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "hello" {
			t.Errorf("dst content = %q, want %q", got, "hello")
		}
	})
}
