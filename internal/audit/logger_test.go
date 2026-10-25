package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestLogger_NilLoggerIsSafe(t *testing.T) {
	var l *Logger
	l.SetRedactor(func(s string) string { return s })
	l.Log(Entry{Action: "x"})
}
