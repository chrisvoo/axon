//go:build !windows

package server

import (
	"strings"
	"testing"
	"time"
)

// The agent answers a confirmation prompt by itself. No operator is involved.
func TestAgentConfirmsAPromptWithSendInput(t *testing.T) {
	ts := startServer(t)

	first := toolResult(t, ts.toolCall(t, "shell", map[string]any{"command": `read -p "Continue? [y/N] " a; echo answer:$a`}))
	if first["status"] != "input_required" {
		t.Fatalf("expected input_required, got %v", first)
	}
	pid, _ := first["process_id"].(string)
	if pid == "" {
		t.Fatalf("expected a process id, got %v", first)
	}

	toolResult(t, ts.toolCall(t, "send_input", map[string]any{"process_id": pid, "data": "y\n"}))

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		view := toolResult(t, ts.toolCall(t, "send_input", map[string]any{"process_id": pid, "data": ""}))
		if view["status"] == "completed" {
			if out, _ := view["stdout"].(string); !strings.Contains(out, "answer:y") {
				t.Fatalf("the command did not get the answer, got %v", view)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the command did not finish after the agent answered")
}

// A command that needs a credential cannot be answered through Axon. The agent
// stops it with cancel_command.
func TestAgentCancelsAStalledCommand(t *testing.T) {
	ts := startServer(t)

	first := toolResult(t, ts.toolCall(t, "shell", map[string]any{"command": "read x; sleep 20"}))
	pid, _ := first["process_id"].(string)
	if first["status"] != "input_required" || pid == "" {
		t.Fatalf("expected input_required with a process id, got %v", first)
	}

	cancelled := toolResult(t, ts.toolCall(t, "cancel_command", map[string]any{"process_id": pid}))
	if cancelled["status"] != "cancelled" {
		t.Fatalf("expected cancelled, got %v", cancelled)
	}
	again := toolResult(t, ts.toolCall(t, "cancel_command", map[string]any{"process_id": pid}))
	if again["status"] != "not_found" {
		t.Fatalf("a second cancel must find nothing, got %v", again)
	}
}
