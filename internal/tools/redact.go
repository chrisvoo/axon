package tools

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

const redacted = "[redacted]"

// secretLines splits operator input into the lines the terminal will echo.
// Empty and whitespace-only lines are dropped: they would redact ordinary text.
func secretLines(data string) []string {
	lines := strings.FieldsFunc(data, func(r rune) bool { return r == '\n' || r == '\r' })
	return slices.DeleteFunc(lines, func(l string) bool { return strings.TrimSpace(l) == "" })
}

// jsonForm returns s as it appears inside a JSON string. Output is often
// encoded after it is built, and quotes, backslashes and angle brackets change.
func jsonForm(s string) string {
	b, err := json.Marshal(s)
	if err != nil || len(b) < 2 {
		return s
	}
	return string(b[1 : len(b)-1])
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
