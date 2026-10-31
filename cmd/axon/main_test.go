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
	k, err := genKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(k, "axon_k_") || len(k) != len("axon_k_")+64 {
		t.Fatalf("a key must start with axon_k_ and hold 256 bits (64 hex characters): %q", k)
	}
	other, _ := genKey()
	if k == other {
		t.Fatal("two keys must differ")
	}
}

func TestEnsureAPIKeyCreatesOnceAndKeepsAnExistingKey(t *testing.T) {
	var cfg config.Config
	if err := ensureAPIKey(&cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.APIKey, "axon_k_") {
		t.Fatalf("unexpected key %q", cfg.APIKey)
	}
	key := cfg.APIKey
	if err := ensureAPIKey(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != key {
		t.Fatal("an existing key must be kept")
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

func apiKeyIn(out string) string {
	for _, f := range strings.Fields(out) {
		if strings.HasPrefix(f, "axon_k_") {
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

func TestInitShowsTheAPIKeyAndKeepsItOnASecondRun(t *testing.T) {
	isolatedHome(t)

	out := capture(t, cmdInit)
	key := apiKeyIn(out)
	if key == "" {
		t.Fatalf("axon init must show the API key:\n%s", out)
	}
	file := readConfigFile(t)
	if !strings.Contains(file, "api_key: "+key) {
		t.Fatalf("the config file must hold the API key:\n%s", file)
	}
	if strings.Contains(file, "operator") {
		t.Fatalf("the config file must not mention an operator key:\n%s", file)
	}

	out = capture(t, cmdInit)
	if apiKeyIn(out) != key {
		t.Fatalf("a second axon init must show the same key:\n%s", out)
	}
	if readConfigFile(t) != file {
		t.Fatal("a second axon init must keep the config")
	}
}

func TestKeygenRotatesTheAPIKey(t *testing.T) {
	isolatedHome(t)
	first := apiKeyIn(capture(t, cmdInit))

	oldArgs := os.Args
	os.Args = []string{"axon", "keygen"}
	defer func() { os.Args = oldArgs }()
	out := capture(t, cmdKeygen)

	key := apiKeyIn(out)
	if key == "" || key == first {
		t.Fatalf("keygen must show a new key, got %q after %q", key, first)
	}
	if !strings.Contains(readConfigFile(t), "api_key: "+key) {
		t.Fatal("the new key must be stored")
	}
}

// The operator key is gone. The flag must fail instead of rotating something else.
func TestKeygenRejectsTheRemovedOperatorFlag(t *testing.T) {
	isolatedHome(t)
	capture(t, cmdInit)

	oldArgs := os.Args
	os.Args = []string{"axon", "keygen", "-operator"}
	defer func() { os.Args = oldArgs }()

	old := os.Stderr
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stderr = devNull
	defer func() { os.Stderr = old; _ = devNull.Close() }()

	if err := cmdKeygen(); err == nil {
		t.Fatal("keygen -operator must fail: the flag no longer exists")
	}
}
