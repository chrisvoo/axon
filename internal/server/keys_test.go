//go:build !windows

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/chrisvoo/axon/internal/config"
)

func (ts *testServer) status(t *testing.T, method, path, key string, header http.Header, body string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.url+path, bytes.NewBufferString(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(resp.Body)
	return resp.StatusCode, out.String(), resp.Header
}

func TestEachRouteAcceptsOnlyItsOwnKey(t *testing.T) {
	ts := startServer(t)
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	input := `{"process_id":"nope","data":"x"}`
	upgrade := http.Header{
		"Connection":            {"Upgrade"},
		"Upgrade":               {"websocket"},
		"Sec-Websocket-Version": {"13"},
		"Sec-Websocket-Key":     {"dGhlIHNhbXBsZSBub25jZQ=="},
	}

	cases := []struct {
		name         string
		method, path string
		key          string
		header       http.Header
		body         string
		want         int
	}{
		{"/mcp with the agent key", "POST", "/mcp", ts.agentKey, nil, initialize, 200},
		{"/mcp with the operator key", "POST", "/mcp", ts.opKey, nil, initialize, 401},
		{"/mcp without a key", "POST", "/mcp", "", nil, initialize, 401},
		{"/api/input with the agent key", "POST", "/api/input", ts.agentKey, nil, input, 401},
		{"/api/input with the operator key", "POST", "/api/input", ts.opKey, nil, input, 400}, // passes auth, process unknown
		{"/api/cancel with the agent key", "POST", "/api/cancel", ts.agentKey, nil, `{"process_id":"nope"}`, 401},
		{"/api/cancel with the operator key", "POST", "/api/cancel", ts.opKey, nil, `{"process_id":"nope"}`, 200},
		{"/api/agent-config with the agent key", "GET", "/api/agent-config", ts.agentKey, nil, "", 401},
		{"/ws with the agent key", "GET", "/ws?key=" + ts.agentKey, "", upgrade, "", 401},
	}
	for _, tc := range cases {
		got, _, _ := ts.status(t, tc.method, tc.path, tc.key, tc.header, tc.body)
		if got != tc.want {
			t.Errorf("%s: got HTTP %d, want %d", tc.name, got, tc.want)
		}
	}
}

// The config file holds a hash. The agent can read it, so it must not work as a key.
func TestStoredHashIsNotAKey(t *testing.T) {
	ts := startServer(t)
	hash := config.HashKey(ts.opKey)
	for _, path := range []string{"/api/input", "/api/cancel"} {
		got, _, _ := ts.status(t, "POST", path, hash, nil, `{"process_id":"nope"}`)
		if got != http.StatusUnauthorized {
			t.Errorf("%s with the stored hash: got HTTP %d, want 401", path, got)
		}
	}
	if got, _, _ := ts.status(t, "GET", "/api/agent-config", hash, nil, ""); got != http.StatusUnauthorized {
		t.Errorf("/api/agent-config with the stored hash: got HTTP %d, want 401", got)
	}
	if got, _, _ := ts.status(t, "GET", "/ws?key="+hash, "", nil, ""); got != http.StatusUnauthorized {
		t.Errorf("/ws with the stored hash: got HTTP %d, want 401", got)
	}
}

func TestOperatorKeyOpensTheWebSocket(t *testing.T) {
	ts := startServer(t)
	// A plain GET with the operator key passes the key check. The upgrade headers
	// are missing, so the server answers 400 instead of 401.
	got, _, _ := ts.status(t, "GET", "/ws?key="+ts.opKey, "", nil, "")
	if got == http.StatusUnauthorized {
		t.Fatalf("the operator key must be accepted on /ws, got %d", got)
	}
}

func TestAgentConfigGoesToTheOperatorOnly(t *testing.T) {
	ts := startServer(t)
	code, body, header := ts.status(t, "GET", "/api/agent-config", ts.opKey, nil, "")
	if code != 200 {
		t.Fatalf("got HTTP %d", code)
	}
	var cfg struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal([]byte(body), &cfg); err != nil || cfg.APIKey != ts.agentKey {
		t.Fatalf("unexpected body %q (%v)", body, err)
	}
	if !strings.Contains(header.Get("Cache-Control"), "no-store") {
		t.Fatalf("the key must not be cached, Cache-Control = %q", header.Get("Cache-Control"))
	}
	if strings.Contains(body, ts.opKey) {
		t.Fatal("the reply must not contain the operator key")
	}
}

// The agent holds only the API key. It must not be able to hide words of its
// choice from the audit log and the operator's events.
func TestAgentCannotRegisterSecrets(t *testing.T) {
	ts := startServer(t)

	code, _, _ := ts.status(t, "POST", "/api/input", ts.agentKey, nil, `{"process_id":"nope","data":"curl"}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("the agent key must not reach /api/input, got %d", code)
	}

	reply := ts.toolCall(t, "shell", map[string]any{"command": "echo curl was here"})
	if strings.Contains(reply, "[redacted]") || !strings.Contains(reply, "curl was here") {
		t.Fatalf("the agent must not be able to hide a word, got %s", reply)
	}
	auditLog, err := os.ReadFile(ts.audit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(auditLog), "echo curl was here") {
		t.Fatalf("the audit log must keep the agent's command: %s", auditLog)
	}
}

// An empty process id must not reach the secret store either.
func TestOperatorInputWithEmptyProcessIDIsRejected(t *testing.T) {
	ts := startServer(t)
	code, _, _ := ts.status(t, "POST", "/api/input", ts.opKey, nil, `{"process_id":"","data":"curl\n"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("got HTTP %d, want 400", code)
	}
	reply := ts.toolCall(t, "shell", map[string]any{"command": "echo curl was here"})
	if strings.Contains(reply, "[redacted]") {
		t.Fatalf("an empty process id must not register a secret, got %s", reply)
	}
}

// Even the operator route registers nothing for a process that does not exist.
func TestOperatorInputForUnknownProcessRegistersNothing(t *testing.T) {
	ts := startServer(t)
	ts.post(t, "/api/input", map[string]any{"process_id": "nope", "data": "curl\n"})

	reply := ts.toolCall(t, "shell", map[string]any{"command": "echo curl was here"})
	if strings.Contains(reply, "[redacted]") {
		t.Fatalf("an unknown process must not register a secret, got %s", reply)
	}
}
