package tools

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"sync"
	"time"
)

// finishedTTL is how long the final result of a process that returned
// input_required stays available to send_input.
const finishedTTL = 2 * time.Minute

// ProcManager tracks running shell processes for send_input / cancel_command.
type ProcManager struct {
	mu       sync.Mutex
	procs    map[string]*TrackedProc
	finished map[string]finishedProc
	now      func() time.Time
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

	// secrets is input typed by the operator that must never appear in any
	// output. Guarded by OutMu.
	secrets []string
}

// addSecret registers operator input so that output never shows it.
func (p *TrackedProc) addSecret(data string) {
	lines := secretLines(data)
	p.OutMu.Lock()
	p.secrets = append(p.secrets, lines...)
	p.OutMu.Unlock()
}

// output returns the output so far with every operator secret replaced.
// All output that leaves the process (agent, dashboard) must come from here.
func (p *TrackedProc) output() string {
	p.OutMu.Lock()
	defer p.OutMu.Unlock()
	return redact(p.OutBuf.String(), p.secrets)
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
	return p.output()
}

// AddSecret registers operator input for a process. Call it before the input
// is written, so that the terminal echo is never visible.
func (m *ProcManager) AddSecret(id, data string) {
	if p := m.Get(id); p != nil {
		p.addSecret(data)
	}
}

// Remove deletes a process id.
func (m *ProcManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.procs, id)
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
