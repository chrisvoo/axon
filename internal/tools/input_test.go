package tools

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// A command may end before the agent calls send_input. The agent must still
// get its final output.
func TestSendInput_ReturnsFinishedResult(t *testing.T) {
	pm := NewProcManager()
	code := 0
	pm.Finish("p1", ShellResult{Status: "completed", Stdout: "got:hello\r\n", ExitCode: &code, Command: "read x"})

	b, err := SendInput("p1", "", pm)
	if err != nil {
		t.Fatal(err)
	}
	var got ShellResult
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" || !strings.Contains(got.Stdout, "got:hello") || got.ExitCode == nil || *got.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestSendInput_UnknownProcessStillFails(t *testing.T) {
	if _, err := SendInput("nope", "x", NewProcManager()); err == nil {
		t.Fatal("expected an error for an unknown process id")
	}
}

func decodeResult(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSendInput_WritesTheDataAndReturnsTheOutput(t *testing.T) {
	pm := NewProcManager()
	var written []byte
	tp := &TrackedProc{ID: "p1", OutBuf: bytes.NewBufferString("Continue? yes\r\n")}
	tp.SendInput = func(b []byte) error { written = b; return nil }
	pm.Put(tp)

	b, err := SendInput("p1", "yes\n", pm)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "yes\n" {
		t.Fatalf("the process must get the data, got %q", written)
	}
	if out := decodeResult(t, b)["stdout"]; out != "Continue? yes\r\n" {
		t.Fatalf("unexpected stdout: %q", out)
	}
}
