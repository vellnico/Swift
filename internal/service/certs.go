package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/vellnico/swift/internal/certs"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

// Certs manages the server certificate shared by the panel, subscriptions and Hysteria2.
type Certs struct {
	Store *store.Store
	Dir   string // e.g. /var/lib/swift/certs
	Now   func() time.Time
}

// Ensure returns the active certificate, issuing a self-signed one if none exists.
func (c *Certs) Ensure(ctx context.Context) (store.Certificate, error) {
	cur, err := c.Store.Certificate(ctx)
	if err == nil {
		return cur, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return cur, err
	}
	return c.IssueSelfSigned(ctx)
}

func (c *Certs) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// IssueSelfSigned replaces the certificate with a new self-signed one for the server IP.
func (c *Certs) IssueSelfSigned(ctx context.Context) (store.Certificate, error) {
	var ip net.IP
	if v, err := c.Store.Setting(ctx, "server_address"); err == nil {
		ip = net.ParseIP(v)
	}
	is, err := certs.SelfSigned(certs.RandomCN(), ip, c.now())
	if err != nil {
		return store.Certificate{}, err
	}
	certPath, keyPath, err := certs.WriteFiles(c.Dir, "server", is)
	if err != nil {
		return store.Certificate{}, fmt.Errorf("write certificate: %w", err)
	}
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	return c.Store.SaveCertificate(ctx, store.Certificate{
		Kind: "self_signed", Domain: is.CN, CertPath: abs(certPath), KeyPath: abs(keyPath),
		SHA256Hex: is.SHA256Hex, NotAfter: is.NotAfter,
	})
}

// Files is the xray.Input certificate hook.
func (c *Certs) Files(ctx context.Context) (*xray.CertFiles, error) {
	cur, err := c.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	return &xray.CertFiles{CertFile: cur.CertPath, KeyFile: cur.KeyPath}, nil
}
