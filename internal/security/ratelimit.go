package security

import (
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

// Allow returns false if the address is rate limited. ip comes from ClientIP.
func (rl *RateLimiter) Allow(ip string) bool {
	if rl.rps <= 0 {
		return true
	}
	rl.mu.Lock()
	lim, ok := rl.limits[ip]
	if !ok {
		if len(rl.limits) >= maxTrackedIPs {
			// ponytail: drop all limiters at the cap. Upgrade to an LRU if scans reach it.
			rl.limits = make(map[string]*rate.Limiter)
		}
		lim = rate.NewLimiter(rate.Limit(rl.rps), int(rl.rps)+1)
		rl.limits[ip] = lim
	}
	rl.mu.Unlock()
	return lim.Allow()
}
