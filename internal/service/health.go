package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/sub"
	"github.com/vellnico/swift/internal/xray"
)

// Health checks every enabled profile from the server (SPEC 7.6): is the
// REALITY target reachable, and does real traffic pass through the inbound.
// It never edits a profile by itself: a fix changes client links, so the
// administrator applies it with a button.
type Health struct {
	Xray     *Xray
	Subs     *Subscriptions
	Profiles *Profiles
	Prober   xray.Prober
	// TrafficTest sends a request through one profile; replaced in tests.
	TrafficTest func(ctx context.Context, e links.Endpoint, u xray.User) (time.Duration, error)
	Event       EventFunc
	// OnReport publishes a finished check to the UI.
	OnReport func(HealthReport)

	mu      sync.Mutex
	running sync.Mutex
	last    HealthReport
}

type HealthFix struct {
	Action string `json:"action"` // "target": move REALITY to another site
	Value  string `json:"value"`
}

type ProfileHealth struct {
	ProfileID int64      `json:"profileId"`
	Name      string     `json:"name"`
	Kind      xray.Kind  `json:"kind"`
	OK        bool       `json:"ok"`
	Problem   string     `json:"problem,omitempty"` // xray_down, target_unreachable, no_traffic
	Detail    string     `json:"detail,omitempty"`
	LatencyMs int64      `json:"latencyMs,omitempty"`
	Fix       *HealthFix `json:"fix,omitempty"`
}

type HealthReport struct {
	CheckedAt time.Time       `json:"checkedAt"`
	Running   bool            `json:"running"`
	Enabled   bool            `json:"enabled"`
	Profiles  []ProfileHealth `json:"profiles"`
}

// HealthInterval is how often background checks run.
const HealthInterval = 15 * time.Minute

func (h *Health) event(level, key string, p map[string]any) {
	if h.Event != nil {
		h.Event(level, "health", key, p)
	}
}

// Report returns the last check.
func (h *Health) Report(ctx context.Context) HealthReport {
	h.mu.Lock()
	r := h.last
	h.mu.Unlock()
	if r.Profiles == nil {
		r.Profiles = []ProfileHealth{}
	}
	r.Enabled = HealthChecks(ctx, h.Xray.Store)
	if h.running.TryLock() {
		h.running.Unlock()
	} else {
		r.Running = true
	}
	return r
}

// Run checks every HealthInterval while background checks are on.
func (h *Health) Run(ctx context.Context) {
	t := time.NewTicker(HealthInterval)
	defer t.Stop()
	// give xray time to start after the panel
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Minute):
	}
	for {
		if HealthChecks(ctx, h.Xray.Store) {
			_, _ = h.Check(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// ErrHealthBusy means a check is already running.
var ErrHealthBusy = errors.New("a health check is already running")

// Check tests every enabled profile now.
func (h *Health) Check(ctx context.Context) (HealthReport, error) {
	if !h.running.TryLock() {
		return h.Report(ctx), ErrHealthBusy
	}
	if h.OnReport != nil {
		h.OnReport(h.Report(ctx)) // shows "checking" in the UI
	}
	err := h.check(ctx)
	h.running.Unlock()
	r := h.Report(ctx)
	if err == nil && h.OnReport != nil {
		h.OnReport(r)
	}
	return r, err
}

func (h *Health) check(ctx context.Context) error {
	profiles, err := h.Xray.Store.Profiles(ctx)
	if err != nil {
		return err
	}
	up := h.Xray.Sup == nil || h.Xray.Sup.Status().Status == xray.StatusRunning
	var tls links.TLS
	if h.Subs != nil {
		if tls, err = h.Subs.TLS(ctx); err != nil {
			return err
		}
	}
	probe := xray.User{ID: -1, Name: "health", VlessUUID: uuid.NewString(), Hy2Password: auth.RandomToken(24), Enabled: true}

	out := []ProfileHealth{}
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		ph := ProfileHealth{ProfileID: p.ID, Name: p.Name, Kind: p.Kind, OK: true}
		switch {
		case !up:
			ph.OK, ph.Problem = false, "xray_down"
		default:
			h.checkProfile(ctx, p, tls, probe, &ph)
		}
		out = append(out, ph)
	}

	h.mu.Lock()
	prev := map[int64]bool{}
	for _, p := range h.last.Profiles {
		prev[p.ProfileID] = p.OK
	}
	h.last = HealthReport{CheckedAt: time.Now(), Profiles: out}
	h.mu.Unlock()
	for _, p := range out {
		wasOK, seen := prev[p.ProfileID]
		switch {
		case !p.OK && (!seen || wasOK):
			h.event("warn", "health.failed", map[string]any{"name": p.Name, "problem": p.Problem})
		case p.OK && seen && !wasOK:
			h.event("info", "health.recovered", map[string]any{"name": p.Name})
		}
	}
	return nil
}

func (h *Health) checkProfile(ctx context.Context, p xray.Profile, tls links.TLS, probe xray.User, ph *ProfileHealth) {
	if p.Kind.Reality() && h.Prober != nil {
		host, _, _ := net.SplitHostPort(p.Params.Target)
		if err := h.Prober.Probe(ctx, host); err != nil {
			ph.OK, ph.Problem, ph.Detail = false, "target_unreachable", err.Error()
			if sni, err := xray.PickSNI(ctx, h.Prober, without(xray.SNICandidates, host)); err == nil {
				ph.Fix = &HealthFix{Action: "target", Value: sni}
			}
			// with a dead target REALITY cannot work; no need to send traffic
			return
		}
	}
	if h.Xray.API == nil {
		return
	}
	e := links.Endpoint{Host: "127.0.0.1", Profile: p}
	if p.Kind == xray.KindHysteria2 {
		e.TLS = tls
	}
	if err := h.Xray.API.AddUser(ctx, p.Tag(), p.Kind, probe); err != nil {
		ph.OK, ph.Problem, ph.Detail = false, "no_traffic", err.Error()
		return
	}
	defer func() { _ = h.Xray.API.RemoveUser(context.WithoutCancel(ctx), p.Tag(), probe.Email()) }()
	test := h.TrafficTest
	if test == nil {
		test = h.xrayTrafficTest
	}
	d, err := test(ctx, e, probe)
	if err != nil {
		ph.OK, ph.Problem, ph.Detail = false, "no_traffic", err.Error()
		return
	}
	ph.LatencyMs = d.Milliseconds()
}

func without(list []string, drop string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		if !strings.EqualFold(s, drop) {
			out = append(out, s)
		}
	}
	return out
}

// xrayTrafficTest starts a throwaway xray client from the subscription config
// of e and fetches sub.ProbeURL through it.
func (h *Health) xrayTrafficTest(ctx context.Context, e links.Endpoint, u xray.User) (time.Duration, error) {
	raw, err := sub.XrayJSON(sub.Request{User: u, Endpoints: []links.Endpoint{e}})
	if err != nil {
		return 0, err
	}
	var cfgs []map[string]any
	if err := json.Unmarshal(raw, &cfgs); err != nil || len(cfgs) != 1 {
		return 0, fmt.Errorf("client config: %v", err)
	}
	cfg := cfgs[0]
	port, err := loopbackPort()
	if err != nil {
		return 0, err
	}
	// the HTTP inbound only; the socks one is not needed here
	cfg["inbounds"] = []any{map[string]any{"tag": "http", "listen": "127.0.0.1", "port": port, "protocol": "http"}}
	cfg["log"] = map[string]any{"loglevel": "none"}
	dir, err := os.MkdirTemp("", "swift-health-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "client.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return 0, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, h.Xray.Bin, "run", "-c", path)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start xray client: %w", err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	proxy, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case <-cctx.Done():
			return 0, cctx.Err()
		case <-time.After(700 * time.Millisecond):
		}
		start := time.Now()
		res, err := client.Get(sub.ProbeURL)
		if err != nil {
			last = err
			continue
		}
		res.Body.Close()
		if res.StatusCode == http.StatusNoContent {
			return time.Since(start), nil
		}
		last = fmt.Errorf("status %d", res.StatusCode)
	}
	return 0, last
}

func loopbackPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// ApplyFix applies a suggested fix to a profile; client links change, so
// people need to refresh the subscription.
func (h *Health) ApplyFix(ctx context.Context, profileID int64, fix HealthFix) (xray.Profile, error) {
	p, err := h.Xray.Store.Profile(ctx, profileID)
	if err != nil {
		return p, err
	}
	switch fix.Action {
	case "target":
		if !p.Kind.Reality() || !hostRe.MatchString(fix.Value) {
			return p, &ValidationError{Fields: map[string]string{"fix": "invalid"}}
		}
		params := p.Params
		if params.Fingerprint == "" {
			params.Fingerprint = xray.DefaultFingerprint
		}
		params.Target = net.JoinHostPort(fix.Value, "443")
		params.ServerNames = []string{fix.Value}
		updated, err := h.Profiles.Update(ctx, profileID, Update{Params: &params})
		if err != nil {
			return updated, err
		}
		h.event("info", "health.fixed", map[string]any{"name": updated.Name, "target": fix.Value})
		go func() { _, _ = h.Check(context.WithoutCancel(ctx)) }()
		return updated, nil
	}
	return p, &ValidationError{Fields: map[string]string{"fix": "invalid"}}
}
