package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type proc struct {
	pid  int
	done chan struct{}
	once sync.Once
}

func (p *proc) Wait() error { <-p.done; return nil }
func (p *proc) Kill() error { p.once.Do(func() { close(p.done) }); return nil }
func (p *proc) PID() int    { return p.pid }

type runner struct {
	mu      sync.Mutex
	pid     int
	reject  string // substring of config that makes Test fail
	configs []string
}

func (r *runner) Test(_ context.Context, _, path string) error {
	b, _ := os.ReadFile(path)
	if r.reject != "" && strings.Contains(string(b), r.reject) {
		return &xray.TestError{Output: "bad config", Err: errors.New("exit 23")}
	}
	return nil
}

func (r *runner) Start(_, path string, _ io.Writer) (xray.Process, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, _ := os.ReadFile(path)
	r.configs = append(r.configs, string(b))
	r.pid++
	return &proc{pid: r.pid, done: make(chan struct{})}, nil
}

type api struct {
	mu     sync.Mutex
	down   bool
	events []string
}

func (a *api) Ping(context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.down {
		return errors.New("unavailable")
	}
	return nil
}
func (a *api) AddUser(_ context.Context, tag string, _ xray.Kind, u xray.User) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, "add "+tag+" "+u.Email())
	return nil
}
func (a *api) RemoveUser(_ context.Context, tag, email string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, "remove "+tag+" "+email)
	return nil
}
func (a *api) Users(context.Context, string) ([]string, error) { return nil, nil }

func setup(t *testing.T) (*Xray, *runner, *api, context.CancelFunc) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	priv, _, _ := xray.RealityKeyPair()
	if _, err := st.CreateProfile(ctx, xray.Profile{Name: "Vision", Kind: xray.KindVision, Port: 443, Enabled: true, Params: xray.Params{
		PrivateKey: priv, ShortIDs: xray.ShortIDs(1), Target: "example.com:443", ServerNames: []string{"example.com"},
	}}); err != nil {
		t.Fatal(err)
	}
	r := &runner{}
	a := &api{}
	sup := &xray.Supervisor{Runner: r, ConfigPath: filepath.Join(dir, "config.json"), Log: discard(), MinBackoff: 10 * time.Millisecond}
	x := &Xray{Store: st, Runner: r, Sup: sup, API: a, ConfigPath: sup.ConfigPath, Log: discard()}
	if err := x.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	rctx, cancel := context.WithCancel(ctx)
	go sup.Run(rctx)
	t.Cleanup(cancel)
	deadline := time.Now().Add(2 * time.Second)
	for sup.Status().Status != xray.StatusRunning {
		if time.Now().After(deadline) {
			t.Fatal("supervisor did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return x, r, a, cancel
}

func TestApplyRestartsWithNewConfig(t *testing.T) {
	x, r, _, _ := setup(t)
	ctx := context.Background()
	p, _ := x.Store.Profile(ctx, 1)
	p.Port = 2053
	if err := x.Store.UpdateProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	var stages []string
	x.Progress = func(s, _ string) { stages = append(stages, s) }
	if err := x.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	last := r.configs[len(r.configs)-1]
	r.mu.Unlock()
	if !strings.Contains(last, `"port": 2053`) {
		t.Fatal("xray restarted with the old config")
	}
	if strings.Join(stages, ",") != "checking,restarting,done" {
		t.Fatalf("stages %v", stages)
	}
}

func TestApplyKeepsOldConfigWhenRejected(t *testing.T) {
	x, r, _, _ := setup(t)
	ctx := context.Background()
	before, _ := os.ReadFile(x.ConfigPath)
	p, _ := x.Store.Profile(ctx, 1)
	p.Port = 9999
	_ = x.Store.UpdateProfile(ctx, p)
	r.reject = `"port": 9999`
	err := x.Apply(ctx)
	var rej *ConfigRejected
	if !errors.As(err, &rej) {
		t.Fatalf("want ConfigRejected, got %v", err)
	}
	after, _ := os.ReadFile(x.ConfigPath)
	if string(before) != string(after) {
		t.Fatal("rejected config replaced the working one")
	}
}

func TestApplyRollsBackWhenXrayDoesNotComeUp(t *testing.T) {
	x, _, a, _ := setup(t)
	ctx := context.Background()
	before, _ := os.ReadFile(x.ConfigPath)
	p, _ := x.Store.Profile(ctx, 1)
	p.Port = 8443
	_ = x.Store.UpdateProfile(ctx, p)
	a.down = true
	go func() {
		time.Sleep(10500 * time.Millisecond)
		a.mu.Lock()
		a.down = false
		a.mu.Unlock()
	}()
	if err := x.Apply(ctx); err == nil {
		t.Fatal("apply must fail when the api never answers")
	}
	after, _ := os.ReadFile(x.ConfigPath)
	if string(before) != string(after) {
		t.Fatal("config not rolled back")
	}
}

func TestHotUserChanges(t *testing.T) {
	x, r, a, _ := setup(t)
	ctx := context.Background()
	u, err := x.Store.CreateUser(ctx, store.User{Name: "Mom", SubToken: "t", VlessUUID: "6f1b0b8e-6d1f-4a7e-9a52-1b2f3c4d5e6f", Hy2Password: "p", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	starts := len(r.configs)
	if err := x.AddUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := x.RemoveUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if len(r.configs) != starts {
		t.Fatal("user change restarted xray")
	}
	if strings.Join(a.events, ";") != "add p1 u1;remove p1 u1" {
		t.Fatalf("api calls %v", a.events)
	}
	b, _ := os.ReadFile(x.ConfigPath)
	if !strings.Contains(string(b), "6f1b0b8e") {
		t.Fatal("config file not kept in sync with users")
	}
}
