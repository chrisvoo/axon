package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Entry is a single audit log record.
type Entry struct {
	Time     time.Time `json:"time"`
	RemoteIP string    `json:"remote_ip"`
	Action   string    `json:"action"`
	Detail   string    `json:"detail,omitempty"`
	ExitCode *int      `json:"exit_code,omitempty"`
}

// Logger appends JSON lines to a file.
type Logger struct {
	path   string
	mu     sync.Mutex
	redact func(string) string
}

// SetRedactor sets a function that hides secrets in every entry's detail.
func (l *Logger) SetRedactor(f func(string) string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.redact = f
	l.mu.Unlock()
}

// New creates an audit logger.
func New(path string) (*Logger, error) {
	if path == "" {
		return &Logger{}, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	return &Logger{path: path}, nil
}

// Log writes one JSON line.
func (l *Logger) Log(e Entry) {
	if l == nil || l.path == "" {
		return
	}
	e.Time = e.Time.UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.redact != nil {
		e.Detail = l.redact(e.Detail)
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s\n", data)
}
