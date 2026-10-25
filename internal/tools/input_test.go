package tools

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// After the operator answers in the dashboard, the command may already be
// gone. The agent must still get its final output.
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

func TestSendOperatorInput_SecretIsRegisteredBeforeTheWrite(t *testing.T) {
	pm := NewProcManager()
	var seenAtWrite []string
	tp := &TrackedProc{ID: "p1", OutBuf: &bytes.Buffer{}}
	tp.SendInput = func(b []byte) error {
		tp.OutMu.Lock()
		seenAtWrite = append([]string(nil), tp.secrets...)
		tp.OutMu.Unlock()
		return nil
	}
	pm.Put(tp)

	if _, err := SendOperatorInput("p1", "hunter2\n", true, pm); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seenAtWrite, []string{"hunter2"}) {
		t.Fatalf("the secret must be registered before it is written, got %q", seenAtWrite)
	}
}

func TestSendOperatorInput_NonSecretIsNotRedacted(t *testing.T) {
	pm := NewProcManager()
	tp := &TrackedProc{ID: "p1", OutBuf: bytes.NewBufferString("Continue? yes\r\n")}
	tp.SendInput = func([]byte) error { return nil }
	pm.Put(tp)

	b, err := SendOperatorInput("p1", "yes\n", false, pm)
	if err != nil {
		t.Fatal(err)
	}
	if out := decodeResult(t, b)["stdout"]; out != "Continue? yes\r\n" {
		t.Fatalf("a plain response must stay visible, got %q", out)
	}
}

func TestSendInput_RedactsSecretsInSnapshot(t *testing.T) {
	pm := NewProcManager()
	tp := &TrackedProc{ID: "p1", OutBuf: bytes.NewBufferString("Password: hunter2\r\n")}
	tp.SendInput = func([]byte) error { return nil }
	pm.Put(tp)

	// The operator's secret, then the agent's own call.
	if _, err := SendOperatorInput("p1", "hunter2\n", true, pm); err != nil {
		t.Fatal(err)
	}
	b, err := SendInput("p1", "", pm)
	if err != nil {
		t.Fatal(err)
	}
	if out := decodeResult(t, b)["stdout"]; out != "Password: [redacted]\r\n" {
		t.Fatalf("unexpected stdout: %q", out)
	}
}
