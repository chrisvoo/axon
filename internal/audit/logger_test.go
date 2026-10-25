package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogger_RedactsDetail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	l.SetRedactor(func(s string) string { return strings.ReplaceAll(s, "hunter2", "[redacted]") })

	l.Log(Entry{RemoteIP: "1.2.3.4", Action: "shell", Detail: "mysql -phunter2 -e 'select 1'"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "hunter2") {
		t.Fatalf("the audit log leaks the secret: %s", data)
	}
	if !strings.Contains(string(data), "mysql -p[redacted]") {
		t.Fatalf("expected the redacted command, got %s", data)
	}
}

func TestLogger_StampsTheTimeWhenTheCallerDoesNot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	before := time.Now().Add(-time.Second)
	l.Log(Entry{RemoteIP: "1.2.3.4", Action: "shell"})
	explicit := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	l.Log(Entry{Time: explicit, RemoteIP: "1.2.3.4", Action: "shell"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), data)
	}

	var stamped, kept Entry
	if err := json.Unmarshal([]byte(lines[0]), &stamped); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &kept); err != nil {
		t.Fatal(err)
	}
	if stamped.Time.Before(before) || stamped.Time.After(time.Now().Add(time.Second)) {
		t.Fatalf("expected a current time, got %s", stamped.Time)
	}
	if !kept.Time.Equal(explicit) {
		t.Fatalf("an explicit time must be kept, got %s", kept.Time)
	}
}

func TestLogger_NilLoggerIsSafe(t *testing.T) {
	var l *Logger
	l.SetRedactor(func(s string) string { return s })
	l.Log(Entry{Action: "x"})
}
