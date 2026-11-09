package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vellnico/swift/internal/xray"
)

func TestUsersHotChanges(t *testing.T) {
	ctx := context.Background()
	x, r, a, _ := setup(t)
	s := &Users{Store: x.Store, Xray: x}
	starts := len(r.configs)

	u, err := s.Create(ctx, "  Mom ", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "Mom" || len(u.SubToken) != 32 || len(u.Hy2Password) != 32 || len(u.VlessUUID) != 36 {
		t.Fatalf("credentials: %+v", u)
	}
	if _, err := s.Create(ctx, "Mom", ""); err == nil {
		t.Fatal("duplicate name accepted")
	} else {
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Fields["name"] != "duplicate" {
			t.Fatalf("want duplicate error, got %v", err)
		}
	}
	if _, err := s.Create(ctx, "   ", ""); err == nil {
		t.Fatal("empty name accepted")
	}

	off := false
	if _, err := s.Update(ctx, u.ID, UserUpdate{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	on := true
	if _, err := s.Update(ctx, u.ID, UserUpdate{Enabled: &on}); err != nil {
		t.Fatal(err)
	}
	re, err := s.Reissue(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if re.SubToken == u.SubToken || re.VlessUUID == u.VlessUUID || re.Hy2Password == u.Hy2Password {
		t.Fatal("reissue kept old credentials")
	}
	if _, err := s.Delete(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if len(r.configs) != starts {
		t.Fatal("user operations restarted xray")
	}
	want := "add p1 u1;remove p1 u1;add p1 u1;remove p1 u1;add p1 u1;remove p1 u1"
	if got := strings.Join(a.events, ";"); got != want {
		t.Fatalf("api calls:\n got %s\nwant %s", got, want)
	}
	if _, err := x.Store.User(ctx, u.ID); err == nil {
		t.Fatal("user still in the database")
	}
}

func TestSubscriptionEndpointsAndURL(t *testing.T) {
	ctx := context.Background()
	x, _, _, _ := setup(t)
	certs := &Certs{Store: x.Store, Dir: t.TempDir()}
	subs := &Subscriptions{Store: x.Store, Certs: certs}
	_ = x.Store.SetSetting(ctx, "sub_port", "31000")

	url, err := subs.URL(ctx, "tok", "panel.local:24680")
	if err != nil || url != "http://panel.local:31000/s/tok" {
		t.Fatalf("fallback host: %q %v", url, err)
	}
	_ = x.Store.SetSetting(ctx, "server_address", "203.0.113.7")
	url, _ = subs.URL(ctx, "tok", "panel.local:24680")
	if url != "http://203.0.113.7:31000/s/tok" {
		t.Fatalf("server address: %q", url)
	}
	_ = x.Store.SetSetting(ctx, "domain", "vpn.example.com")
	url, _ = subs.URL(ctx, "tok", "")
	if url != "http://vpn.example.com:31000/s/tok" {
		t.Fatalf("domain: %q", url)
	}

	if _, err := certs.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	tls, err := subs.TLS(ctx)
	if err != nil || len(tls.PinSHA256) != 64 || !strings.Contains(tls.CertPEM, "BEGIN CERTIFICATE") || tls.ServerName == "" {
		t.Fatalf("self-signed TLS: %+v %v", tls, err)
	}
	eps, err := subs.Endpoints(ctx, "")
	if err != nil || len(eps) != 1 || eps[0].Profile.Kind != xray.KindVision || eps[0].Host != "vpn.example.com" {
		t.Fatalf("endpoints: %+v %v", eps, err)
	}

	port, err := EnsureSubPort(ctx, x.Store, 0, 31000, func() int { return 31000 })
	if err != nil || port != 31000 {
		t.Fatalf("existing sub port must be kept: %d %v", port, err)
	}
}

func TestEnsureSubPortAvoidsPanel(t *testing.T) {
	ctx := context.Background()
	x, _, _, _ := setup(t)
	seq := []int{24680, 24680, 31001}
	port, err := EnsureSubPort(ctx, x.Store, 0, 24680, func() int { v := seq[0]; seq = seq[1:]; return v })
	if err != nil || port != 31001 {
		t.Fatalf("port %d %v", port, err)
	}
}
