package xray

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// SNICandidates are Reality targets tried in order. Large sites with TLS 1.3,
// X25519 and HTTP/2 that are not on Xray's "likely to get you blocked" list.
var SNICandidates = []string{
	"www.samsung.com",
	"www.nvidia.com",
	"www.amd.com",
	"www.asus.com",
	"www.dell.com",
	"www.intel.com",
	"dl.google.com",
	"addons.mozilla.org",
	"www.lovelive-anime.jp",
	"www.yahoo.com",
}

// Prober checks whether a host is a usable Reality target from this server.
type Prober interface {
	Probe(ctx context.Context, host string) error
}

// TLSProber requires TLS 1.3, the X25519 key exchange and ALPN h2 within the timeout.
type TLSProber struct {
	Timeout time.Duration
}

func (p TLSProber) Probe(ctx context.Context, host string) error {
	timeout := p.Timeout
	if timeout == 0 {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	d := tls.Dialer{Config: &tls.Config{
		ServerName:       host,
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519},
		NextProtos:       []string{"h2"},
	}}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return fmt.Errorf("%s: %w", host, err)
	}
	defer conn.Close()
	cs := conn.(*tls.Conn).ConnectionState()
	if cs.Version != tls.VersionTLS13 {
		return fmt.Errorf("%s: not TLS 1.3", host)
	}
	if cs.NegotiatedProtocol != "h2" {
		return fmt.Errorf("%s: no HTTP/2", host)
	}
	if cs.CurveID != tls.X25519 {
		return fmt.Errorf("%s: key exchange is not X25519", host)
	}
	return nil
}

// PickSNI returns the first candidate that passes the probe.
func PickSNI(ctx context.Context, p Prober, candidates []string) (string, error) {
	var last error
	for _, host := range candidates {
		if err := p.Probe(ctx, host); err != nil {
			last = err
			continue
		}
		return host, nil
	}
	return "", fmt.Errorf("no suitable Reality target found: %w", last)
}
