package http

import (
	"net"
	"sync"
	"time"
)

type rateLimiter struct {
	mu         sync.Mutex
	attempts   map[string][]time.Time
	perSec     int
	burst      int
	stopCh     chan struct{}
}

func newRateLimiter(perSec int, burst int) *rateLimiter {
	rl := &rateLimiter{
		attempts: map[string][]time.Time{},
		perSec:   perSec,
		burst:    burst,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *rateLimiter) Stop() {
	close(rl.stopCh)
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
	return true
}

func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.evictExpired()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *rateLimiter) evictExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	for ip, attempts := range rl.attempts {
		valid := attempts[:0]
		for _, t := range attempts {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.attempts, ip)
		} else {
			rl.attempts[ip] = valid
		}
	}
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
