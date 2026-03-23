package security

import "testing"

func TestDenylist_Match(t *testing.T) {
	d := &Denylist{patterns: []string{"rm -rf /", "mkfs"}}
	if !d.Match("sudo rm -rf /") {
		t.Fatal("expected match")
	}
	if d.Match("echo hello") {
		t.Fatal("expected no match")
	}
}

func TestDenylist_Match_ExtraWhitespace(t *testing.T) {
	d := &Denylist{patterns: []string{"rm -rf /", "curl | sh"}}

	cases := []struct {
		cmd   string
		match bool
	}{
		// Extra spaces between tokens must still match.
		{"rm    -rf   /", true},
		// Tabs between tokens.
		{"rm\t-rf\t/", true},
		// Leading/trailing whitespace around the dangerous part.
		{"  rm -rf /  ", true},
		// Pattern itself loaded with extra spaces normalises correctly.
		{"curl | sh", true},
		// Safe commands must not be blocked.
		{"ls -la", false},
		{"echo hello world", false},
	}

	for _, tc := range cases {
		got := d.Match(tc.cmd)
		if got != tc.match {
			t.Errorf("Match(%q) = %v, want %v", tc.cmd, got, tc.match)
		}
	}
}

func TestDenylist_LoadNormalizesPatterns(t *testing.T) {
	// Patterns with extra internal spaces must still block the canonical form.
	d := &Denylist{patterns: []string{normalizeWS("rm  -rf  /")}}
	if !d.Match("rm -rf /") {
		t.Fatal("normalised pattern should match canonical command")
	}
}
