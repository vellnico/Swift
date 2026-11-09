package sysstat

import (
	"math"
	"sync"
	"time"
)

// Fake produces plausible changing counters for development on non-Linux
// machines, so the dashboard animations can be checked. Never used in production.
type Fake struct {
	mu    sync.Mutex
	start time.Time
	cpu   CPUTimes
	rx    uint64
	tx    uint64
}

func (f *Fake) phase() float64 {
	if f.start.IsZero() {
		f.start = time.Now()
	}
	return time.Since(f.start).Seconds()
}

func (f *Fake) CPU() (CPUTimes, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.phase()
	busy := 18 + 14*math.Sin(p/9) + 6*math.Sin(p/2.3)
	f.cpu.Total += 200
	f.cpu.Idle += uint64(200 * (100 - busy) / 100)
	return f.cpu, nil
}

func (f *Fake) Memory() (uint64, uint64, error) {
	p := f.phase()
	total := uint64(2 << 30)
	return total, uint64(float64(total) * (0.42 + 0.04*math.Sin(p/20))), nil
}

func (f *Fake) Net() (uint64, uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.phase()
	f.rx += uint64((2.2e6 + 1.6e6*math.Sin(p/5)) * 2 / 8)
	f.tx += uint64((4.0e5 + 3.0e5*math.Sin(p/7+1)) * 2 / 8)
	return f.rx, f.tx, nil
}

func (f *Fake) Uptime() (float64, error) { return 4*86400 + f.phase(), nil }

func (f *Fake) Disk() (uint64, uint64, error) { return 7_300_000_000, 25_000_000_000, nil }
