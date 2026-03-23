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
