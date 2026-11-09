package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vellnico/swift/internal/certs"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

// Certs manages the server certificate shared by the panel, subscriptions and Hysteria2.
type Certs struct {
	Store  *store.Store
	Dir    string // e.g. /var/lib/swift/certs
	Issuer certs.Issuer
	Holder *certs.Holder
	// Apply restarts xray with the new certificate; nil in unit tests of this file.
	Apply func(ctx context.Context) error
	Event EventFunc
	Now   func() time.Time
	// Resolve looks up a domain; replaced in tests.
	Resolve func(ctx context.Context, host string) ([]string, error)
}

const RenewBefore = 30 * 24 * time.Hour

func (c *Certs) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Certs) event(level, key string, params map[string]any) {
	if c.Event != nil {
		c.Event(level, "certs", key, params)
	}
}

// Ensure returns the active certificate, issuing a self-signed one if none exists,
// and makes sure the TLS listeners serve it.
func (c *Certs) Ensure(ctx context.Context) (store.Certificate, error) {
	cur, err := c.Store.Certificate(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return c.selfSigned(ctx)
	}
	if err != nil {
		return cur, err
	}
	if c.Holder != nil {
		if _, herr := c.Holder.GetCertificate(nil); herr != nil {
			if err := c.Holder.Load(cur.CertPath, cur.KeyPath); err != nil {
				return cur, err
			}
		}
	}
	return cur, nil
}

func (c *Certs) install(ctx context.Context, kind, domain string, is certs.Issued) (store.Certificate, error) {
	certPath, keyPath, err := certs.WriteFiles(c.Dir, "server", is)
	if err != nil {
		return store.Certificate{}, fmt.Errorf("write certificate: %w", err)
	}
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	saved, err := c.Store.SaveCertificate(ctx, store.Certificate{
		Kind: kind, Domain: domain, CertPath: abs(certPath), KeyPath: abs(keyPath), SHA256Hex: is.SHA256Hex, NotAfter: is.NotAfter,
	})
	if err != nil {
		return saved, err
	}
	if c.Holder != nil {
		if err := c.Holder.Load(saved.CertPath, saved.KeyPath); err != nil {
			return saved, err
		}
	}
	return saved, nil
}

func (c *Certs) selfSigned(ctx context.Context) (store.Certificate, error) {
	var ip net.IP
	if v, err := c.Store.Setting(ctx, "server_address"); err == nil {
		ip = net.ParseIP(v)
	}
	is, err := certs.SelfSigned(certs.RandomCN(), ip, c.now())
	if err != nil {
		return store.Certificate{}, err
	}
	return c.install(ctx, "self_signed", is.CN, is)
}

// IssueSelfSigned switches to a new self-signed certificate (IP mode) and reloads xray.
func (c *Certs) IssueSelfSigned(ctx context.Context) (store.Certificate, error) {
	saved, err := c.selfSigned(ctx)
	if err != nil {
		return saved, err
	}
	if err := c.Store.SetSetting(ctx, "domain", ""); err != nil {
		return saved, err
	}
	c.event("info", "certs.self_signed", nil)
	return saved, c.apply(ctx)
}

func (c *Certs) apply(ctx context.Context) error {
	if c.Apply == nil {
		return nil
	}
	return c.Apply(ctx)
}

var domainRe = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

// IssueACME gets a Let's Encrypt certificate for domain (domain mode) and reloads xray.
func (c *Certs) IssueACME(ctx context.Context, domain string) (store.Certificate, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !domainRe.MatchString(domain) || net.ParseIP(domain) != nil {
		return store.Certificate{}, &ValidationError{Fields: map[string]string{"domain": "invalid"}}
	}
	if err := c.checkDNS(ctx, domain); err != nil {
		return store.Certificate{}, err
	}
	if c.Issuer == nil {
		return store.Certificate{}, errors.New("no ACME issuer configured")
	}
	is, err := c.Issuer.Obtain(ctx, domain)
	if err != nil {
		c.event("error", "certs.acme_failed", map[string]any{"domain": domain, "error": err.Error()})
		return store.Certificate{}, &IssueError{Detail: err.Error()}
	}
	saved, err := c.install(ctx, "acme", domain, is)
	if err != nil {
		return saved, err
	}
	if err := c.Store.SetSetting(ctx, "domain", domain); err != nil {
		return saved, err
	}
	c.event("info", "certs.issued", map[string]any{"domain": domain})
	return saved, c.apply(ctx)
}

// IssueError carries the ACME failure for the UI.
type IssueError struct{ Detail string }

func (e *IssueError) Error() string { return "certificate not issued: " + e.Detail }

// checkDNS verifies the domain points at this server when its address is known.
func (c *Certs) checkDNS(ctx context.Context, domain string) error {
	addr, err := c.Store.Setting(ctx, "server_address")
	if err != nil || addr == "" {
		return nil
	}
	resolve := c.Resolve
	if resolve == nil {
		resolve = net.DefaultResolver.LookupHost
	}
	ips, err := resolve(ctx, domain)
	if err != nil {
		return &ValidationError{Fields: map[string]string{"domain": "dns_not_found"}}
	}
	if !slices.Contains(ips, addr) {
		return &ValidationError{Fields: map[string]string{"domain": "dns_mismatch"}, Detail: map[string]string{"ips": strings.Join(ips, ", "), "server": addr}}
	}
	return nil
}

// RenewIfNeeded renews an ACME certificate that expires within 30 days (or always
// when force), reloads the panel's certificate in place and restarts xray.
func (c *Certs) RenewIfNeeded(ctx context.Context, force bool) (bool, error) {
	cur, err := c.Store.Certificate(ctx)
	if err != nil {
		return false, err
	}
	if cur.Kind != "acme" {
		return false, nil
	}
	if !force && cur.NotAfter.Sub(c.now()) > RenewBefore {
		return false, nil
	}
	is, err := c.Issuer.Obtain(ctx, cur.Domain)
	if err != nil {
		_ = c.Store.SetCertificateError(ctx, err.Error())
		c.event("error", "certs.renew_failed", map[string]any{"domain": cur.Domain, "error": err.Error()})
		return false, &IssueError{Detail: err.Error()}
	}
	if _, err := c.install(ctx, "acme", cur.Domain, is); err != nil {
		return false, err
	}
	c.event("info", "certs.renewed", map[string]any{"domain": cur.Domain})
	return true, c.apply(ctx)
}

// RunRenewal checks the certificate once a day until ctx ends.
func (c *Certs) RunRenewal(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		_, _ = c.RenewIfNeeded(ctx, false)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Files is the xray.Input certificate hook.
func (c *Certs) Files(ctx context.Context) (*xray.CertFiles, error) {
	cur, err := c.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	return &xray.CertFiles{CertFile: cur.CertPath, KeyFile: cur.KeyPath}, nil
}

// Scheme is https unless the panel deliberately runs without TLS (local development).
func Scheme(insecureHTTP bool) string {
	if insecureHTTP {
		return "http"
	}
	return "https"
}
