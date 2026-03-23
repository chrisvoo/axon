package security

import (
	"os"
	"strings"
)

// Denylist holds blocked command substrings (one per line, trimmed).
type Denylist struct {
	patterns []string
}

// LoadDenylist reads patterns from a file; empty file is valid.
func LoadDenylist(path string) (*Denylist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Denylist{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var p []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p = append(p, line)
	}
	return &Denylist{patterns: p}, nil
}

// Match returns true if command matches any denylisted pattern (substring).
func (d *Denylist) Match(command string) bool {
	for _, pat := range d.patterns {
		if pat != "" && strings.Contains(command, pat) {
			return true
		}
	}
	return false
}
