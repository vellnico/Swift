package auth

import (
	"sync"
	"time"
)

// Limiter blocks an IP after Max failed logins within Window for Block.
type Limiter struct {
	Max    int
	Window time.Duration
	Block  time.Duration
	Now    func() time.Time

	mu      sync.Mutex
	entries map[string]*limitEntry
}

type limitEntry struct {
	fails        []time.Time
	blockedUntil time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{Max: 5, Window: 10 * time.Minute, Block: 15 * time.Minute, Now: time.Now, entries: map[string]*limitEntry{}}
}

// Allow reports whether ip may try to log in now, and if not, for how long it is blocked.
func (l *Limiter) Allow(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[ip]
	if e == nil {
		return true, 0
	}
	if now := l.Now(); now.Before(e.blockedUntil) {
		return false, e.blockedUntil.Sub(now)
	}
	return true, 0
}

// Fail records a failed attempt and reports whether this attempt triggered a block.
func (l *Limiter) Fail(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Now()
	l.gc(now)
	e := l.entries[ip]
	if e == nil {
		e = &limitEntry{}
		l.entries[ip] = e
	}
	cut := now.Add(-l.Window)
	kept := e.fails[:0]
	for _, t := range e.fails {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	e.fails = append(kept, now)
	if len(e.fails) >= l.Max {
		e.blockedUntil = now.Add(l.Block)
		e.fails = nil
		return true
	}
	return false
}

// Success clears the failure history of ip.
func (l *Limiter) Success(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

func (l *Limiter) gc(now time.Time) {
	if len(l.entries) < 1024 {
		return
	}
	for ip, e := range l.entries {
		if now.After(e.blockedUntil) && (len(e.fails) == 0 || now.Sub(e.fails[len(e.fails)-1]) > l.Window) {
			delete(l.entries, ip)
		}
	}
}
