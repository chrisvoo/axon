package tools

import (
	"slices"
	"testing"
)

func TestSecretLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"hunter2\n", []string{"hunter2"}},
		{"hunter2", []string{"hunter2"}},
		{"a b\r\nsecond\n", []string{"a b", "second"}},
		{"\n\r\n", nil},
		{"   \n\t\n", nil},
		{"", nil},
	}
	for _, tc := range cases {
		if got := secretLines(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("secretLines(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRedact(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		secrets []string
		want    string
	}{
		{"no secrets", "plain output", nil, "plain output"},
		{"terminal echo", "Password: hunter2\r\ndone\r\n", []string{"hunter2"}, "Password: [redacted]\r\ndone\r\n"},
		{"every occurrence", "hunter2 and hunter2", []string{"hunter2"}, "[redacted] and [redacted]"},
		{"several secrets", "a=one b=two", []string{"one", "two"}, "a=[redacted] b=[redacted]"},
		{"longest first", "pass12345", []string{"pass", "pass12345"}, "[redacted]"},
		{"short secret", "y", []string{"y"}, "[redacted]"},
		{"secret with spaces", "my pass phrase!", []string{"my pass phrase!"}, "[redacted]"},
		{"not present", "nothing here", []string{"hunter2"}, "nothing here"},
	}
	for _, tc := range cases {
		if got := redact(tc.in, tc.secrets); got != tc.want {
			t.Errorf("%s: redact(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}
