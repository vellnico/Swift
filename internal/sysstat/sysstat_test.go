package sysstat

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const statA = `cpu  1000 0 500 8000 500 0 0 0 0 0
cpu0 500 0 250 4000 250 0 0 0 0 0
`
const statB = `cpu  1300 0 600 8500 600 0 0 0 0 0
`

func TestCPU(t *testing.T) {
	a, err := ParseCPU(strings.NewReader(statA))
	if err != nil || a.Total != 10000 || a.Idle != 8500 {
		t.Fatalf("parse: %+v %v", a, err)
	}
	b, _ := ParseCPU(strings.NewReader(statB))
	// busy grew by 400 of 1000 jiffies
	if p := CPUPercent(a, b); p != 40 {
		t.Fatalf("cpu %% = %v", p)
	}
	if CPUPercent(b, a) != 0 {
		t.Fatal("going backwards must not produce garbage")
	}
}

func TestMeminfo(t *testing.T) {
	total, used, err := ParseMeminfo(strings.NewReader("MemTotal:        2000000 kB\nMemFree:  100000 kB\nMemAvailable:    1500000 kB\n"))
	if err != nil || total != 2000000*1024 || used != 500000*1024 {
		t.Fatalf("%d %d %v", total, used, err)
	}
	if _, _, err := ParseMeminfo(strings.NewReader("MemTotal: 1 kB\n")); err == nil {
		t.Fatal("missing MemAvailable must fail")
	}
}

const netDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 999999    100    0    0    0     0          0         0   999999     100    0    0    0     0       0          0
  eth0: 1000      10     0    0    0     0          0         0   2000       20     0    0    0     0       0          0
  wg0:  500       5      0    0    0     0          0         0   100        1      0    0    0     0       0          0
`

func TestNetDevSkipsLoopback(t *testing.T) {
	rx, tx, err := ParseNetDev(strings.NewReader(netDev))
	if err != nil || rx != 1500 || tx != 2100 {
		t.Fatalf("%d %d %v", rx, tx, err)
	}
}

func TestUptime(t *testing.T) {
	u, err := ParseUptime(strings.NewReader("350735.47 234388.90\n"))
	if err != nil || u != 350735.47 {
		t.Fatalf("%v %v", u, err)
	}
}

type clock struct{ t time.Time }

func TestCollectorDeltasAndRing(t *testing.T) {
	proc := fstest.MapFS{
		"stat":    {Data: []byte(statA)},
		"meminfo": {Data: []byte("MemTotal: 1000 kB\nMemAvailable: 250 kB\n")},
		"net/dev": {Data: []byte(netDev)},
		"uptime":  {Data: []byte("100.5 1\n")},
	}
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	col := &Collector{Source: ProcSource{Proc: proc}, Size: 3, Now: func() time.Time { return c.t }}
	if _, ok := col.Tick(); ok {
		t.Fatal("first tick only primes")
	}
	c.t = c.t.Add(2 * time.Second)
	proc["stat"] = &fstest.MapFile{Data: []byte(statB)}
	proc["net/dev"] = &fstest.MapFile{Data: []byte(strings.Replace(strings.Replace(netDev, "1000      10", "251000    10", 1), "2000       20", "52000      20", 1))}
	s, ok := col.Tick()
	if !ok || s.CPU != 40 || s.MemUsed != 750*1024 || s.Uptime != 100 {
		t.Fatalf("sample %+v", s)
	}
	// 250000 bytes in 2 s = 1 Mbit/s down, 50000 bytes = 200 kbit/s up
	if s.RxBps != 1_000_000 || s.TxBps != 200_000 {
		t.Fatalf("net %v %v", s.RxBps, s.TxBps)
	}
	for i := 0; i < 4; i++ {
		c.t = c.t.Add(2 * time.Second)
		col.Tick()
	}
	h := col.History()
	if len(h) != 3 || h[0].TS >= h[2].TS {
		t.Fatalf("ring buffer: %d samples, order %v", len(h), []int64{h[0].TS, h[2].TS})
	}
}

func TestFakeSourceMoves(t *testing.T) {
	col := &Collector{Source: &Fake{}}
	col.Tick()
	time.Sleep(5 * time.Millisecond)
	s, ok := col.Tick()
	if !ok || s.CPU <= 0 || s.MemTotal == 0 || s.RxBps <= 0 {
		t.Fatalf("fake sample %+v", s)
	}
}
