package certs

import (
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
)

// Holder serves the current certificate to TLS listeners and can be
// reloaded at any time without restarting them (SPEC 9).
type Holder struct {
	mu   sync.RWMutex
	cert *tls.Certificate
}

func (h *Holder) Load(certPath, keyPath string) error {
	c, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("load certificate: %w", err)
	}
	h.mu.Lock()
	h.cert = &c
	h.mu.Unlock()
	return nil
}

var ErrNoCertificate = errors.New("no certificate loaded")

func (h *Holder) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.cert == nil {
		return nil, ErrNoCertificate
	}
	return h.cert, nil
}

// TLSConfig is the server config used by the panel and the subscription server.
func (h *Holder) TLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: h.GetCertificate}
}
