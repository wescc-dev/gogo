package server

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestResolveSelectorPathWithRelativeRootDoesNotLeakRoot(t *testing.T) {
	workingDir := t.TempDir()
	root := filepath.Join(workingDir, "gopher-root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workingDir)

	selector, resolvedPath, err := resolveSelectorPath("gopher-root", "/Ab")
	if err != nil {
		t.Fatal(err)
	}
	if selector != "Ab" {
		t.Errorf("selector = %q, want %q", selector, "Ab")
	}
	if filepath.IsAbs(selector) || selector == resolvedPath {
		t.Errorf("client selector exposed filesystem path %q", selector)
	}
	if resolvedPath != filepath.Join(root, "Ab") {
		t.Errorf("resolved path = %q, want %q", resolvedPath, filepath.Join(root, "Ab"))
	}
}

func TestResolveSelectorPathAllowsSymlinkOutsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require additional privileges on Windows")
	}

	parent := t.TempDir()
	root := filepath.Join(parent, "gopher-root")
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "external")); err != nil {
		t.Fatal(err)
	}

	selector, resolvedPath, err := resolveSelectorPath(root, "external")
	if err != nil {
		t.Fatalf("symlink selector was rejected: %v", err)
	}
	if selector != "external" {
		t.Errorf("selector = %q, want %q", selector, "external")
	}
	resolvedSymlink, err := filepath.EvalSymlinks(resolvedPath)
	if err != nil {
		t.Fatal(err)
	}
	canonicalOutside, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedSymlink != canonicalOutside {
		t.Errorf("symlink resolved to %q, want %q", resolvedSymlink, canonicalOutside)
	}
}
