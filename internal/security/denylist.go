package security

import (
	"os"
	"strings"
)

// Denylist holds blocked command substrings (one per line, trimmed).
type Denylist struct {
	patterns []string
}

// normalizeWS collapses every run of whitespace to a single space and trims.
// This prevents bypasses like "rm    -rf   /" evading the pattern "rm -rf /".
func normalizeWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
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
		line = normalizeWS(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p = append(p, line)
	}
	return &Denylist{patterns: p}, nil
}

// Match returns true if command matches any denylisted pattern (substring).
// Whitespace in both the command and the pattern is normalized before comparison
// so that extra spaces between tokens cannot be used to evade a rule.
func (d *Denylist) Match(command string) bool {
	normalized := normalizeWS(command)
	for _, pat := range d.patterns {
		if pat != "" && strings.Contains(normalized, pat) {
			return true
		}
	}
	return false
}
