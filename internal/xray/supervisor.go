package xray

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Status string

const (
	StatusRunning    Status = "ok"
	StatusRestarting Status = "busy"
	StatusStopped    Status = "off"
)

type StatusInfo struct {
	Status   Status    `json:"status"`
	PID      int       `json:"pid,omitempty"`
	Since    time.Time `json:"since"`
	Restarts int       `json:"restarts"`
}

// Supervisor keeps one xray process alive. A crashed process is restarted
// with backoff 1, 2, 4 ... 30 s; the backoff resets after 60 s of uptime.
type Supervisor struct {
	Runner     Runner
	Bin        string
	ConfigPath string
	Log        *slog.Logger
	// OnStatus is called on every status change; OnEvent for journal entries.
	OnStatus func(StatusInfo)
	OnEvent  func(level, key string, params map[string]any)

	MinBackoff time.Duration
	MaxBackoff time.Duration
	StableFor  time.Duration

	mu       sync.Mutex
	info     StatusInfo
	restart  chan chan error
	started  chan struct{}
	runStart sync.Once
}

func (s *Supervisor) init() {
	s.runStart.Do(func() {
		if s.MinBackoff == 0 {
			s.MinBackoff = time.Second
		}
		if s.MaxBackoff == 0 {
			s.MaxBackoff = 30 * time.Second
		}
		if s.StableFor == 0 {
			s.StableFor = 60 * time.Second
		}
		if s.Log == nil {
			s.Log = slog.Default()
		}
		s.restart = make(chan chan error)
		s.started = make(chan struct{})
		s.info = StatusInfo{Status: StatusStopped, Since: time.Now()}
	})
}

func (s *Supervisor) Status() StatusInfo {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

func (s *Supervisor) setStatus(st Status, pid int, restarted bool) {
	s.mu.Lock()
	s.info.Status, s.info.PID, s.info.Since = st, pid, time.Now()
	if restarted {
		s.info.Restarts++
	}
	info := s.info
	s.mu.Unlock()
	if s.OnStatus != nil {
		s.OnStatus(info)
	}
}

func (s *Supervisor) event(level, key string, params map[string]any) {
	if s.OnEvent != nil {
		s.OnEvent(level, key, params)
	}
}

var ErrNotRunning = errors.New("supervisor is not running")

// Restart replaces the running process and returns once the new one started.
func (s *Supervisor) Restart(ctx context.Context) error {
	s.init()
	done := make(chan error, 1)
	select {
	case s.restart <- done:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run blocks until ctx is cancelled, then kills xray.
func (s *Supervisor) Run(ctx context.Context) {
	s.init()
	out := &lineWriter{fn: func(l string) { s.Log.Info("xray", "line", l) }}
	backoff := s.MinBackoff
	var pending chan error // restart request waiting for the next successful start
	first := true

	for {
		proc, err := s.Runner.Start(s.Bin, s.ConfigPath, out)
		if err != nil {
			s.Log.Error("xray start failed", "err", err)
			s.event("error", "xray.start_failed", map[string]any{"error": err.Error()})
			s.setStatus(StatusStopped, 0, false)
			if pending != nil {
				pending <- err
				pending = nil
			}
			if !s.sleep(ctx, &pending, backoff) {
				return
			}
			backoff = min(backoff*2, s.MaxBackoff)
			continue
		}
		startedAt := time.Now()
		s.setStatus(StatusRunning, proc.PID(), !first)
		if first {
			s.event("info", "xray.started", map[string]any{"pid": proc.PID()})
		}
		first = false
		if pending != nil {
			pending <- nil
			pending = nil
		}

		exited := make(chan error, 1)
		go func() { exited <- proc.Wait() }()

		select {
		case <-ctx.Done():
			_ = proc.Kill()
			<-exited
			s.setStatus(StatusStopped, 0, false)
			return
		case req := <-s.restart:
			s.setStatus(StatusRestarting, 0, false)
			_ = proc.Kill()
			<-exited
			pending = req
			backoff = s.MinBackoff
			s.event("info", "xray.restarted", nil)
		case err := <-exited:
			if time.Since(startedAt) > s.StableFor {
				backoff = s.MinBackoff
			}
			code := -1
			var ec interface{ ExitCode() int }
			if errors.As(err, &ec) {
				code = ec.ExitCode()
			}
			s.Log.Warn("xray exited", "code", code, "restart_in", backoff)
			s.event("error", "xray.crashed", map[string]any{"code": code, "retry_s": int(backoff.Seconds())})
			s.setStatus(StatusRestarting, 0, false)
			if !s.sleep(ctx, &pending, backoff) {
				return
			}
			backoff = min(backoff*2, s.MaxBackoff)
		}
	}
}

// sleep waits for d, returning early (true) on a restart request and false when ctx ends.
func (s *Supervisor) sleep(ctx context.Context, pending *chan error, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		s.setStatus(StatusStopped, 0, false)
		return false
	case req := <-s.restart:
		if *pending != nil {
			*pending <- nil
		}
		*pending = req
		return true
	case <-t.C:
		return true
	}
}
