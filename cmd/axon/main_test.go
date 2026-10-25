package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrisvoo/axon/internal/config"
)

func TestGenKey(t *testing.T) {
	a, err := genKey(apiKeyPrefix)
	if err != nil {
		t.Fatal(err)
	}
	o, err := genKey(operatorKeyPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "axon_k_") || !strings.HasPrefix(o, "axon_o_") {
		t.Fatalf("unexpected prefixes: %q, %q", a, o)
	}
	if len(a) != len("axon_k_")+64 || len(o) != len("axon_o_")+64 {
		t.Fatalf("a key must hold 256 bits (64 hex characters): %q, %q", a, o)
	}
	other, _ := genKey(apiKeyPrefix)
	if a == other {
		t.Fatal("two keys must differ")
	}
}

func TestEnsureKeysStoresOnlyAHashAndKeepsExistingKeys(t *testing.T) {
	var cfg config.Config
	opKey, err := ensureKeys(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(opKey, "axon_o_") || !strings.HasPrefix(cfg.APIKey, "axon_k_") {
		t.Fatalf("unexpected keys: %q, %q", opKey, cfg.APIKey)
	}
	if !cfg.OperatorKeyMatches(opKey) || strings.Contains(cfg.OperatorKeyHash, opKey) {
		t.Fatalf("the config must hold a hash of the operator key: %q", cfg.OperatorKeyHash)
	}

	hash, api := cfg.OperatorKeyHash, cfg.APIKey
	again, err := ensureKeys(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if again != "" || cfg.OperatorKeyHash != hash || cfg.APIKey != api {
		t.Fatal("existing keys must be kept, and the operator key is not shown again")
	}
}

func isolatedHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("APPDATA", dir)
}

func capture(t *testing.T, f func() error) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := f()
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatal(runErr)
	}
	return string(out)
}

func operatorKeyIn(out string) string {
	for _, f := range strings.Fields(out) {
		if strings.HasPrefix(f, "axon_o_") {
			return f
		}
	}
	return ""
}

func readConfigFile(t *testing.T) string {
	t.Helper()
	path, err := config.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitShowsTheOperatorKeyOnceAndStoresOnlyItsHash(t *testing.T) {
	isolatedHome(t)

	out := capture(t, cmdInit)
	key := operatorKeyIn(out)
	if key == "" {
		t.Fatalf("axon init must show the operator key once:\n%s", out)
	}
	file := readConfigFile(t)
	if strings.Contains(file, key) {
		t.Fatalf("the operator key is in the config file:\n%s", file)
	}
	if !strings.Contains(file, "operator_key_hash: sha256:") {
		t.Fatalf("the config file must hold the hash:\n%s", file)
	}

	out = capture(t, cmdInit)
	if operatorKeyIn(out) != "" {
		t.Fatalf("a second axon init must not show an operator key:\n%s", out)
	}
	if readConfigFile(t) != file {
		t.Fatal("a second axon init must keep the keys")
	}
}

func TestKeygenOperatorShowsTheNewKeyOnceAndStoresOnlyItsHash(t *testing.T) {
	isolatedHome(t)
	capture(t, cmdInit)
	before := readConfigFile(t)

	oldArgs := os.Args
	os.Args = []string{"axon", "keygen", "-operator"}
	defer func() { os.Args = oldArgs }()
	out := capture(t, cmdKeygen)

	key := operatorKeyIn(out)
	if key == "" {
		t.Fatalf("keygen -operator must show the new key:\n%s", out)
	}
	after := readConfigFile(t)
	if after == before || strings.Contains(after, key) {
		t.Fatalf("the hash must change, and the key must not be stored:\n%s", after)
	}
}
