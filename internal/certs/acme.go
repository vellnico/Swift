package certs

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

// Issuer obtains a publicly trusted certificate for a domain.
type Issuer interface {
	Obtain(ctx context.Context, domain string) (Issued, error)
}

// LegoIssuer talks to Let's Encrypt: HTTP-01 on port 80 first, then
// TLS-ALPN-01 on port 443 if that fails (SPEC 9).
type LegoIssuer struct {
	Dir     string // where the ACME account key lives
	Email   string // optional contact
	Staging bool
	// Ports are overridable for tests behind a proxy; defaults 80 and 443.
	HTTPPort string
	TLSPort  string
}

type acmeUser struct {
	email string
	key   crypto.PrivateKey
	reg   *registration.Resource
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.reg }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func (l *LegoIssuer) accountKey() (crypto.PrivateKey, error) {
	path := filepath.Join(l.Dir, "acme-account.key")
	if b, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(b)
		if block == nil {
			return nil, errors.New("acme account key: bad PEM")
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(l.Dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (l *LegoIssuer) client(user *acmeUser) (*lego.Client, error) {
	cfg := lego.NewConfig(user)
	cfg.Certificate.KeyType = certcrypto.EC256
	if l.Staging {
		cfg.CADirURL = lego.LEDirectoryStaging
	}
	c, err := lego.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("acme client: %w", err)
	}
	return c, nil
}

func (l *LegoIssuer) Obtain(ctx context.Context, domain string) (Issued, error) {
	key, err := l.accountKey()
	if err != nil {
		return Issued{}, fmt.Errorf("acme account key: %w", err)
	}
	user := &acmeUser{email: l.Email, key: key}
	httpPort, tlsPort := l.HTTPPort, l.TLSPort
	if httpPort == "" {
		httpPort = "80"
	}
	if tlsPort == "" {
		tlsPort = "443"
	}

	attempt := func(setup func(*lego.Client) error) (Issued, error) {
		c, err := l.client(user)
		if err != nil {
			return Issued{}, err
		}
		if err := setup(c); err != nil {
			return Issued{}, err
		}
		if user.reg == nil {
			reg, err := c.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
			if err != nil {
				return Issued{}, fmt.Errorf("acme registration: %w", err)
			}
			user.reg = reg
		}
		res, err := c.Certificate.Obtain(certificate.ObtainRequest{Domains: []string{domain}, Bundle: true})
		if err != nil {
			return Issued{}, err
		}
		fp, notAfter, err := LeafFingerprint(res.Certificate)
		if err != nil {
			return Issued{}, err
		}
		return Issued{CertPEM: res.Certificate, KeyPEM: res.PrivateKey, SHA256Hex: fp, NotAfter: notAfter, CN: domain}, nil
	}

	if err := ctx.Err(); err != nil {
		return Issued{}, err
	}
	is, errHTTP := attempt(func(c *lego.Client) error {
		return c.Challenge.SetHTTP01Provider(http01.NewProviderServer("", httpPort))
	})
	if errHTTP == nil {
		return is, nil
	}
	is, errALPN := attempt(func(c *lego.Client) error {
		return c.Challenge.SetTLSALPN01Provider(tlsalpn01.NewProviderServer("", tlsPort))
	})
	if errALPN == nil {
		return is, nil
	}
	return Issued{}, fmt.Errorf("http-01: %v; tls-alpn-01: %w", errHTTP, errALPN)
}
