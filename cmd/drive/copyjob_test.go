package driveCmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCopyJobErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, tmp string) (src, dst *resolvedEndpoint)
		wantErr string
	}{
		{
			name: "same local source and destination",
			setup: func(t *testing.T, tmp string) (*resolvedEndpoint, *resolvedEndpoint) {
				t.Helper()
				f := filepath.Join(tmp, "file.txt")
				if err := os.WriteFile(f, []byte("data"), 0600); err != nil {
					t.Fatal(err)
				}
				info, _ := os.Stat(f)
				ep := &resolvedEndpoint{
					pathType:  PathLocal,
					raw:       f,
					localPath: f,
					localInfo: info,
				}
				return ep, &resolvedEndpoint{
					pathType:  PathLocal,
					raw:       f,
					localPath: f,
					localInfo: info,
				}
			},
			wantErr: "source and destination are the same",
		},
		{
			name: "different local paths succeed",
			setup: func(t *testing.T, tmp string) (*resolvedEndpoint, *resolvedEndpoint) {
				t.Helper()
				src := filepath.Join(tmp, "src.txt")
				dst := filepath.Join(tmp, "dst.txt")
				if err := os.WriteFile(src, []byte("data"), 0600); err != nil {
					t.Fatal(err)
				}
				srcInfo, _ := os.Stat(src)
				return &resolvedEndpoint{
						pathType:  PathLocal,
						raw:       src,
						localPath: src,
						localInfo: srcInfo,
					}, &resolvedEndpoint{
						pathType:  PathLocal,
						raw:       dst,
						localPath: dst,
						localInfo: nil,
					}
			},
			wantErr: "",
		},
		{
			name: "destination parent does not exist",
			setup: func(t *testing.T, tmp string) (*resolvedEndpoint, *resolvedEndpoint) {
				t.Helper()
				src := filepath.Join(tmp, "src.txt")
				if err := os.WriteFile(src, []byte("data"), 0600); err != nil {
					t.Fatal(err)
				}
				srcInfo, _ := os.Stat(src)
				dst := filepath.Join(tmp, "no", "such", "dir", "dst.txt")
				return &resolvedEndpoint{
						pathType:  PathLocal,
						raw:       src,
						localPath: src,
						localInfo: srcInfo,
					}, &resolvedEndpoint{
						pathType:  PathLocal,
						raw:       dst,
						localPath: dst,
						localInfo: nil,
					}
			},
			wantErr: "no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			src, dst := tt.setup(t, tmp)
			ctx := context.Background()

			job, err := buildCopyJob(ctx, nil, src, dst)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if job == nil {
				t.Fatal("expected non-nil job")
			}
			if job.Src == nil {
				t.Error("expected non-nil Src reader")
			}
			if job.Dst == nil {
				t.Error("expected non-nil Dst writer")
			}
		})
	}
}
