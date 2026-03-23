//go:build windows

package tools

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
)

func runShellPlatform(ctx context.Context, command string, cmdTimeout, stall time.Duration, pm *ProcManager) (ShellResult, error) {
	shell, args := resolveWindowsShell(command)
	c := exec.CommandContext(ctx, shell, args...)
	c.Env = os.Environ()
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	stdin, err := c.StdinPipe()
	if err != nil {
		return ShellResult{Status: "error", ErrorMessage: err.Error()}, nil
	}

	if err := c.Start(); err != nil {
		return ShellResult{Status: "error", ErrorMessage: err.Error()}, nil
	}

	var lastRead atomic.Int64
	var lastLen atomic.Int64
	lastRead.Store(time.Now().UnixNano())
	lastLen.Store(0)

	id := randomID()
	tp := &TrackedProc{
		ID:      id,
		Command: command,
		Cmd:     c,
		OutBuf:  &buf,
	}
	tp.Cancel = func() {
		_ = c.Process.Kill()
		_ = stdin.Close()
	}
	tp.SendInput = func(b []byte) error {
		_, err := stdin.Write(b)
		return err
	}
	pm.Put(tp)

	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = c.Wait()
		close(done)
	}()

	deadline := time.After(cmdTimeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			pm.Remove(id)
			_ = stdin.Close()
			code := 0
			if waitErr != nil {
				if exitErr, ok := waitErr.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				} else {
					return ShellResult{Status: "error", ErrorMessage: waitErr.Error()}, nil
				}
			}
			return ShellResult{
				Status:   "completed",
				Stdout:   buf.String(),
				ExitCode: &code,
				Command:  command,
			}, nil

		case <-deadline:
			tp.Cancel()
			pm.Remove(id)
			return ShellResult{Status: "error", ErrorMessage: "command timed out"}, nil

		case <-ctx.Done():
			tp.Cancel()
			pm.Remove(id)
			return ShellResult{Status: "error", ErrorMessage: ctx.Err().Error()}, nil

		case <-ticker.C:
			if c.ProcessState != nil {
				continue
			}
			n := int64(buf.Len())
			if n > lastLen.Load() {
				lastLen.Store(n)
				lastRead.Store(time.Now().UnixNano())
			}
			if time.Since(time.Unix(0, lastRead.Load())) >= stall {
				go func() {
					<-done
					pm.Remove(id)
					_ = stdin.Close()
				}()
				return ShellResult{
					Status:     "input_required",
					Command:    command,
					LastOutput: tailLines(buf.String(), 20),
					Hint:       "The command is waiting for interactive input. Use send_input with process_id or cancel_command.",
					ProcessID:  id,
					ElapsedSec: int(stall.Seconds()),
					Stdout:     buf.String(),
				}, nil
			}
		}
	}
}

func resolveWindowsShell(command string) (string, []string) {
	if p, err := exec.LookPath("pwsh.exe"); err == nil {
		return p, []string{"-NoProfile", "-Command", command}
	}
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p, []string{"-NoProfile", "-Command", command}
	}
	com := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	return com, []string{"/C", command}
}
