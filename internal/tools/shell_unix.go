//go:build !windows

package tools

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

func runShellPlatform(ctx context.Context, command string, cmdTimeout, stall time.Duration, pm *ProcManager) (ShellResult, error) {
	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	c := exec.CommandContext(ctx, shell, "-c", command)
	c.Env = os.Environ()

	ptmx, err := pty.Start(c)
	if err != nil {
		return ShellResult{Status: "error", ErrorMessage: err.Error()}, nil
	}

	var lastRead atomic.Int64
	lastRead.Store(time.Now().UnixNano())

	id := randomID()
	tp := &TrackedProc{
		ID:      id,
		Command: command,
		Cmd:     c,
		OutBuf:  &bytes.Buffer{},
	}
	tp.Cancel = func() {
		_ = c.Process.Signal(syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		_ = c.Process.Kill()
		_ = ptmx.Close()
	}
	tp.SendInput = func(b []byte) error {
		_, err := ptmx.Write(b)
		return err
	}
	pm.Put(tp)

	go func() {
		br := bufio.NewReader(ptmx)
		buf := make([]byte, 4096)
		for {
			n, err := br.Read(buf)
			if n > 0 {
				tp.OutMu.Lock()
				_, _ = tp.OutBuf.Write(buf[:n])
				tp.OutMu.Unlock()
				lastRead.Store(time.Now().UnixNano())
			}
			if err != nil {
				return
			}
		}
	}()

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
			_ = ptmx.Close()
			code := 0
			if waitErr != nil {
				if exitErr, ok := waitErr.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				} else {
					return ShellResult{Status: "error", ErrorMessage: waitErr.Error()}, nil
				}
			}
			tp.OutMu.Lock()
			out := tp.OutBuf.String()
			tp.OutMu.Unlock()
			return ShellResult{
				Status:   "completed",
				Stdout:   out,
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
			if time.Since(time.Unix(0, lastRead.Load())) >= stall {
				go func() {
					<-done
					pm.Remove(id)
					_ = ptmx.Close()
				}()
				tp.OutMu.Lock()
				out := tp.OutBuf.String()
				tp.OutMu.Unlock()
				return ShellResult{
					Status:     "input_required",
					Command:    command,
					LastOutput: tailLines(out, 20),
					Hint:       "The command is waiting for interactive input. Use send_input with process_id or cancel_command.",
					ProcessID:  id,
					ElapsedSec: int(stall.Seconds()),
					Stdout:     out,
				}, nil
			}
		}
	}
}

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
