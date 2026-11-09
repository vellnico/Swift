package certs

import (
	"crypto/tls"
	"net"
	"regexp"
	"testing"
	"time"
)

func TestSelfSigned(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cn := RandomCN()
	if !regexp.MustCompile(`^[a-z]+-\d\.[a-z-]+\.[a-z]+$`).MatchString(cn) {
		t.Fatalf("cn %q", cn)
	}
	is, err := SelfSigned(cn, net.ParseIP("203.0.113.7"), now)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(is.CertPEM, is.KeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if got := FingerprintHex(pair.Certificate[0]); got != is.SHA256Hex || len(got) != 64 {
		t.Fatalf("fingerprint %q vs %q", got, is.SHA256Hex)
	}
	if is.NotAfter.Year() != now.Year()+10 {
		t.Fatalf("not after %v", is.NotAfter)
	}
	fp, na, err := LeafFingerprint(is.CertPEM)
	if err != nil || fp != is.SHA256Hex || !na.Equal(is.NotAfter) {
		t.Fatalf("leaf %q %v %v", fp, na, err)
	}
	dir := t.TempDir()
	cp, kp, err := WriteFiles(dir, "hy2", is)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tls.LoadX509KeyPair(cp, kp); err != nil {
		t.Fatal(err)
	}
}
