package security

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter limits requests per IP when rps > 0.
type RateLimiter struct {
	rps    float64
	mu     sync.Mutex
	limits map[string]*rate.Limiter
}

// NewRateLimiter creates a limiter; rps 0 disables limiting.
func NewRateLimiter(rps float64) *RateLimiter {
	if rps <= 0 {
		return &RateLimiter{rps: 0}
	}
	return &RateLimiter{
		rps:    rps,
		limits: make(map[string]*rate.Limiter),
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Allow returns false if rate limited.
func (rl *RateLimiter) Allow(r *http.Request) bool {
	if rl.rps <= 0 {
		return true
	}
	ip := clientIP(r)
	rl.mu.Lock()
	lim, ok := rl.limits[ip]
	if !ok {
		lim = rate.NewLimiter(rate.Limit(rl.rps), int(rl.rps)+1)
		rl.limits[ip] = lim
	}
	rl.mu.Unlock()
	return lim.Allow()
}
