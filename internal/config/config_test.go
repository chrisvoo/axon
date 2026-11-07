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

// A config from a 2.x or 1.x build holds a key (and maybe an operator key). The file
// must still load. The next save must drop all of them: the key lives in memory only.
func TestLoadAcceptsAndSaveDropsLegacyKeys(t *testing.T) {
	isolatedHome(t)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	legacy := "api_key: axon_k_old\noperator_key_hash: sha256:abc\noperator_key: axon_o_old\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("an old config must still load: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"api_key", "axon_k_old", "operator", "axon_o_old"} {
		if strings.Contains(string(data), gone) {
			t.Fatalf("%q is still in the file:\n%s", gone, data)
		}
	}
}
