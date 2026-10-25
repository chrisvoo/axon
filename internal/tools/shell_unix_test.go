//go:build !windows

package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chrisvoo/axon/internal/config"
)

func waitFor(t *testing.T, what string, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The request context ends as soon as the HTTP handler returns. A command that
// returned input_required must survive that so that send_input can reach it.
func TestRunShell_InputRequiredSurvivesRequestContext(t *testing.T) {
	cfg := &config.Config{ShellTimeout: "30s", InputStallSec: 1}
	pm := NewProcManager()
	ctx, cancel := context.WithCancel(context.Background())

	res, err := RunShell(ctx, cfg, nil, false, "read x; echo got:$x", pm)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "input_required" || res.ProcessID == "" {
		t.Fatalf("expected input_required with a process id, got %+v", res)
	}
	defer func() { _, _ = CancelCommand(res.ProcessID, pm) }()

	cancel() // the handler returned
	time.Sleep(300 * time.Millisecond)

	if _, err := SendInput(res.ProcessID, "hello\n", pm); err != nil {
		t.Fatalf("send_input after request end: %v", err)
	}
	waitFor(t, "command output", 3*time.Second, func() bool {
		return strings.Contains(pm.SnapshotOutput(res.ProcessID), "got:hello") || pm.Get(res.ProcessID) == nil
	})
}

// A command that never receives input must not live forever.
func TestRunShell_InputRequiredHonoursShellTimeout(t *testing.T) {
	cfg := &config.Config{ShellTimeout: "2s", InputStallSec: 1}
	pm := NewProcManager()

	res, err := RunShell(context.Background(), cfg, nil, false, "read x", pm)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "input_required" {
		t.Fatalf("expected input_required, got %+v", res)
	}
	waitFor(t, "process removal after shell_timeout", 5*time.Second, func() bool {
		return pm.Get(res.ProcessID) == nil
	})
}

// The request context still stops a command that has not stalled yet.
func TestRunShell_RequestContextStopsRunningCommand(t *testing.T) {
	cfg := &config.Config{ShellTimeout: "30s", InputStallSec: 10}
	pm := NewProcManager()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := RunShell(ctx, cfg, nil, false, "sleep 20", pm)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "error" || time.Since(start) > 5*time.Second {
		t.Fatalf("expected a prompt error, got %+v after %s", res, time.Since(start))
	}
}
