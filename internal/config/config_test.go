package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testOperatorKey = "axon_o_0123456789abcdef"

func TestHashKey(t *testing.T) {
	h := HashKey(testOperatorKey)
	if !strings.HasPrefix(h, "sha256:") || len(h) != len("sha256:")+64 {
		t.Fatalf("unexpected hash %q", h)
	}
	if h != HashKey(testOperatorKey) || h == HashKey(testOperatorKey+"x") {
		t.Fatal("the hash must be stable and depend on the key")
	}
	if strings.Contains(h, testOperatorKey) {
		t.Fatal("the hash must not contain the key")
	}
}

func TestOperatorKeyMatches(t *testing.T) {
	cfg := Config{OperatorKeyHash: HashKey(testOperatorKey)}
	cases := []struct {
		name  string
		token string
		want  bool
	}{
		{"the key", testOperatorKey, true},
		{"another key", "axon_o_other", false},
		{"empty", "", false},
		// The agent can read the stored hash. It must not work as a key.
		{"the stored hash", cfg.OperatorKeyHash, false},
	}
	for _, tc := range cases {
		if got := cfg.OperatorKeyMatches(tc.token); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if (&Config{}).OperatorKeyMatches("anything") {
		t.Error("a config without an operator key must accept no token")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"both keys", Config{APIKey: "a", OperatorKeyHash: HashKey("o")}, ""},
		{"no API key", Config{OperatorKeyHash: HashKey("o")}, "missing API key"},
		{"no operator key", Config{APIKey: "a"}, "missing operator key"},
		{"same key", Config{APIKey: "k", OperatorKeyHash: HashKey("k")}, "must be different"},
	}
	for _, tc := range cases {
		err := tc.cfg.Validate()
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tc.name, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestSaveStoresOnlyTheOperatorKeyHash(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("APPDATA", dir)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.APIKey, cfg.OperatorKeyHash = "axon_k_test", HashKey(testOperatorKey)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), testOperatorKey) {
		t.Fatalf("the operator key must not be written to the config file:\n%s", data)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "axon_k_test" || !got.OperatorKeyMatches(testOperatorKey) {
		t.Fatal("the keys were not kept")
	}
}

// A config from a build that stored the key in clear text must not keep it.
func TestSaveDropsALegacyPlaintextOperatorKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("APPDATA", dir)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	legacy := "api_key: axon_k_test\noperator_key: " + testOperatorKey + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("a legacy config has no operator key hash and must not validate")
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), testOperatorKey) {
		t.Fatalf("the legacy clear-text key is still in the file:\n%s", data)
	}
}
