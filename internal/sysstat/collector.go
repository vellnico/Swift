package sysstat

import (
	"context"
	"io/fs"
	"math"
	"sync"
	"time"
)

// Sample is one dashboard reading.
type Sample struct {
	TS        int64   `json:"ts"` // unix ms
	CPU       float64 `json:"cpu"`
	MemUsed   uint64  `json:"memUsed"`
	MemTotal  uint64  `json:"memTotal"`
	DiskUsed  uint64  `json:"diskUsed"`
	DiskTotal uint64  `json:"diskTotal"`
	RxBps     float64 `json:"rxBps"` // bits per second
	TxBps     float64 `json:"txBps"`
	Uptime    int64   `json:"uptime"` // seconds
}

// Source reads raw counters; the Linux one uses /proc, tests and dev use others.
type Source interface {
	CPU() (CPUTimes, error)
	Memory() (total, used uint64, err error)
	Net() (rx, tx uint64, err error)
	Uptime() (float64, error)
	Disk() (used, total uint64, err error)
}

// ProcSource reads a /proc tree (fs.FS so tests can use fixtures) and statfs for the disk.
type ProcSource struct {
	Proc     fs.FS
	DiskPath string
}

func (p ProcSource) open(name string, parse func(fs.File) error) error {
	f, err := p.Proc.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return parse(f)
}

func (p ProcSource) CPU() (t CPUTimes, err error) {
	err = p.open("stat", func(f fs.File) error { t, err = ParseCPU(f); return err })
	return
}

func (p ProcSource) Memory() (total, used uint64, err error) {
	err = p.open("meminfo", func(f fs.File) error { total, used, err = ParseMeminfo(f); return err })
	return
}

func (p ProcSource) Net() (rx, tx uint64, err error) {
	err = p.open("net/dev", func(f fs.File) error { rx, tx, err = ParseNetDev(f); return err })
	return
}

func (p ProcSource) Uptime() (u float64, err error) {
	err = p.open("uptime", func(f fs.File) error { u, err = ParseUptime(f); return err })
	return
}

func (p ProcSource) Disk() (used, total uint64, err error) { return diskUsage(p.DiskPath) }

// Collector samples every Interval and keeps the last Size samples (5 minutes at 2 s).
type Collector struct {
	Source   Source
	Interval time.Duration
	Size     int
	OnSample func(Sample)
	Now      func() time.Time

	mu      sync.Mutex
	ring    []Sample
	next    int
	full    bool
	prevCPU CPUTimes
	prevRx  uint64
	prevTx  uint64
	prevAt  time.Time
	primed  bool
}

func (c *Collector) defaults() {
	if c.Interval == 0 {
		c.Interval = 2 * time.Second
	}
	if c.Size == 0 {
		c.Size = 150
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.ring == nil {
		c.ring = make([]Sample, c.Size)
	}
}

// Tick takes one reading; the first call only primes the deltas.
func (c *Collector) Tick() (Sample, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaults()
	now := c.Now()
	s := Sample{TS: now.UnixMilli()}
	cpu, cpuErr := c.Source.CPU()
	rx, tx, netErr := c.Source.Net()
	s.MemTotal, s.MemUsed, _ = c.Source.Memory()
	s.DiskUsed, s.DiskTotal, _ = c.Source.Disk()
	if up, err := c.Source.Uptime(); err == nil {
		s.Uptime = int64(up)
	}
	if !c.primed {
		c.prevCPU, c.prevRx, c.prevTx, c.prevAt, c.primed = cpu, rx, tx, now, true
		return s, false
	}
	if cpuErr == nil {
		s.CPU = math.Round(CPUPercent(c.prevCPU, cpu)*10) / 10
		c.prevCPU = cpu
	}
	if dt := now.Sub(c.prevAt).Seconds(); netErr == nil && dt > 0 {
		// counters reset when an interface goes away; treat that as zero traffic
		if rx >= c.prevRx {
			s.RxBps = float64(rx-c.prevRx) * 8 / dt
		}
		if tx >= c.prevTx {
			s.TxBps = float64(tx-c.prevTx) * 8 / dt
		}
		c.prevRx, c.prevTx = rx, tx
	}
	c.prevAt = now
	c.ring[c.next] = s
	c.next = (c.next + 1) % c.Size
	if c.next == 0 {
		c.full = true
	}
	return s, true
}

// History returns the buffered samples, oldest first.
func (c *Collector) History() []Sample {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaults()
	if !c.full {
		return append([]Sample(nil), c.ring[:c.next]...)
	}
	return append(append([]Sample(nil), c.ring[c.next:]...), c.ring[:c.next]...)
}

// Run samples until ctx ends.
func (c *Collector) Run(ctx context.Context) {
	c.mu.Lock()
	c.defaults()
	c.mu.Unlock()
	c.Tick()
	t := time.NewTicker(c.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s, ok := c.Tick(); ok && c.OnSample != nil {
				c.OnSample(s)
			}
		}
	}
}
