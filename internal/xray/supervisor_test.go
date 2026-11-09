package xray

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type fakeProc struct {
	pid  int
	done chan struct{}
	once sync.Once
}

func (p *fakeProc) Wait() error { <-p.done; return errors.New("exit status 1") }
func (p *fakeProc) Kill() error { p.once.Do(func() { close(p.done) }); return nil }
func (p *fakeProc) PID() int    { return p.pid }

type fakeRunner struct {
	mu     sync.Mutex
	starts []time.Time
	procs  []*fakeProc
	failN  int // fail the next N starts
}

func (r *fakeRunner) Test(context.Context, string, string) error { return nil }

func (r *fakeRunner) Start(string, string, io.Writer) (Process, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, time.Now())
	if r.failN > 0 {
		r.failN--
		return nil, errors.New("boom")
	}
	p := &fakeProc{pid: 100 + len(r.procs), done: make(chan struct{})}
	r.procs = append(r.procs, p)
	return p, nil
}

func (r *fakeRunner) last() *fakeProc {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.procs) == 0 {
		return nil
	}
	return r.procs[len(r.procs)-1]
}

func (r *fakeRunner) startCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.starts)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func newSup(r *fakeRunner) *Supervisor {
	return &Supervisor{Runner: r, MinBackoff: 20 * time.Millisecond, MaxBackoff: 80 * time.Millisecond, StableFor: time.Hour, Log: discardLog()}
}

func TestSupervisorRestartsCrashesWithBackoff(t *testing.T) {
	r := &fakeRunner{}
	s := newSup(r)
	var events []string
	var mu sync.Mutex
	s.OnEvent = func(_, key string, _ map[string]any) { mu.Lock(); events = append(events, key); mu.Unlock() }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	for i := 1; i <= 4; i++ {
		waitFor(t, func() bool { return r.startCount() == i && s.Status().Status == StatusRunning })
		r.last().Kill()
	}
	waitFor(t, func() bool { return r.startCount() == 5 })
	r.mu.Lock()
	gaps := []time.Duration{}
	for i := 1; i < len(r.starts); i++ {
		gaps = append(gaps, r.starts[i].Sub(r.starts[i-1]))
	}
	r.mu.Unlock()
	// 20ms, 40ms, 80ms, 80ms (capped)
	wants := []time.Duration{20, 40, 80, 80}
	for i, w := range wants {
		if gaps[i] < w*time.Millisecond || gaps[i] > w*time.Millisecond+150*time.Millisecond {
			t.Errorf("gap %d = %v, want ≈%dms", i, gaps[i], w)
		}
	}
	if s.Status().Restarts != 4 {
		t.Errorf("restarts = %d", s.Status().Restarts)
	}
	cancel()
	<-done
	if s.Status().Status != StatusStopped {
		t.Fatal("not stopped after cancel")
	}
	mu.Lock()
	defer mu.Unlock()
	if events[0] != "xray.started" || events[1] != "xray.crashed" {
		t.Fatalf("events %v", events)
	}
}

func TestSupervisorBackoffResetsAfterStableRun(t *testing.T) {
	r := &fakeRunner{}
	s := newSup(r)
	s.StableFor = 30 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	waitFor(t, func() bool { return r.startCount() == 1 })
	r.last().Kill() // quick crash: next backoff 20ms then 40ms
	waitFor(t, func() bool { return r.startCount() == 2 })
	time.Sleep(50 * time.Millisecond) // stable for longer than StableFor
	before := time.Now()
	r.last().Kill()
	waitFor(t, func() bool { return r.startCount() == 3 })
	if d := time.Since(before); d > 35*time.Millisecond+100*time.Millisecond {
		t.Fatalf("backoff not reset: %v", d)
	}
}

func TestSupervisorRestartOnRequest(t *testing.T) {
	r := &fakeRunner{}
	s := newSup(r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	waitFor(t, func() bool { return s.Status().Status == StatusRunning })
	old := s.Status().PID
	if err := s.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.Status != StatusRunning || st.PID == old {
		t.Fatalf("after restart: %+v (old pid %d)", st, old)
	}
}

func TestSupervisorStartFailureRetries(t *testing.T) {
	r := &fakeRunner{failN: 2}
	s := newSup(r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	waitFor(t, func() bool { return s.Status().Status == StatusRunning })
	if r.startCount() != 3 {
		t.Fatalf("starts = %d", r.startCount())
	}
}
