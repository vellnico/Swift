package service

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

type prober struct{ bad map[string]bool }

func (p prober) Probe(_ context.Context, host string) error {
	if p.bad[host] {
		return errors.New("no h2")
	}
	return nil
}

type checker struct{ busy map[string]bool }

func (c checker) Free(network string, port int) bool {
	return !c.busy[network+":"+strconv.Itoa(port)]
}

func newProfiles(t *testing.T, busy map[string]bool) (*Profiles, *runner) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r := &runner{}
	sup := &xray.Supervisor{Runner: r, ConfigPath: filepath.Join(dir, "config.json"), Log: discard(), MinBackoff: 10 * time.Millisecond}
	certs := &Certs{Store: st, Dir: filepath.Join(dir, "certs")}
	x := &Xray{Store: st, Runner: r, Sup: sup, API: &api{}, ConfigPath: sup.ConfigPath, Log: discard(), Certs: certs.Files}
	if err := x.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	rctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go sup.Run(rctx)
	for sup.Status().Status != xray.StatusRunning {
		time.Sleep(5 * time.Millisecond)
	}
	return &Profiles{
		Store:  st,
		Xray:   x,
		Ports:  &Ports{Store: st, Checker: checker{busy: busy}, Rand: func(int) int { return 4321 }},
		Prober: prober{bad: map[string]bool{xray.SNICandidates[0]: true}},
		Certs:  certs,
	}, r
}

func TestCreateEveryKindWithOneCall(t *testing.T) {
	ctx := context.Background()
	s, r := newProfiles(t, nil)
	vision, err := s.Create(ctx, xray.KindVision)
	if err != nil {
		t.Fatal(err)
	}
	if vision.Port != 443 || vision.Params.ServerNames[0] != xray.SNICandidates[1] || len(vision.Params.ShortIDs) != 3 || vision.Params.PublicKey == "" {
		t.Fatalf("vision: %+v", vision)
	}
	xh, err := s.Create(ctx, xray.KindXHTTP)
	if err != nil {
		t.Fatal(err)
	}
	if xh.Port != 8443 || xh.Params.Mode != "auto" || !strings.HasPrefix(xh.Params.Path, "/") {
		t.Fatalf("xhttp: %+v", xh)
	}
	hy, err := s.Create(ctx, xray.KindHysteria2)
	if err != nil {
		t.Fatal(err)
	}
	if hy.Port != 443 || len(hy.Params.ObfsPassword) != 20 {
		t.Fatalf("hysteria2 should share 443 over UDP: %+v", hy)
	}
	second, err := s.Create(ctx, xray.KindVision)
	if err != nil {
		t.Fatal(err)
	}
	if second.Port != 2053 || second.Name != "VLESS Reality 2" {
		t.Fatalf("second vision: port %d name %q", second.Port, second.Name)
	}
	r.mu.Lock()
	last := r.configs[len(r.configs)-1]
	r.mu.Unlock()
	for _, tag := range []string{`"tag": "p1"`, `"tag": "p2"`, `"tag": "p3"`, `"tag": "p4"`, `"salamander"`} {
		if !strings.Contains(last, tag) {
			t.Fatalf("running config lacks %s", tag)
		}
	}
	if _, err := s.Store.Certificate(ctx); err != nil {
		t.Fatal("hysteria2 profile created without a certificate")
	}
}

func TestPickSkipsBusyAndFallsBackToRandom(t *testing.T) {
	ctx := context.Background()
	s, _ := newProfiles(t, map[string]bool{"tcp:443": true, "tcp:8443": true, "tcp:2053": true})
	_ = s.Store.SetSetting(ctx, "panel_port", "24321")
	seq := []int{4321, 4322}
	s.Ports.Rand = func(int) int { v := seq[0]; seq = seq[1:]; return v }
	p, err := s.Ports.Pick(ctx, xray.KindVision)
	if err != nil {
		t.Fatal(err)
	}
	if p != 24322 {
		t.Fatalf("expected the second random port (the first is the panel), got %d", p)
	}
	// 24321 is the panel: the random pick must not take it
	if err := s.Ports.Check(ctx, "tcp", 24321, 0); err == nil {
		t.Fatal("panel port not reported as conflict")
	}
}

func TestManualPortChangeChecksConflicts(t *testing.T) {
	ctx := context.Background()
	s, r := newProfiles(t, nil)
	a, _ := s.Create(ctx, xray.KindVision)
	b, _ := s.Create(ctx, xray.KindXHTTP)
	hy, _ := s.Create(ctx, xray.KindHysteria2)

	port := a.Port
	_, err := s.Update(ctx, b.ID, Update{Port: &port})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Fields["port"] != "port_conflict" || ve.Detail["by"] != a.Name {
		t.Fatalf("tcp conflict not detected: %v", err)
	}
	// the same number over UDP does not clash with TCP
	if _, err := s.Update(ctx, hy.ID, Update{Port: &port}); err != nil {
		t.Fatalf("udp 443 next to tcp 443: %v", err)
	}
	newPort := 9443
	got, err := s.Update(ctx, b.ID, Update{Port: &newPort})
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 9443 || !got.Manual {
		t.Fatalf("update: %+v", got)
	}
	r.mu.Lock()
	last := r.configs[len(r.configs)-1]
	r.mu.Unlock()
	if !strings.Contains(last, `"port": 9443`) {
		t.Fatal("new port not applied")
	}
}

func TestUpdateRestoresRowWhenXrayRejects(t *testing.T) {
	ctx := context.Background()
	s, r := newProfiles(t, nil)
	p, _ := s.Create(ctx, xray.KindVision)
	r.reject = `"port": 7777`
	port := 7777
	if _, err := s.Update(ctx, p.ID, Update{Port: &port}); err == nil {
		t.Fatal("expected rejection")
	}
	back, _ := s.Store.Profile(ctx, p.ID)
	if back.Port != p.Port {
		t.Fatalf("row not restored: port %d", back.Port)
	}
}

func TestValidationAndRegenerate(t *testing.T) {
	ctx := context.Background()
	s, _ := newProfiles(t, nil)
	p, _ := s.Create(ctx, xray.KindXHTTP)
	bad := p.Params
	bad.Path, bad.Fingerprint, bad.ServerNames, bad.Target = "no-slash", "netscape", []string{"bad host"}, "host"
	_, err := s.Update(ctx, p.ID, Update{Params: &bad})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want validation error, got %v", err)
	}
	for _, f := range []string{"path", "fingerprint", "serverNames", "target"} {
		if ve.Fields[f] == "" {
			t.Errorf("%s not flagged: %v", f, ve.Fields)
		}
	}
	got, err := s.Update(ctx, p.ID, Update{Regenerate: []string{"privateKey", "path"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Params.PrivateKey == p.Params.PrivateKey || got.Params.Path == p.Params.Path || got.Params.PublicKey == p.Params.PublicKey {
		t.Fatal("regenerate did not change values")
	}
	name := "Office"
	renamed, _ := s.Update(ctx, p.ID, Update{Name: &name})
	if renamed.Name != "Office" {
		t.Fatal("rename failed")
	}
	if _, err := s.Update(ctx, p.ID, Update{Regenerate: []string{"obfsPassword"}}); err == nil {
		t.Fatal("regenerating a field of another kind must fail")
	}
}

func TestNoSNIAvailable(t *testing.T) {
	ctx := context.Background()
	s, _ := newProfiles(t, nil)
	bad := map[string]bool{}
	for _, h := range xray.SNICandidates {
		bad[h] = true
	}
	s.Prober = prober{bad: bad}
	if _, err := s.Create(ctx, xray.KindVision); err == nil {
		t.Fatal("expected an error when no Reality target works")
	}
	ps, _ := s.Store.Profiles(ctx)
	if len(ps) != 0 {
		t.Fatal("half-created profile left behind")
	}
}
