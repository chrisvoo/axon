package tools

import (
	"cmp"
	"slices"
	"strings"
)

const redacted = "[redacted]"

// secretLines splits operator input into the lines the terminal will echo.
// Empty lines are dropped, so an empty secret never redacts anything.
func secretLines(data string) []string {
	return strings.FieldsFunc(data, func(r rune) bool { return r == '\n' || r == '\r' })
}

// redact replaces every occurrence of every secret in s. Longer secrets go
// first, and the replacer scans s once, so inserted text is never rescanned.
func redact(s string, secrets []string) string {
	if len(secrets) == 0 {
		return s
	}
	sorted := slices.Clone(secrets)
	slices.SortFunc(sorted, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	pairs := make([]string, 0, 2*len(sorted))
	for _, secret := range sorted {
		pairs = append(pairs, secret, redacted)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
