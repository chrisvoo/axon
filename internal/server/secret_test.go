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
	"github.com/chrisvoo/axon/internal/security"
)

const secretText = "hunter2-Zq9"

type testServer struct {
	url    string
	key    string
	audit  string
	events <-chan events.Event
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
		ListenAddr:    "127.0.0.1",
		ListenPort:    port,
		APIKey:        "test-key",
		ShellTimeout:  "30s",
		InputStallSec: 1,
		AuditLog:      filepath.Join(dir, "audit.log"),
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
		_ = Run(ctx, cfg, deny, log, Options{Version: "test", Events: bus, DevMode: true})
	}()
	t.Cleanup(func() { cancel(); <-done })

	ts := &testServer{url: fmt.Sprintf("http://127.0.0.1:%d", port), key: cfg.APIKey, audit: cfg.AuditLog, events: sub}
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
	req.Header.Set("Authorization", "Bearer "+ts.key)
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
