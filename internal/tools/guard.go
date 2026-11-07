package tools

import (
	"path/filepath"
	"strings"
)

// resolve returns the absolute path of p with symbolic links followed. A path that
// does not exist yet (a new file for write_file) resolves through its nearest
// existing parent, so a link in the parent cannot hide the target.
func resolve(p string) string {
	p, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	rest := ""
	for {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Join(p, rest)
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// InsideAny reports whether target is one of the protected paths or inside one.
// An empty protected entry is skipped.
//
// This is a guardrail for the file tools, not a boundary: the shell tool can reach
// the same files.
func InsideAny(target string, protected ...string) bool {
	t := resolve(target)
	for _, p := range protected {
		if p == "" {
			continue
		}
		rel, err := filepath.Rel(resolve(p), t)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
