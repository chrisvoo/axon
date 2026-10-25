package tools

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/chrisvoo/axon/internal/config"
	"github.com/chrisvoo/axon/internal/security"
)

// ShellResult is returned from the shell tool.
type ShellResult struct {
	Status       string `json:"status"` // "completed" | "input_required" | "error"
	Stdout       string `json:"stdout,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	Command      string `json:"command,omitempty"`
	LastOutput   string `json:"last_output,omitempty"`
	Hint         string `json:"hint,omitempty"`
	ProcessID    string `json:"process_id,omitempty"`
	ElapsedSec   int    `json:"elapsed_seconds,omitempty"`
	ErrorMessage string `json:"error,omitempty"`
}

// RunShell executes a shell command with policy checks.
func RunShell(ctx context.Context, cfg *config.Config, deny *security.Denylist, readOnly bool, command string, pm *ProcManager) (ShellResult, error) {
	if readOnly {
		return ShellResult{Status: "error", ErrorMessage: "read_only mode: shell is disabled"}, nil
	}
	if deny != nil && deny.Match(command) {
		return ShellResult{Status: "error", ErrorMessage: "command blocked by denylist"}, nil
	}
	timeout := cfg.ShellTimeoutDuration()
	stall := time.Duration(cfg.InputStallSec) * time.Second
	if stall <= 0 {
		stall = 5 * time.Second
	}
	res, err := runShellPlatform(ctx, command, timeout, stall, pm)
	return pm.redactResult(res), err
}

// ShellResultJSON returns JSON bytes for MCP content.
func ShellResultJSON(r ShellResult) ([]byte, error) {
	return json.Marshal(r)
}

// exitCodeOf returns the exit code of a finished process; -1 if it did not exit normally.
func exitCodeOf(waitErr error) int {
	if waitErr == nil {
		return 0
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// tailLines returns the last maxLines lines of s.
func tailLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return s
	}
	lines := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			lines++
			if lines >= maxLines {
				return s[i+1:]
			}
		}
	}
	return s
}
