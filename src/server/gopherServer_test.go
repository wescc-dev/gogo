package server

import (
	"path/filepath"
	"testing"
)

func TestResolveSelectorPath(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name     string
		selector string
		want     string
		wantErr  bool
	}{
		{name: "root", selector: "", want: "."},
		{name: "leading slash", selector: "/docs/readme.txt", want: filepath.Join("docs", "readme.txt")},
		{name: "normalizes within root", selector: "docs/../readme.txt", want: "readme.txt"},
		{name: "parent traversal", selector: "../secret.txt", wantErr: true},
		{name: "deep parent traversal", selector: "docs/../../../secret.txt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, resolvedPath, err := resolveSelectorPath(root, tt.selector)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveSelectorPath(%q) unexpectedly succeeded with %q", tt.selector, resolvedPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSelectorPath(%q): %v", tt.selector, err)
			}
			if selector != tt.want {
				t.Errorf("selector = %q, want %q", selector, tt.want)
			}
			if resolvedPath != filepath.Join(root, tt.want) {
				t.Errorf("resolved path = %q, want path beneath %q", resolvedPath, root)
			}
			if filepath.IsAbs(selector) {
				t.Errorf("selector exposed an absolute filesystem path: %q", selector)
			}
		})
	}
}
