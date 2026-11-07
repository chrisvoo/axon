//go:build !windows

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInsideAny(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".axon")
	other := filepath.Join(root, "project")
	for _, d := range []string{cfgDir, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A link inside the project that points into the config folder.
	link := filepath.Join(other, "sneaky")
	if err := os.Symlink(cfgDir, link); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		target string
		want   bool
	}{
		{"a file in the folder", filepath.Join(cfgDir, "audit.log"), true},
		{"the folder itself", cfgDir, true},
		{"a new file in the folder", filepath.Join(cfgDir, "new", "file"), true},
		{"a dot-dot path back into it", filepath.Join(other, "..", ".axon", "denylist.txt"), true},
		{"a link to the folder", filepath.Join(link, "config.yaml"), true},
		{"a new file under a link to the folder", filepath.Join(link, "new.txt"), true},
		{"a sibling folder with the same prefix", filepath.Join(root, ".axon-other", "x"), false},
		{"a project file", filepath.Join(other, "main.go"), false},
		{"the parent of the folder", root, false},
	}
	for _, tc := range cases {
		if got := InsideAny(tc.target, cfgDir); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if InsideAny(filepath.Join(cfgDir, "x"), "") {
		t.Error("an empty protected entry must protect nothing")
	}
	file := filepath.Join(other, "audit.log")
	if !InsideAny(file, file) || InsideAny(filepath.Join(other, "main.go"), file) {
		t.Error("a protected file protects itself and nothing else")
	}
}
