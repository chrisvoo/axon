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

// The key is made at every serve and lives in memory. Nothing writes it to the disk.
func TestInitSavesNoKey(t *testing.T) {
	isolatedHome(t)

	out := capture(t, cmdInit)
	if strings.Contains(out, "axon_k_") {
		t.Fatalf("axon init must not show a key:\n%s", out)
	}
	file := readConfigFile(t)
	if strings.Contains(file, "api_key") || strings.Contains(file, "axon_k_") || strings.Contains(file, "operator") {
		t.Fatalf("the config file must hold no key:\n%s", file)
	}
}

func TestDashboardLinkKeepsTheKeyInTheFragment(t *testing.T) {
	got := dashboardLink("https://host:8443/", "axon_k_abc")
	if got != "https://host:8443/#axon_k_abc" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(strings.SplitN(got, "#", 2)[0], "axon_k_") {
		t.Fatal("the key must be after the #, where no server or proxy sees it")
	}
}

// The terminal shows the clickable link with the key, so one click connects.
func TestServerInfoPrintsAClickableLinkWithTheKey(t *testing.T) {
	out := capture(t, func() error {
		printServerInfo("http://127.0.0.1:8443/mcp", "http://127.0.0.1:8443/", "axon_k_abc")
		return nil
	})
	if !strings.Contains(out, "\033]8;;http://127.0.0.1:8443/#axon_k_abc\033\\") {
		t.Fatalf("the Dashboard line must be a hyperlink to the address with the key:\n%q", out)
	}
}
