package tools

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProcManager_FinishedResultExpires(t *testing.T) {
	pm := NewProcManager()
	now := time.Unix(1000, 0)
	pm.now = func() time.Time { return now }
	code := 3
	pm.Finish("p1", ShellResult{Status: "completed", Stdout: "out", ExitCode: &code})

	res, ok := pm.Finished("p1")
	if !ok || res.Stdout != "out" || res.ExitCode == nil || *res.ExitCode != 3 {
		t.Fatalf("expected the stored result, got %+v ok=%v", res, ok)
	}

	now = now.Add(finishedTTL - time.Second)
	if _, ok := pm.Finished("p1"); !ok {
		t.Fatal("result must still be available just before the TTL")
	}

	now = now.Add(2 * time.Second)
	if _, ok := pm.Finished("p1"); ok {
		t.Fatal("result must expire after the TTL")
	}
}

func TestProcManager_FinishedUnknownID(t *testing.T) {
	if _, ok := NewProcManager().Finished("nope"); ok {
		t.Fatal("unknown id must not have a result")
	}
}

func TestProcManager_FinishDropsExpiredResults(t *testing.T) {
	pm := NewProcManager()
	now := time.Unix(1000, 0)
	pm.now = func() time.Time { return now }
	pm.Finish("old", ShellResult{Status: "completed"})

	now = now.Add(finishedTTL + time.Second)
	pm.Finish("new", ShellResult{Status: "completed"})

	if len(pm.finished) != 1 {
		t.Fatalf("expired results must be pruned, %d left", len(pm.finished))
	}
}

func TestProcManager_SecretStoreCoversAnyOutput(t *testing.T) {
	pm := NewProcManager()
	pm.AddSecret("hunter2\n")
	if got := pm.Redact("a hunter2 b"); got != "a [redacted] b" {
		t.Fatalf("got %q", got)
	}
}

func TestProcManager_SecretExpires(t *testing.T) {
	pm := NewProcManager()
	now := time.Unix(1000, 0)
	pm.now = func() time.Time { return now }
	pm.AddSecret("hunter2")

	now = now.Add(secretTTL - time.Second)
	if got := pm.Redact("hunter2"); got != "[redacted]" {
		t.Fatalf("the secret must still be hidden just before the TTL, got %q", got)
	}
	now = now.Add(2 * time.Second)
	if got := pm.Redact("hunter2"); got != "hunter2" {
		t.Fatalf("an expired secret must not be replaced, got %q", got)
	}
	if len(pm.secrets) != 0 {
		t.Fatalf("expired secrets must be dropped, %d left", len(pm.secrets))
	}
}

// Output is often JSON-encoded after it is built. The encoded form of a secret
// with quotes, backslashes or angle brackets differs from the typed form.
func TestProcManager_SecretIsHiddenInJSONEncodedOutput(t *testing.T) {
	const secret = `hu"nt\er2<Zq9`
	pm := NewProcManager()
	pm.AddSecret(secret + "\n")

	encoded, err := json.Marshal(map[string]string{"text": "password=" + secret})
	if err != nil {
		t.Fatal(err)
	}
	got := pm.Redact(string(encoded))
	if strings.Contains(got, "Zq9") {
		t.Fatalf("the encoded secret leaks: %s", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Fatalf("expected [redacted], got %s", got)
	}
}

func TestProcManager_WhitespaceOnlyInputIsNotASecret(t *testing.T) {
	pm := NewProcManager()
	pm.AddSecret("  \n")
	if got := pm.Redact("a b c"); got != "a b c" {
		t.Fatalf("got %q", got)
	}
}

func TestProcManager_NilManagerPassesTextThrough(t *testing.T) {
	var pm *ProcManager
	if got := pm.Redact("text"); got != "text" {
		t.Fatalf("got %q", got)
	}
}

// A secret typed into one process must be hidden in the output of every other.
func TestProcManager_SecretFromOneProcessIsHiddenInAnother(t *testing.T) {
	pm := NewProcManager()
	other := &TrackedProc{ID: "other", OutBuf: bytes.NewBufferString("leftover: hunter2\n")}
	pm.Put(other)
	pm.AddSecret("hunter2\n") // typed into some other process

	if got := pm.SnapshotOutput("other"); got != "leftover: [redacted]\n" {
		t.Fatalf("got %q", got)
	}
}

func TestProcManager_RedactResultKeepsTheCommand(t *testing.T) {
	pm := NewProcManager()
	pm.AddSecret("hunter2")
	res := pm.redactResult(ShellResult{
		Command:    "echo hunter2",
		Stdout:     "hunter2",
		Stderr:     "err hunter2",
		LastOutput: "last hunter2",
	})
	if res.Stdout != "[redacted]" || res.Stderr != "err [redacted]" || res.LastOutput != "last [redacted]" {
		t.Fatalf("output fields must be redacted: %+v", res)
	}
	if res.Command != "echo hunter2" {
		t.Fatalf("the command is the agent's own text and must stay: %q", res.Command)
	}
}

func TestSendInput_RedactsFinishedResultWithCurrentSecrets(t *testing.T) {
	pm := NewProcManager()
	code := 0
	pm.Finish("p1", ShellResult{Status: "completed", Stdout: "token hunter2\n", ExitCode: &code})
	pm.AddSecret("hunter2") // typed after the process ended

	b, err := SendInput("p1", "", pm)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "hunter2") {
		t.Fatalf("the finished result leaks: %s", b)
	}
}
