package http

import (
	"net"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	perSec   int
	burst    int
}

func newRateLimiter(perSec int, burst int) *rateLimiter {
	return &rateLimiter{
		attempts: map[string][]time.Time{},
		perSec:   perSec,
		burst:    burst,
	}
}

func (rl *rateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	window := now.Add(-time.Second)

	attempts := rl.attempts[ip]
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(window) {
			valid = append(valid, t)
		}
	}
	rl.attempts[ip] = valid

	if len(valid) >= rl.burst {
		return false
	}

	rl.attempts[ip] = append(valid, now)

	// Clean up old entries periodically.
	if len(rl.attempts) > 10000 {
		for ip, attempts := range rl.attempts {
			if len(attempts) == 0 {
				delete(rl.attempts, ip)
			}
		}
	}

	return true
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
