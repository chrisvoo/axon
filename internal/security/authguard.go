package security

import (
	"sync"
	"time"
)

const (
	// authMaxFails is the number of wrong keys in a row that starts a block.
	authMaxFails = 5
	// authBlockFor is how long a blocked address stays blocked.
	authBlockFor = 60 * time.Second
	// maxTrackedIPs bounds the maps of the guard and of the rate limiter.
	maxTrackedIPs = 10000
)

// AuthGuard blocks an address for a while after repeated wrong keys. A 256-bit
// key cannot be guessed, so the aim is not to protect the key. The aim is to keep
// a scanner on the public tunnel URL from filling the audit log.
type AuthGuard struct {
	mu    sync.Mutex
	state map[string]*authState
	now   func() time.Time
}

type authState struct {
	fails int
	until time.Time
}

// NewAuthGuard makes an empty guard.
func NewAuthGuard() *AuthGuard {
	return &AuthGuard{state: make(map[string]*authState), now: time.Now}
}

// Blocked reports whether the address is in a block.
func (g *AuthGuard) Blocked(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.state[ip]
	return s != nil && g.now().Before(s.until)
}

// Fail records a wrong key. It returns true when this failure starts a block.
func (g *AuthGuard) Fail(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.state) >= maxTrackedIPs {
		// ponytail: drop all state at the cap. Upgrade to an LRU if scans reach it.
		g.state = make(map[string]*authState)
	}
	s := g.state[ip]
	if s == nil {
		s = &authState{}
		g.state[ip] = s
	}
	s.fails++
	if s.fails < authMaxFails {
		return false
	}
	s.fails = 0
	s.until = g.now().Add(authBlockFor)
	return true
}

// OK forgets the failures of an address that sent the right key.
func (g *AuthGuard) OK(ip string) {
	g.mu.Lock()
	delete(g.state, ip)
	g.mu.Unlock()
}
