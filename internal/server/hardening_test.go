//go:build !windows

package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chrisvoo/axon/internal/config"
	"github.com/gorilla/websocket"
)

const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`

// send posts to /mcp with extra headers and returns the HTTP status.
func (ts *testServer) send(t *testing.T, key string, header map[string]string, body string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.url+"/mcp", strings.NewReader(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func (ts *testServer) auditLines(t *testing.T, action string) int {
	t.Helper()
	data, err := os.ReadFile(ts.audit)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), `"action":"`+action+`"`)
}

func TestRunRefusesToStartWithoutAKey(t *testing.T) {
	err := Run(context.Background(), &config.Config{}, nil, nil, Options{})
	if err == nil || !strings.Contains(err.Error(), "no API key") {
		t.Fatalf("Run must refuse an empty key, got %v", err)
	}
}

func TestWrongKeysAreAuditedThenTheAddressIsBlocked(t *testing.T) {
	ts := startServer(t)
	for i := 1; i <= 5; i++ {
		if got := ts.send(t, "wrong", nil, initialize); got != 401 {
			t.Fatalf("wrong key %d: got HTTP %d, want 401", i, got)
		}
	}
	if n := ts.auditLines(t, "auth_fail"); n != 5 {
		t.Fatalf("want 5 auth_fail lines, got %d", n)
	}
	// Blocked: even the right key waits, and the request leaves no line.
	if got := ts.send(t, ts.key, nil, initialize); got != 429 {
		t.Fatalf("a blocked address must get 429, got %d", got)
	}
	ts.send(t, "wrong", nil, initialize)
	if n := ts.auditLines(t, "auth_fail"); n != 5 {
		t.Fatalf("a blocked request must not add a line, got %d lines", n)
	}
}

func TestTheRightKeyClearsTheFailures(t *testing.T) {
	ts := startServer(t)
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			ts.send(t, "wrong", nil, initialize)
		}
		if got := ts.send(t, ts.key, nil, initialize); got != 200 {
			t.Fatalf("round %d: the right key must pass, got %d", round, got)
		}
	}
}

func TestTunnelModeReadsTheRealClientAddress(t *testing.T) {
	allow := func(c *config.Config) { c.IPAllowlist = []string{"203.0.113.5"} }
	tunnel := startServerWith(t, allow, func(o *Options) { o.BehindTunnel = true })
	plain := startServerWith(t, allow, nil)

	cases := []struct {
		name   string
		ts     *testServer
		header map[string]string
		want   int
	}{
		{"tunnel, allowed client", tunnel, map[string]string{"CF-Connecting-IP": "203.0.113.5"}, 200},
		{"tunnel, other client", tunnel, map[string]string{"CF-Connecting-IP": "198.51.100.1"}, 403},
		{"tunnel, no header", tunnel, nil, 403},
		{"no tunnel: a spoofed header does nothing", plain, map[string]string{"CF-Connecting-IP": "203.0.113.5"}, 403},
	}
	for _, tc := range cases {
		if got := tc.ts.send(t, tc.ts.key, tc.header, initialize); got != tc.want {
			t.Errorf("%s: got HTTP %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestTunnelClientsHaveTheirOwnRateLimit(t *testing.T) {
	ts := startServerWith(t, func(c *config.Config) { c.RateLimitRPS = 1 }, func(o *Options) { o.BehindTunnel = true })
	hit := func(ip string) int {
		return ts.send(t, ts.key, map[string]string{"CF-Connecting-IP": ip}, initialize)
	}
	hit("203.0.113.1")
	hit("203.0.113.1")
	if got := hit("203.0.113.1"); got != 429 {
		t.Fatalf("client 1 must hit its limit, got %d", got)
	}
	if got := hit("203.0.113.2"); got != 200 {
		t.Fatalf("client 2 must not share the limit of client 1, got %d", got)
	}
}

func TestMCPBodyIsCapped(t *testing.T) {
	ts := startServer(t)
	big := `{"jsonrpc":"2.0","id":1,"method":"initialize","pad":"` + strings.Repeat("a", maxBodyBytes+1024) + `"}`
	_, out, _ := ts.status(t, "POST", "/mcp", ts.key, big)
	if !strings.Contains(out, "Parse error") {
		t.Fatalf("a body above the cap must fail, got %.200s", out)
	}
	if _, out, _ := ts.status(t, "POST", "/mcp", ts.key, initialize); !strings.Contains(out, "protocolVersion") {
		t.Fatalf("a small body must work, got %s", out)
	}
}

func TestSecurityHeaders(t *testing.T) {
	ts := startServer(t)
	_, _, h := ts.status(t, "GET", "/", "", "")
	csp := h.Get("Content-Security-Policy")
	for _, want := range []string{"connect-src 'self'", "frame-ancestors 'none'", "script-src 'self'", "https://fonts.googleapis.com"} {
		if !strings.Contains(csp, want) {
			t.Errorf("the CSP lacks %q: %s", want, csp)
		}
	}
	if strings.Contains(csp, "wss:") || strings.Contains(csp, "ws:") {
		t.Errorf("connect-src must not allow other hosts: %s", csp)
	}
	for k, v := range map[string]string{"X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"} {
		if got := h.Get(k); got != v {
			t.Errorf("%s: got %q, want %q", k, got, v)
		}
	}
}

func TestWebSocketRejectsAForeignOrigin(t *testing.T) {
	ts := startServer(t)
	dial := func(origin string) (*http.Response, error) {
		d := websocket.Dialer{
			Subprotocols:     []string{"axon.v1", "axon.key." + ts.key},
			HandshakeTimeout: 2 * time.Second,
		}
		conn, resp, err := d.Dial("ws"+strings.TrimPrefix(ts.url, "http")+"/ws", http.Header{"Origin": {origin}})
		if conn != nil {
			_ = conn.Close()
		}
		return resp, err
	}
	if _, err := dial(ts.url); err != nil {
		t.Fatalf("the dashboard's own origin must work: %v", err)
	}
	resp, err := dial("https://evil.example")
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a foreign origin must get 403, got %v (%v)", resp, err)
	}
}

func TestFileToolsRefuseTheAxonFiles(t *testing.T) {
	ts := startServer(t)
	ts.toolCall(t, "system_info", nil) // puts a line in the audit log
	dir := filepath.Dir(ts.audit)
	notes := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notes, []byte("hello audit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(ts.audit)

	refused := func(reply string) bool {
		return strings.Contains(reply, `"isError":true`) && strings.Contains(reply, "protected")
	}
	if r := ts.toolCall(t, "read_file", map[string]any{"path": ts.audit}); !refused(r) {
		t.Errorf("read_file on the audit log must be refused: %s", r)
	}
	if r := ts.toolCall(t, "write_file", map[string]any{"path": ts.audit, "content": ""}); !refused(r) {
		t.Errorf("write_file on the audit log must be refused: %s", r)
	}
	if r := ts.toolCall(t, "edit_file", map[string]any{"path": ts.audit, "old_string": "system_info", "new_string": "x"}); !refused(r) {
		t.Errorf("edit_file on the audit log must be refused: %s", r)
	}
	if after, _ := os.ReadFile(ts.audit); !strings.HasPrefix(string(after), string(before)) {
		t.Error("the audit log was changed")
	}

	// A search over the folder finds the notes and never the audit log.
	grep := ts.toolCall(t, "grep", map[string]any{"root": dir, "pattern": "audit"})
	if !strings.Contains(grep, "notes.txt") || strings.Contains(grep, "audit.log") {
		t.Errorf("grep must hit notes.txt only: %s", grep)
	}
	glob := ts.toolCall(t, "glob", map[string]any{"root": dir, "pattern": "*"})
	if !strings.Contains(glob, "notes.txt") || strings.Contains(glob, "audit.log") {
		t.Errorf("glob must list notes.txt only: %s", glob)
	}
	if r := ts.toolCall(t, "read_file", map[string]any{"path": notes}); refused(r) || !strings.Contains(r, "hello audit") {
		t.Errorf("an ordinary file must stay readable: %s", r)
	}
}
