// Package sysstat samples server metrics from /proc for the dashboard.
package sysstat

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CPUTimes are the aggregate jiffies from the first line of /proc/stat.
type CPUTimes struct {
	Total uint64
	Idle  uint64 // idle + iowait
}

func ParseCPU(r io.Reader) (CPUTimes, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 5 || f[0] != "cpu" {
			continue
		}
		var t CPUTimes
		for i, s := range f[1:] {
			v, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return t, fmt.Errorf("/proc/stat: %w", err)
			}
			// guest and guest_nice (8, 9) are already counted in user and nice
			if i < 8 {
				t.Total += v
			}
			if i == 3 || i == 4 {
				t.Idle += v
			}
		}
		return t, nil
	}
	return CPUTimes{}, fmt.Errorf("/proc/stat: no cpu line")
}

// CPUPercent is the busy share between two readings, 0–100.
func CPUPercent(prev, cur CPUTimes) float64 {
	dt := float64(cur.Total - prev.Total)
	if cur.Total <= prev.Total || dt == 0 {
		return 0
	}
	busy := dt - float64(cur.Idle-prev.Idle)
	if busy < 0 {
		busy = 0
	}
	return 100 * busy / dt
}

// ParseMeminfo returns total and used memory in bytes (used = total − available).
func ParseMeminfo(r io.Reader) (total, used uint64, err error) {
	sc := bufio.NewScanner(r)
	var avail uint64
	var haveAvail bool
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		v, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			continue
		}
		switch f[0] {
		case "MemTotal:":
			total = v * 1024
		case "MemAvailable:":
			avail, haveAvail = v*1024, true
		}
	}
	if total == 0 || !haveAvail {
		return 0, 0, fmt.Errorf("/proc/meminfo: MemTotal or MemAvailable missing")
	}
	return total, total - avail, nil
}

// ParseNetDev sums received and sent bytes over all interfaces except loopback.
func ParseNetDev(r io.Reader) (rx, tx uint64, err error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "lo" {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		r, err1 := strconv.ParseUint(f[0], 10, 64)
		t, err2 := strconv.ParseUint(f[8], 10, 64)
		if err1 != nil || err2 != nil {
			return 0, 0, fmt.Errorf("/proc/net/dev: bad counters for %s", name)
		}
		rx += r
		tx += t
	}
	return rx, tx, sc.Err()
}

// ParseUptime returns seconds since boot.
func ParseUptime(r io.Reader) (float64, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, fmt.Errorf("/proc/uptime: empty")
	}
	return strconv.ParseFloat(f[0], 64)
}
