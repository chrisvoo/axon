//go:build !windows

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func finishedResult(t *testing.T, pm *ProcManager, id string) ShellResult {
	t.Helper()
	waitFor(t, "recorded result", 5*time.Second, func() bool {
		_, ok := pm.Finished(id)
		return ok
	})
	b, err := SendInput(id, "", pm)
	if err != nil {
		t.Fatalf("send_input for a finished process: %v", err)
	}
	var res ShellResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

// The operator answers in the dashboard (POST /api/input), the command ends,
// and the agent then asks for the result with send_input.
func TestRunShell_AgentReadsResultAfterOperatorAnswers(t *testing.T) {
	cfg := &config.Config{ShellTimeout: "30s", InputStallSec: 1}
	pm := NewProcManager()

	res, err := RunShell(context.Background(), cfg, nil, false, "read x; echo got:$x", pm)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "input_required" {
		t.Fatalf("expected input_required, got %+v", res)
	}
	if _, err := SendInput(res.ProcessID, "hello\n", pm); err != nil { // the operator
		t.Fatal(err)
	}

	got := finishedResult(t, pm, res.ProcessID) // the agent
	if got.Status != "completed" || !strings.Contains(got.Stdout, "got:hello") {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %v", got.ExitCode)
	}
}

func TestRunShell_AgentReadsExitCodeOfFailedCommand(t *testing.T) {
	cfg := &config.Config{ShellTimeout: "30s", InputStallSec: 1}
	pm := NewProcManager()

	res, err := RunShell(context.Background(), cfg, nil, false, "read x; exit 7", pm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SendInput(res.ProcessID, "\n", pm); err != nil {
		t.Fatal(err)
	}

	got := finishedResult(t, pm, res.ProcessID)
	if got.ExitCode == nil || *got.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %+v", got)
	}
}

func TestRunShell_AgentReadsTimeoutResult(t *testing.T) {
	cfg := &config.Config{ShellTimeout: "2s", InputStallSec: 1}
	pm := NewProcManager()

	res, err := RunShell(context.Background(), cfg, nil, false, "read x", pm)
	if err != nil {
		t.Fatal(err)
	}

	got := finishedResult(t, pm, res.ProcessID)
	if got.Status != "error" || got.ErrorMessage != "command timed out" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// The operator types a secret in the dashboard. Whether the terminal echoes it
// or the program prints it, the agent must only ever see [redacted].
func TestRunShell_OperatorSecretNeverReachesTheAgent(t *testing.T) {
	const secret = "hunter2-Zq9"
	cfg := &config.Config{ShellTimeout: "30s", InputStallSec: 1}

	t.Run("echoed by the terminal while the command runs", func(t *testing.T) {
		pm := NewProcManager()
		res, err := RunShell(context.Background(), cfg, nil, false, "read x; sleep 20", pm)
		if err != nil || res.Status != "input_required" {
			t.Fatalf("expected input_required, got %+v, %v", res, err)
		}
		defer func() { _, _ = CancelCommand(res.ProcessID, pm) }()

		reply, err := SendOperatorInput(res.ProcessID, secret+"\n", true, pm)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(reply), secret) {
			t.Fatalf("the dashboard reply leaks the secret: %s", reply)
		}

		seenRedacted := false
		for i := 0; i < 20 && !seenRedacted; i++ {
			b, err := SendInput(res.ProcessID, "", pm) // the agent
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), secret) {
				t.Fatalf("the agent sees the secret: %s", b)
			}
			seenRedacted = strings.Contains(string(b), "[redacted]")
			time.Sleep(50 * time.Millisecond)
		}
		if !seenRedacted {
			t.Fatal("expected the echoed secret to show as [redacted]")
		}
	})

	t.Run("printed by the program after the command ends", func(t *testing.T) {
		pm := NewProcManager()
		res, err := RunShell(context.Background(), cfg, nil, false, "read x; echo got:$x", pm)
		if err != nil || res.Status != "input_required" {
			t.Fatalf("expected input_required, got %+v, %v", res, err)
		}
		if _, err := SendOperatorInput(res.ProcessID, secret+"\n", true, pm); err != nil {
			t.Fatal(err)
		}

		got := finishedResult(t, pm, res.ProcessID) // the agent
		if strings.Contains(got.Stdout, secret) {
			t.Fatalf("the agent sees the secret: %q", got.Stdout)
		}
		if !strings.Contains(got.Stdout, "got:[redacted]") {
			t.Fatalf("expected got:[redacted], got %q", got.Stdout)
		}
	})
}

// The program writes what the operator typed to a file. The agent then reads
// that file with a new command. The secret is not tied to the first process.
func TestRunShell_SecretIsHiddenInAnotherCommand(t *testing.T) {
	const secret = "hunter2-Zq9"
	cfg := &config.Config{ShellTimeout: "30s", InputStallSec: 1}
	pm := NewProcManager()
	file := filepath.Join(t.TempDir(), "saved.txt")

	res, err := RunShell(context.Background(), cfg, nil, false, `read -r x; printf '%s\n' "$x" > `+file+`; sleep 20`, pm)
	if err != nil || res.Status != "input_required" {
		t.Fatalf("expected input_required, got %+v, %v", res, err)
	}
	defer func() { _, _ = CancelCommand(res.ProcessID, pm) }()
	if _, err := SendOperatorInput(res.ProcessID, secret+"\n", true, pm); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the program to write the file", 5*time.Second, func() bool {
		b, err := os.ReadFile(file)
		return err == nil && strings.Contains(string(b), secret)
	})

	read, err := RunShell(context.Background(), cfg, nil, false, "cat "+file, pm) // the agent
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read.Stdout, secret) {
		t.Fatalf("the agent reads the secret with a new command: %q", read.Stdout)
	}
	if !strings.Contains(read.Stdout, "[redacted]") {
		t.Fatalf("expected [redacted], got %q", read.Stdout)
	}
}
