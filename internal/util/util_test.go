package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirs(t *testing.T) {
	tests := []struct {
		name string
		path string // relative to tmpdir; "" means pass "" directly
		want bool   // true = expect directory to exist after call
	}{
		{
			name: "creates nested dirs",
			path: "a/b/c",
			want: true,
		},
		{
			name: "idempotent",
			path: "a/b/c",
			want: true,
		},
		{
			name: "skips empty path",
			path: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.path == "" {
				if err := EnsureDirs(""); err != nil {
					t.Fatalf("EnsureDirs(\"\") returned error: %v", err)
				}
				return
			}

			tmp := t.TempDir()
			full := filepath.Join(tmp, tt.path)

			// Call once
			if err := EnsureDirs(full); err != nil {
				t.Fatalf("EnsureDirs(%q) returned error: %v", full, err)
			}

			// Call again (idempotent)
			if err := EnsureDirs(full); err != nil {
				t.Fatalf("EnsureDirs(%q) second call returned error: %v", full, err)
			}

			info, err := os.Stat(full)
			if err != nil {
				t.Fatalf("directory %q does not exist after EnsureDirs: %v", full, err)
			}
			if !info.IsDir() {
				t.Fatalf("%q is not a directory", full)
			}
		})
	}
}
