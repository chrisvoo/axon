package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolatedHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("APPDATA", dir)
}

func TestValidate(t *testing.T) {
	if err := (&Config{APIKey: "a"}).Validate(); err != nil {
		t.Errorf("a config with an API key must validate, got %v", err)
	}
	err := (&Config{}).Validate()
	if err == nil || !strings.Contains(err.Error(), "missing API key") {
		t.Errorf("a config without an API key must fail, got %v", err)
	}
}

func TestSaveAndLoadKeepTheAPIKey(t *testing.T) {
	isolatedHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.APIKey = "axon_k_test"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "axon_k_test" {
		t.Fatalf("the key was not kept: %q", got.APIKey)
	}
}

// A config from a 1.x build holds an operator key (a hash, or an older clear-text
// key). The file must still load, and the next save must drop those fields.
func TestLoadAcceptsAndSaveDropsLegacyOperatorKeys(t *testing.T) {
	isolatedHome(t)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	legacy := "api_key: axon_k_test\noperator_key_hash: sha256:abc\noperator_key: axon_o_old\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("a 1.x config must still load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a 1.x config with an API key must validate: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "operator") || strings.Contains(string(data), "axon_o_old") {
		t.Fatalf("the legacy operator fields are still in the file:\n%s", data)
	}
}
