//go:build !windows

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chrisvoo/axon/internal/audit"
	"github.com/chrisvoo/axon/internal/config"
	"github.com/chrisvoo/axon/internal/events"
	"github.com/chrisvoo/axon/internal/hub"
	"github.com/chrisvoo/axon/internal/security"
	"github.com/gorilla/websocket"
)

// client opens one connection per request. A shared keep-alive client can leave
// an unused connection open, and http.Server.Shutdown waits 5 s for it.
var client = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

type testServer struct {
	url   string
	key   string
	audit string
}

func startServer(t *testing.T) *testServer {
	t.Helper()
	return startServerWith(t, nil, nil)
}

// startServerWith starts a server on a free port. tune may change the config and the
// options before Run.
func startServerWith(t *testing.T, tuneCfg func(*config.Config), tuneOpts func(*Options)) *testServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	dir := t.TempDir()
	cfg := &config.Config{
		ListenAddr:    "127.0.0.1",
		ListenPort:    port,
		ShellTimeout:  "30s",
		InputStallSec: 1,
		AuditLog:      filepath.Join(dir, "audit.log"),
	}
	if tuneCfg != nil {
		tuneCfg(cfg)
	}
	deny, err := security.LoadDenylist("")
	if err != nil {
		t.Fatal(err)
	}
	log, err := audit.New(cfg.AuditLog)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		opts := Options{Version: "test", APIKey: "the-key", Events: bus, Hub: hub.NewHub(bus, "test"), DevMode: true}
		if tuneOpts != nil {
			tuneOpts(&opts)
		}
		_ = Run(ctx, cfg, deny, log, opts)
	}()
	t.Cleanup(func() { cancel(); <-done })

	ts := &testServer{url: fmt.Sprintf("http://127.0.0.1:%d", port), key: "the-key", audit: cfg.AuditLog}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := client.Get(ts.url + "/health"); err == nil {
			_ = resp.Body.Close()
			return ts
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not start")
	return nil
}

// status sends one request and returns the HTTP status, the body and the headers.
func (ts *testServer) status(t *testing.T, method, path, key, body string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.url+path, bytes.NewBufferString(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out), resp.Header
}

func (ts *testServer) post(t *testing.T, path string, body any) string {
	t.Helper()
	b, _ := json.Marshal(body)
	_, out, _ := ts.status(t, "POST", path, ts.key, string(b))
	return out
}

func (ts *testServer) toolCall(t *testing.T, name string, args map[string]any) string {
	t.Helper()
	return ts.post(t, "/mcp", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
}

// toolResult returns the JSON that a tool call put in its text content.
func toolResult(t *testing.T, rpcReply string) map[string]any {
	t.Helper()
	var outer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(rpcReply), &outer); err != nil || len(outer.Result.Content) == 0 {
		t.Fatalf("unexpected reply: %s", rpcReply)
	}
	var inner map[string]any
	if err := json.Unmarshal([]byte(outer.Result.Content[0].Text), &inner); err != nil {
		t.Fatalf("the tool result is not JSON: %s", rpcReply)
	}
	return inner
}

func (ts *testServer) dialWS(t *testing.T, path string, protocols ...string) (*websocket.Conn, *http.Response) {
	t.Helper()
	d := websocket.Dialer{Subprotocols: protocols, HandshakeTimeout: 2 * time.Second}
	conn, resp, err := d.Dial("ws"+strings.TrimPrefix(ts.url, "http")+path, nil)
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, resp
}

func TestMCPAcceptsOnlyTheKey(t *testing.T) {
	ts := startServer(t)
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	cases := []struct {
		name string
		key  string
		want int
	}{
		{"the key", ts.key, 200},
		{"a wrong key", "not-the-key", 401},
		{"no key", "", 401},
	}
	for _, tc := range cases {
		if got, _, _ := ts.status(t, "POST", "/mcp", tc.key, initialize); got != tc.want {
			t.Errorf("/mcp with %s: got HTTP %d, want %d", tc.name, got, tc.want)
		}
	}
}

// The dashboard only watches. The routes that wrote to it are gone, and no route
// gives out the key.
func TestRemovedDashboardRoutesDoNothing(t *testing.T) {
	ts := startServer(t)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/input"},
		{"POST", "/api/cancel"},
		{"GET", "/api/agent-config"},
	} {
		got, body, _ := ts.status(t, tc.method, tc.path, ts.key, `{"process_id":"nope","data":"x"}`)
		if got == http.StatusOK {
			t.Errorf("%s %s must not exist, got HTTP 200", tc.method, tc.path)
		}
		if strings.Contains(body, ts.key) {
			t.Errorf("%s %s leaks the key", tc.method, tc.path)
		}
	}
}

func TestWebSocketTakesTheKeyFromTheSubprotocol(t *testing.T) {
	ts := startServer(t)
	conn, resp := ts.dialWS(t, "/ws", hub.Subprotocol, hub.KeyProtocolPrefix+ts.key)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil || !strings.Contains(string(msg), `"connected"`) {
		t.Fatalf("expected the connected event, got %q (%v)", msg, err)
	}
	if strings.Contains(strings.Join(resp.Header.Values("Sec-WebSocket-Protocol"), ","), ts.key) {
		t.Fatal("the server must not echo the key")
	}
}

func TestWebSocketClosesOnAWrongOrMovedKey(t *testing.T) {
	ts := startServer(t)
	cases := []struct {
		name      string
		path      string
		protocols []string
	}{
		{"a wrong key", "/ws", []string{hub.Subprotocol, hub.KeyProtocolPrefix + "wrong"}},
		{"no key", "/ws", []string{hub.Subprotocol}},
		{"the key in the URL", "/ws?key=" + ts.key, []string{hub.Subprotocol}},
	}
	for _, tc := range cases {
		conn, _ := ts.dialWS(t, tc.path, tc.protocols...)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if !websocket.IsCloseError(err, hub.CloseUnauthorized) {
			t.Errorf("%s: expected close code %d, got %q and %v", tc.name, hub.CloseUnauthorized, msg, err)
		}
	}
}

// Nothing is hidden from the agent or the audit log any more: what the agent runs
// and what it sees is what it gets.
func TestNothingIsRedacted(t *testing.T) {
	ts := startServer(t)

	reply := ts.toolCall(t, "shell", map[string]any{"command": "echo curl was here"})
	if strings.Contains(reply, "[redacted]") || !strings.Contains(reply, "curl was here") {
		t.Fatalf("output must stay as it is, got %s", reply)
	}
	auditLog, err := os.ReadFile(ts.audit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(auditLog), "echo curl was here") {
		t.Fatalf("the audit log must keep the agent's command: %s", auditLog)
	}
}
