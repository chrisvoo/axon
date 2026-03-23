package server

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestCursorMCPInstallDeeplink(t *testing.T) {
	t.Parallel()
	link, err := CursorMCPInstallDeeplink("axon", "https://127.0.0.1:8443/mcp", "axon_k_test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "cursor://anysphere.cursor-deeplink/mcp/install?") {
		t.Fatalf("unexpected prefix: %s", link)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "cursor" || u.Host != "anysphere.cursor-deeplink" || u.Path != "/mcp/install" {
		t.Fatalf("unexpected URL: %#v", u)
	}
	q := u.Query()
	if q.Get("name") != "axon" {
		t.Fatalf("name: got %q", q.Get("name"))
	}
	raw, err := base64.StdEncoding.DecodeString(q.Get("config"))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	axon, ok := payload["axon"].(map[string]any)
	if !ok {
		t.Fatalf("payload axon: %#v", payload["axon"])
	}
	if axon["url"] != "https://127.0.0.1:8443/mcp" {
		t.Fatalf("url: %#v", axon["url"])
	}
	h, ok := axon["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers: %#v", axon["headers"])
	}
	if h["Authorization"] != "Bearer axon_k_test" {
		t.Fatalf("Authorization: %#v", h["Authorization"])
	}
}

func TestMCPHTTPSURL(t *testing.T) {
	t.Parallel()
	if got := MCPHTTPSURL("0.0.0.0", 8443); got != "https://127.0.0.1:8443/mcp" {
		t.Fatalf("got %q", got)
	}
	if got := MCPHTTPSURL("10.0.0.5", 9000); got != "https://10.0.0.5:9000/mcp" {
		t.Fatalf("got %q", got)
	}
}
