package tools

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"sync"
	"time"
)

// ProcManager tracks running shell processes for send_input / cancel_command.
type ProcManager struct {
	mu    sync.Mutex
	procs map[string]*TrackedProc
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

type contextCancel func()

// NewProcManager creates an empty manager.
func NewProcManager() *ProcManager {
	return &ProcManager{procs: make(map[string]*TrackedProc)}
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
	p.OutMu.Lock()
	defer p.OutMu.Unlock()
	return p.OutBuf.String()
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
