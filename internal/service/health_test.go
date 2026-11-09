package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/xray"
)

type fakeProber struct{ down map[string]bool }

func (f fakeProber) Probe(_ context.Context, host string) error {
	if f.down[host] {
		return errors.New(host + ": timeout")
	}
	return nil
}

func TestHealthFindsProblemsAndAppliesFixes(t *testing.T) {
	ctx := context.Background()
	x, _, a, _ := setup(t) // one Vision profile with target example.com
	prober := fakeProber{down: map[string]bool{"example.com": true}}
	traffic := errors.New("no answer")
	var events []string
	var reports int
	h := &Health{
		Xray: x, Prober: prober,
		Profiles: &Profiles{Store: x.Store, Xray: x, Ports: &Ports{Store: x.Store}},
		TrafficTest: func(_ context.Context, e links.Endpoint, u xray.User) (time.Duration, error) {
			if e.Host != "127.0.0.1" || u.Email() != "u-1" {
				t.Errorf("traffic test against %s as %s", e.Host, u.Email())
			}
			return 42 * time.Millisecond, traffic
		},
		Event:    func(_, _, key string, _ map[string]any) { events = append(events, key) },
		OnReport: func(HealthReport) { reports++ },
	}

	r, err := h.Check(ctx)
	if err != nil || len(r.Profiles) != 1 || r.Running {
		t.Fatalf("report %+v %v", r, err)
	}
	p := r.Profiles[0]
	if p.OK || p.Problem != "target_unreachable" || p.Fix == nil || p.Fix.Value != xray.SNICandidates[0] {
		t.Fatalf("dead target: %+v", p)
	}
	if reports != 2 || !slices.Equal(events, []string{"health.failed"}) {
		t.Fatalf("reports %d, events %v", reports, events)
	}

	// the same problem again is not journaled twice
	_, _ = h.Check(ctx)
	if len(events) != 1 {
		t.Fatalf("events %v", events)
	}

	// target fixed, traffic still failing: the temporary user is added and removed
	updated, err := h.ApplyFix(ctx, p.ProfileID, *p.Fix)
	if err != nil || updated.Params.Target != xray.SNICandidates[0]+":443" || updated.Params.ServerNames[0] != xray.SNICandidates[0] {
		t.Fatalf("fix: %+v %v", updated.Params, err)
	}
	time.Sleep(50 * time.Millisecond) // ApplyFix rechecks in the background
	for h.Report(ctx).Running {
		time.Sleep(10 * time.Millisecond)
	}
	r, _ = h.Check(ctx)
	if p := r.Profiles[0]; p.OK || p.Problem != "no_traffic" {
		t.Fatalf("no traffic: %+v", p)
	}
	a.mu.Lock()
	log := strings.Join(a.events, ",")
	a.mu.Unlock()
	if !strings.Contains(log, "add p1 u-1") || !strings.Contains(log, "remove p1 u-1") {
		t.Fatalf("temporary user: %s", log)
	}

	traffic = nil
	r, _ = h.Check(ctx)
	if p := r.Profiles[0]; !p.OK || p.LatencyMs != 42 {
		t.Fatalf("healthy: %+v", p)
	}
	if events[len(events)-1] != "health.recovered" {
		t.Fatalf("events %v", events)
	}

	if _, err := h.ApplyFix(ctx, p.ProfileID, HealthFix{Action: "target", Value: "not a host"}); err == nil {
		t.Fatal("bad fix accepted")
	}
}

func TestHealthSwitch(t *testing.T) {
	ctx := context.Background()
	x, _, _, _ := setup(t)
	h := &Health{Xray: x}
	if !h.Report(ctx).Enabled || !AutoSelect(ctx, x.Store) {
		t.Fatal("both switches are on by default")
	}
	if err := SetSubscriptionOptions(ctx, x.Store, false, false); err != nil {
		t.Fatal(err)
	}
	if h.Report(ctx).Enabled || AutoSelect(ctx, x.Store) {
		t.Fatal("switches did not turn off")
	}
}
