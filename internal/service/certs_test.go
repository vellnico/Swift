package service

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/certs"
)

// fakeIssuer stands in for Let's Encrypt: it issues a fresh certificate per call.
type fakeIssuer struct {
	calls atomic.Int32
	fail  bool
	now   time.Time
}

func (f *fakeIssuer) Obtain(_ context.Context, domain string) (certs.Issued, error) {
	f.calls.Add(1)
	if f.fail {
		return certs.Issued{}, errors.New("urn:ietf:params:acme:error:connection: timeout during connect")
	}
	is, err := certs.SelfSigned(domain, nil, f.now)
	is.NotAfter = f.now.Add(90 * 24 * time.Hour)
	if net.ParseIP(domain) != nil {
		is.NotAfter = f.now.Add(160 * time.Hour) // Let's Encrypt shortlived profile
	}
	return is, err
}

func leafFP(t *testing.T, addr string) string {
	t.Helper()
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // test inspects the served certificate
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return certs.FingerprintHex(conn.ConnectionState().PeerCertificates[0].Raw)
}

// Forced renewal with a mock ACME provider restarts xray and swaps the panel
// certificate without restarting the TLS listener (phase 7 acceptance).
func TestForcedRenewalSwapsCertificateWithoutDowntime(t *testing.T) {
	ctx := context.Background()
	x, r, _, _ := setup(t)
	now := time.Unix(1_800_000_000, 0)
	issuer := &fakeIssuer{now: now}
	holder := &certs.Holder{}
	c := &Certs{
		Store: x.Store, Dir: filepath.Join(t.TempDir(), "certs"), Issuer: issuer, Holder: holder, Apply: x.Reload,
		Now: func() time.Time { return now }, Resolve: func(context.Context, string) ([]string, error) { return []string{"203.0.113.7"}, nil },
	}
	x.Certs = c.Files
	_ = x.Store.SetSetting(ctx, "server_address", "203.0.113.7")
	if _, err := c.Ensure(ctx); err != nil {
		t.Fatal(err)
	}

	// a TLS listener like the panel's, never restarted during the test
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }), TLSConfig: holder.TLSConfig()}
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	defer srv.Close()
	addr := ln.Addr().String()
	selfFP := leafFP(t, addr)

	issued, err := c.IssueACME(ctx, "vpn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	acmeFP := leafFP(t, addr)
	if acmeFP == selfFP || acmeFP != issued.SHA256Hex {
		t.Fatal("panel did not switch to the ACME certificate in place")
	}
	if d, _ := x.Store.Setting(ctx, "domain"); d != "vpn.example.com" {
		t.Fatalf("domain setting %q", d)
	}

	// not due yet: nothing happens
	if renewed, err := c.RenewIfNeeded(ctx, false); err != nil || renewed {
		t.Fatalf("early renewal: %v %v", renewed, err)
	}
	starts := len(r.configs)

	// keep a client hammering the listener while renewing: no request may fail
	var failures atomic.Int32
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, DisableKeepAlives: true}} //nolint:gosec
		for {
			select {
			case <-stop:
				return
			default:
			}
			resp, err := client.Get("https://" + addr)
			if err != nil {
				failures.Add(1)
				continue
			}
			resp.Body.Close()
		}
	}()
	renewed, err := c.RenewIfNeeded(ctx, true)
	close(stop)
	<-done
	if err != nil || !renewed {
		t.Fatalf("forced renewal: %v %v", renewed, err)
	}
	if failures.Load() != 0 {
		t.Fatalf("%d requests failed during renewal", failures.Load())
	}
	if leafFP(t, addr) == acmeFP {
		t.Fatal("panel still serves the old certificate")
	}
	if len(r.configs) <= starts {
		t.Fatal("xray was not restarted with the renewed certificate")
	}

	// expiring within 30 days renews automatically
	now = now.Add(61 * 24 * time.Hour)
	if renewed, err := c.RenewIfNeeded(ctx, false); err != nil || !renewed {
		t.Fatalf("due renewal: %v %v", renewed, err)
	}
}

func TestRenewalFailureIsRecorded(t *testing.T) {
	ctx := context.Background()
	x, _, _, _ := setup(t)
	issuer := &fakeIssuer{now: time.Now()}
	var events []string
	c := &Certs{Store: x.Store, Dir: t.TempDir(), Issuer: issuer, Holder: &certs.Holder{}, Event: func(_, _, key string, _ map[string]any) { events = append(events, key) }}
	if _, err := c.IssueACME(ctx, "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	issuer.fail = true
	_, err := c.RenewIfNeeded(ctx, true)
	var ie *IssueError
	if !errors.As(err, &ie) {
		t.Fatalf("want IssueError, got %v", err)
	}
	cur, _ := x.Store.Certificate(ctx)
	if !strings.Contains(cur.LastError, "timeout") || cur.Kind != "acme" {
		t.Fatalf("error not recorded: %+v", cur)
	}
	if events[len(events)-1] != "certs.renew_failed" {
		t.Fatalf("events %v", events)
	}
}

func TestIssueACMEValidatesDomain(t *testing.T) {
	ctx := context.Background()
	x, _, _, _ := setup(t)
	_ = x.Store.SetSetting(ctx, "server_address", "203.0.113.7")
	c := &Certs{Store: x.Store, Dir: t.TempDir(), Issuer: &fakeIssuer{now: time.Now()},
		Resolve: func(context.Context, string) ([]string, error) { return []string{"198.51.100.1"}, nil }}
	for domain, code := range map[string]string{"not a domain": "invalid", "1.2.3.4": "invalid", "vpn.example.com": "dns_mismatch"} {
		_, err := c.IssueACME(ctx, domain)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Fields["domain"] != code {
			t.Errorf("%q: want %s, got %v", domain, code, err)
		}
	}
}

// Without a domain the server gets a short-lived Let's Encrypt certificate for its IP.
func TestIPCertificateLifecycle(t *testing.T) {
	ctx := context.Background()
	x, _, _, _ := setup(t)
	now := time.Unix(1_800_000_000, 0)
	issuer := &fakeIssuer{now: now, fail: true}
	var events []string
	c := &Certs{Store: x.Store, Dir: filepath.Join(t.TempDir(), "certs"), Issuer: issuer, Holder: &certs.Holder{},
		Now: func() time.Time { return now }, Event: func(_, _, key string, _ map[string]any) { events = append(events, key) }}
	_ = x.Store.SetSetting(ctx, "server_address", "203.0.113.7")
	if _, err := c.Ensure(ctx); err != nil {
		t.Fatal(err)
	}

	// Let's Encrypt unreachable: stays self-signed, the same error is journaled once
	for range 2 {
		if up, err := c.RenewIfNeeded(ctx, false); err != nil || up {
			t.Fatalf("failed upgrade: %v %v", up, err)
		}
	}
	if cur, _ := x.Store.Certificate(ctx); cur.Kind != "self_signed" || cur.LastError == "" || len(events) != 1 {
		t.Fatalf("after failed upgrade: %+v, events %v", cur, events)
	}

	issuer.fail = false
	if up, err := c.RenewIfNeeded(ctx, false); err != nil || !up {
		t.Fatalf("upgrade: %v %v", up, err)
	}
	cur, _ := x.Store.Certificate(ctx)
	if !IsIPCertificate(cur) || cur.Domain != "203.0.113.7" || cur.LastError != "" {
		t.Fatalf("not an IP certificate: %+v", cur)
	}
	if d, _ := x.Store.Setting(ctx, "domain"); d != "" {
		t.Fatalf("IP mode must not set a domain, got %q", d)
	}

	// six days of validity: not due on day 1, due with less than 3 days left
	now = now.Add(24 * time.Hour)
	if renewed, _ := c.RenewIfNeeded(ctx, false); renewed {
		t.Fatal("renewed too early")
	}
	now = now.Add(3 * 24 * time.Hour)
	if renewed, err := c.RenewIfNeeded(ctx, false); err != nil || !renewed {
		t.Fatalf("due renewal: %v %v", renewed, err)
	}

	// renewal keeps failing until the certificate expires: fall back to self-signed
	issuer.fail = true
	now = now.Add(7 * 24 * time.Hour)
	if _, err := c.RenewIfNeeded(ctx, false); err == nil {
		t.Fatal("want renewal error")
	}
	if cur, _ := x.Store.Certificate(ctx); cur.Kind != "self_signed" {
		t.Fatalf("expired IP certificate kept: %+v", cur)
	}
}

func TestSwitchToIPFallsBackToSelfSigned(t *testing.T) {
	ctx := context.Background()
	x, _, _, _ := setup(t)
	issuer := &fakeIssuer{now: time.Now()}
	c := &Certs{Store: x.Store, Dir: t.TempDir(), Issuer: issuer, Holder: &certs.Holder{},
		Resolve: func(context.Context, string) ([]string, error) { return []string{"203.0.113.7"}, nil }}
	_ = x.Store.SetSetting(ctx, "server_address", "203.0.113.7")
	if _, err := c.IssueACME(ctx, "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	issuer.fail = true
	cur, err := c.SwitchToIP(ctx)
	if err != nil || cur.Kind != "self_signed" || !strings.Contains(cur.LastError, "timeout") {
		t.Fatalf("fallback: %+v %v", cur, err)
	}
	issuer.fail = false
	if cur, err := c.SwitchToIP(ctx); err != nil || !IsIPCertificate(cur) {
		t.Fatalf("switch: %+v %v", cur, err)
	}
	// a private address cannot be validated by Let's Encrypt
	_ = x.Store.SetSetting(ctx, "server_address", "10.0.0.5")
	if _, err := c.IssueIP(ctx); err == nil {
		t.Fatal("private address accepted")
	}
}
