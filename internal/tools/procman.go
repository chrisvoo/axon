package tools

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"slices"
	"sync"
	"time"
)

// finishedTTL is how long the final result of a process that returned
// input_required stays available to send_input.
const finishedTTL = 2 * time.Minute

// secretTTL is how long operator input stays hidden. It covers a program that
// writes the text to a file or log that the agent reads later.
const secretTTL = 10 * time.Minute

// ProcManager tracks running shell processes for send_input / cancel_command.
// It also keeps the operator's secrets. They are shared by all processes, so
// any text the agent can read must pass through Redact. A secret lives as long
// as the process it was typed into, then for secretTTL.
type ProcManager struct {
	mu       sync.Mutex
	procs    map[string]*TrackedProc
	finished map[string]finishedProc
	secrets  []secretEntry
	now      func() time.Time
}

type secretEntry struct {
	proc    string // the process it was typed into; "" if none
	text    string
	expires time.Time // zero while that process is live
}

type finishedProc struct {
	result  ShellResult
	expires time.Time
}

// TrackedProc holds runtime state for a shell invocation.
type TrackedProc struct {
	ID        string
	Command   string
	Cancel    contextCancel
	SendInput func([]byte) error
	Cmd       *exec.Cmd
	OutBuf    *bytes.Buffer
	OutMu     sync.Mutex
	CreatedAt time.Time
}

// snapshot returns the raw output so far. It may hold operator secrets, so it
// must go through ProcManager.Redact before it leaves the process.
func (p *TrackedProc) snapshot() string {
	p.OutMu.Lock()
	defer p.OutMu.Unlock()
	return p.OutBuf.String()
}

type contextCancel func()

// NewProcManager creates an empty manager.
func NewProcManager() *ProcManager {
	return &ProcManager{
		procs:    make(map[string]*TrackedProc),
		finished: make(map[string]finishedProc),
		now:      time.Now,
	}
}

// Finish keeps the final result of a process for finishedTTL. The operator may
// answer a prompt in the dashboard, so the agent can only learn the outcome
// from this record. Expired records are dropped here.
func (m *ProcManager) Finish(id string, res ShellResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for k, f := range m.finished {
		if !now.Before(f.expires) {
			delete(m.finished, k)
		}
	}
	m.finished[id] = finishedProc{result: res, expires: now.Add(finishedTTL)}
	m.releaseSecretsLocked(id)
}

// Finished returns the recorded result of a process that has ended.
func (m *ProcManager) Finished(id string) (ShellResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.finished[id]
	if !ok || !m.now().Before(f.expires) {
		delete(m.finished, id)
		return ShellResult{}, false
	}
	return f.result, true
}

// Register adds a process and returns its ID.
func (m *ProcManager) Register(command string, cancel contextCancel) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := randomID()
	m.procs[id] = &TrackedProc{
		ID:        id,
		Command:   command,
		Cancel:    cancel,
		CreatedAt: time.Now(),
		OutBuf:    &bytes.Buffer{},
	}
	return id
}

// Put stores the full tracked process (after cmd starts).
func (m *ProcManager) Put(tp *TrackedProc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.procs[tp.ID] = tp
}

// SetSendInput registers a writer for interactive stdin (PTY).
func (m *ProcManager) SetSendInput(id string, fn func([]byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.procs[id]; ok {
		p.SendInput = fn
	}
}

// SnapshotOutput returns a copy of accumulated stdout.
func (m *ProcManager) SnapshotOutput(id string) string {
	m.mu.Lock()
	p := m.procs[id]
	m.mu.Unlock()
	if p == nil {
		return ""
	}
	return m.Redact(p.snapshot())
}

// AddSecret hides operator input. Call it before the input is written, so that
// the terminal echo is never visible. A secret needs a live process: for an
// empty id, an unknown process or an ended process nothing is registered and the
// result is false. This stops a caller from hiding words of its choice.
func (m *ProcManager) AddSecret(procID, data string) bool {
	lines := secretLines(data)
	if procID == "" || len(lines) == 0 {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.procs[procID]; !ok || p.SendInput == nil {
		return false
	}
	// The expiry stays zero until the process ends.
	for _, line := range lines {
		m.secrets = append(m.secrets, secretEntry{proc: procID, text: line})
		if enc := jsonForm(line); enc != line {
			m.secrets = append(m.secrets, secretEntry{proc: procID, text: enc})
		}
	}
	return true
}

// releaseSecretsLocked starts the countdown for the secrets of a process that
// has ended. The caller holds m.mu.
func (m *ProcManager) releaseSecretsLocked(id string) {
	expires := m.now().Add(secretTTL)
	for i := range m.secrets {
		if m.secrets[i].proc == id && m.secrets[i].expires.IsZero() {
			m.secrets[i].expires = expires
		}
	}
}

// Redact replaces every active operator secret in s. A nil manager returns s.
func (m *ProcManager) Redact(s string) string {
	if m == nil {
		return s
	}
	m.mu.Lock()
	now := m.now()
	m.secrets = slices.DeleteFunc(m.secrets, func(e secretEntry) bool { return !e.expires.IsZero() && !now.Before(e.expires) })
	active := make([]string, len(m.secrets))
	for i, e := range m.secrets {
		active[i] = e.text
	}
	m.mu.Unlock()
	return redact(s, active)
}

// redactResult hides secrets in the output fields of a shell result. The
// command is the agent's own text and stays as it is.
func (m *ProcManager) redactResult(r ShellResult) ShellResult {
	r.Stdout = m.Redact(r.Stdout)
	r.Stderr = m.Redact(r.Stderr)
	r.LastOutput = m.Redact(r.LastOutput)
	return r
}

// Remove deletes a process id.
func (m *ProcManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.procs, id)
	m.releaseSecretsLocked(id)
}

// Get returns a tracked process.
func (m *ProcManager) Get(id string) *TrackedProc {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.procs[id]
}

// Cancel invokes cancel callback and removes the process.
func (m *ProcManager) Cancel(id string) bool {
	m.mu.Lock()
	p, ok := m.procs[id]
	if ok {
		delete(m.procs, id)
		m.releaseSecretsLocked(id)
	}
	m.mu.Unlock()
	if !ok || p == nil || p.Cancel == nil {
		return false
	}
	p.Cancel()
	return true
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
