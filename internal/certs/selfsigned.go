// Package certs issues and stores the TLS certificates used by the panel,
// subscriptions and Hysteria2.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// plausible host names for the self-signed certificate's CN
var cnWords = []string{"cdn", "static", "media", "assets", "img", "edge", "files", "api"}
var cnDomains = []string{"cloudfront-cache.net", "storage-nodes.com", "fastedge.io", "content-delivery.org", "mediacache.net"}

func randIndex(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}

// RandomCN returns a believable host name such as "media-3.fastedge.io".
func RandomCN() string {
	return fmt.Sprintf("%s-%d.%s", cnWords[randIndex(len(cnWords))], 1+randIndex(9), cnDomains[randIndex(len(cnDomains))])
}

type Issued struct {
	CertPEM   []byte
	KeyPEM    []byte
	SHA256Hex string // fingerprint of the leaf certificate
	NotAfter  time.Time
	CN        string
}

// SelfSigned issues an ECDSA P-256 certificate valid for 10 years.
// ip, if set, is added as a SAN so clients connecting by address accept it.
func SelfSigned(cn string, ip net.IP, now time.Time) (Issued, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Issued{}, fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return Issued{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              []string{cn},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return Issued{}, fmt.Errorf("create certificate: %w", err)
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return Issued{}, err
	}
	return Issued{
		CertPEM:   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		KeyPEM:    pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}),
		SHA256Hex: FingerprintHex(der),
		NotAfter:  tmpl.NotAfter,
		CN:        cn,
	}, nil
}

// FingerprintHex is the lowercase hex SHA-256 of a DER certificate (pinSHA256 in links).
func FingerprintHex(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// LeafFingerprint reads a PEM file and returns the hex SHA-256 of its first certificate.
func LeafFingerprint(certPEM []byte) (string, time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", time.Time{}, fmt.Errorf("no PEM certificate")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", time.Time{}, err
	}
	return FingerprintHex(block.Bytes), c.NotAfter, nil
}

// WriteFiles stores cert and key atomically under dir with the given base name.
func WriteFiles(dir, base string, is Issued) (certPath, keyPath string, err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	certPath = filepath.Join(dir, base+".crt")
	keyPath = filepath.Join(dir, base+".key")
	for _, f := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{{keyPath, is.KeyPEM, 0o600}, {certPath, is.CertPEM, 0o644}} {
		tmp := f.path + ".tmp"
		if err := os.WriteFile(tmp, f.data, f.mode); err != nil {
			return "", "", err
		}
		if err := os.Rename(tmp, f.path); err != nil {
			return "", "", err
		}
	}
	return certPath, keyPath, nil
}
