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
)

const secretText = "hunter2-Zq9"

type testServer struct {
	url      string
	agentKey string
	opKey    string
	audit    string
	events   <-chan events.Event
}

func startServer(t *testing.T) *testServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	dir := t.TempDir()
	cfg := &config.Config{
		ListenAddr:      "127.0.0.1",
		ListenPort:      port,
		APIKey:          "agent-key",
		OperatorKeyHash: config.HashKey("operator-key"),
		ShellTimeout:    "30s",
		InputStallSec:   1,
		AuditLog:        filepath.Join(dir, "audit.log"),
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
	sub := bus.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Run(ctx, cfg, deny, log, Options{Version: "test", Events: bus, Hub: hub.NewHub(bus, "test"), DevMode: true})
	}()
	t.Cleanup(func() { cancel(); <-done })

	ts := &testServer{url: fmt.Sprintf("http://127.0.0.1:%d", port), agentKey: cfg.APIKey, opKey: "operator-key", audit: cfg.AuditLog, events: sub}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(ts.url + "/health"); err == nil {
			_ = resp.Body.Close()
			return ts
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not start")
	return nil
}

func (ts *testServer) post(t *testing.T, path string, body any) string {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", ts.url+path, bytes.NewReader(b))
	key := ts.agentKey
	if strings.HasPrefix(path, "/api/") { // the dashboard routes need the operator key
		key = ts.opKey
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return string(out)
}

func (ts *testServer) toolCall(t *testing.T, name string, args map[string]any) string {
	t.Helper()
	return ts.post(t, "/mcp", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
}

func processID(t *testing.T, rpcReply string) string {
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
	var inner struct {
		ProcessID string `json:"process_id"`
	}
	if err := json.Unmarshal([]byte(outer.Result.Content[0].Text), &inner); err != nil || inner.ProcessID == "" {
		t.Fatalf("expected input_required with a process id, got %s", rpcReply)
	}
	return inner.ProcessID
}

// The operator types a password in the dashboard. It must not appear in the
// agent's view, the dashboard events, the audit log or any HTTP reply.
func TestOperatorSecretNeverLeavesTheHost(t *testing.T) {
	ts := startServer(t)

	reply := ts.toolCall(t, "shell", map[string]any{"command": "read x; echo got:$x; sleep 20"})
	pid := processID(t, reply)
	defer ts.toolCall(t, "cancel_command", map[string]any{"process_id": pid})

	everything := []string{reply}
	everything = append(everything, ts.post(t, "/api/input", map[string]any{"process_id": pid, "data": secretText + "\n"}))

	deadline := time.Now().Add(5 * time.Second)
	sawRedacted := false
	for time.Now().Before(deadline) && !sawRedacted {
		view := ts.toolCall(t, "send_input", map[string]any{"process_id": pid, "data": ""}) // the agent
		everything = append(everything, view)
		sawRedacted = strings.Contains(view, "got:[redacted]")
		time.Sleep(100 * time.Millisecond)
	}
	if !sawRedacted {
		t.Fatalf("expected got:[redacted] in the agent's view, last: %s", everything[len(everything)-1])
	}

	for {
		select {
		case e := <-ts.events:
			b, _ := json.Marshal(e)
			everything = append(everything, string(b))
			continue
		default:
		}
		break
	}
	auditLog, err := os.ReadFile(ts.audit)
	if err != nil {
		t.Fatal(err)
	}
	everything = append(everything, string(auditLog))

	for i, text := range everything {
		if strings.Contains(text, secretText) {
			t.Fatalf("the secret leaked (item %d): %s", i, text)
		}
	}
}

// A plain response (the dashboard sends secret:false) stays visible.
func TestOperatorPlainResponseStaysVisible(t *testing.T) {
	ts := startServer(t)

	pid := processID(t, ts.toolCall(t, "shell", map[string]any{"command": "read x; echo got:$x; sleep 20"}))
	defer ts.toolCall(t, "cancel_command", map[string]any{"process_id": pid})
	ts.post(t, "/api/input", map[string]any{"process_id": pid, "data": "yes\n", "secret": false})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if view := ts.toolCall(t, "send_input", map[string]any{"process_id": pid, "data": ""}); strings.Contains(view, "got:yes") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("a plain response should reach the agent")
}

// A secret typed into one command must be hidden in every other channel the
// agent can read, including JSON-encoded output, errors, events and the audit log.
func TestSecretIsHiddenInEveryAgentVisibleChannel(t *testing.T) {
	const secret = `hu"nt\er2<Zq9`
	ts := startServer(t)
	dir := t.TempDir()
	saved := filepath.Join(dir, "saved.txt")
	if err := os.WriteFile(saved, []byte("password="+secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	pid := processID(t, ts.toolCall(t, "shell", map[string]any{"command": "read -r x; sleep 20"}))
	defer ts.toolCall(t, "cancel_command", map[string]any{"process_id": pid})
	ts.post(t, "/api/input", map[string]any{"process_id": pid, "data": secret + "\n"})

	replies := map[string]string{
		"read_file": ts.toolCall(t, "read_file", map[string]any{"path": saved}),
		"grep":      ts.toolCall(t, "grep", map[string]any{"root": dir, "pattern": "password"}),
		"error":     ts.toolCall(t, "read_file", map[string]any{"path": filepath.Join(dir, secret)}),
		"shell":     ts.toolCall(t, "shell", map[string]any{"command": "cat " + saved}),
		// The agent puts the text in its own command, for example after it learned it elsewhere.
		"own command": ts.toolCall(t, "shell", map[string]any{"command": "echo '" + secret + "'"}),
	}

	all := make([]string, 0, len(replies)+2)
	for name, reply := range replies {
		all = append(all, reply)
		if !strings.Contains(reply, "[redacted]") {
			t.Errorf("%s: expected [redacted] in %s", name, reply)
		}
	}
	for {
		select {
		case e := <-ts.events:
			b, _ := json.Marshal(e)
			all = append(all, string(b))
			continue
		default:
		}
		break
	}
	auditLog, err := os.ReadFile(ts.audit)
	if err != nil {
		t.Fatal(err)
	}
	all = append(all, string(auditLog))

	forms := []string{secret, `hu\"nt\\er2<Zq9`, "Zq9"}
	for i, text := range all {
		for _, form := range forms {
			if strings.Contains(text, form) {
				t.Fatalf("the secret leaks as %q (item %d): %s", form, i, text)
			}
		}
	}
}
