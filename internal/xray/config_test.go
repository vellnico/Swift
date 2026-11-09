package xray

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

const (
	testCert = "testdata/cert.pem"
	testKey  = "testdata/key.pem"
)

// ensureTestCert creates a throwaway self-signed certificate for golden configs.
func ensureTestCert(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(testCert); err == nil {
		return
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "swift-test.invalid"},
		DNSNames:     []string{"swift-test.invalid"},
		NotBefore:    time.Unix(1_700_000_000, 0),
		NotAfter:     time.Unix(1_700_000_000, 0).AddDate(30, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kb, _ := x509.MarshalECPrivateKey(key)
	_ = os.MkdirAll("testdata", 0o755)
	_ = os.WriteFile(testCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
	_ = os.WriteFile(testKey, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600)
}

// Fixed test values; the private key is a valid clamped x25519 scalar.
var (
	testPriv = "8Pf3Vq0Lr8HbW2n2RjKq3xS1o9vY6aZcD4eFgHiJk2w"
	users    = []User{
		{ID: 1, Name: "Mom", VlessUUID: "6f1b0b8e-6d1f-4a7e-9a52-1b2f3c4d5e6f", Hy2Password: "R7mKp2VnX9qT4wZs8LcY1bFd", Enabled: true},
		{ID: 2, Name: "Dad", VlessUUID: "0a9c8b7d-5e4f-4321-8abc-def012345678", Hy2Password: "Hq3Nf6TjW8yPz1KxM5vB0sDg", Enabled: true},
		{ID: 3, Name: "Off", VlessUUID: "11111111-2222-4333-8444-555555555555", Hy2Password: "disabled-user-password-x", Enabled: false},
	}
	vision = Profile{ID: 1, Name: "Vision", Kind: KindVision, Port: 443, Enabled: true, Position: 0, Params: Params{
		PrivateKey: testPriv, ShortIDs: []string{"a1b2c3d4e5f60718", "0f1e2d3c4b5a6978", "1122334455667788"},
		Target: "www.microsoft.com:443", ServerNames: []string{"www.microsoft.com"}, Fingerprint: "chrome",
	}}
	xhttp = Profile{ID: 2, Name: "XHTTP", Kind: KindXHTTP, Port: 8443, Enabled: true, Position: 1, Params: Params{
		PrivateKey: testPriv, ShortIDs: []string{"9988776655443322"}, Target: "www.apple.com:443",
		ServerNames: []string{"www.apple.com"}, Fingerprint: "chrome", Path: "/k3j9x2m4", Mode: "auto",
	}}
	hy2 = Profile{ID: 3, Name: "Hysteria2", Kind: KindHysteria2, Port: 443, Enabled: true, Position: 2, Params: Params{
		ObfsPassword: "Zt7Qw2Lp9Xk4Vn6Rb8Mc",
	}}
)

func golden(t *testing.T, name string, in Input) {
	t.Helper()
	got, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", name+".json")
	if *update {
		_ = os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run make golden)", err)
	}
	if !bytes.Equal(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")), got) {
		t.Fatalf("%s differs from golden file; run make golden and review the diff\n%s", path, got)
	}
	checkWithXray(t, path)
}

// checkWithXray runs `xray run -test` on a config; a missing binary fails the test.
func checkWithXray(t *testing.T, path string) {
	t.Helper()
	bin := TestBinary()
	if bin == "" {
		t.Fatal("xray binary not found: run `make xray` (or set SWIFT_XRAY_BIN)")
	}
	// xray resolves relative certificate paths against its asset directory,
	// so test a copy with absolute paths; the golden file stays portable.
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{testCert, testKey} {
		abs, _ := filepath.Abs(rel)
		body = bytes.ReplaceAll(body, []byte(`"`+rel+`"`), []byte(`"`+filepath.ToSlash(abs)+`"`))
	}
	tmp := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (ExecRunner{}).Test(context.Background(), bin, tmp); err != nil {
		t.Fatalf("xray rejected %s: %v", path, err)
	}
}

func certFiles() *CertFiles {
	return &CertFiles{CertFile: testCert, KeyFile: testKey}
}

func TestGoldenVision(t *testing.T) {
	golden(t, "vision", Input{APIPort: 10085, Profiles: []Profile{vision}, Users: users})
}

func TestGoldenXHTTP(t *testing.T) {
	golden(t, "xhttp", Input{APIPort: 10085, Profiles: []Profile{xhttp}, Users: users})
}

func TestGoldenHysteria2(t *testing.T) {
	ensureTestCert(t)
	golden(t, "hysteria2", Input{APIPort: 10085, Profiles: []Profile{hy2}, Users: users, Cert: certFiles()})
}

func TestGoldenAllProfiles(t *testing.T) {
	ensureTestCert(t)
	off := xhttp
	off.ID, off.Enabled, off.Port = 9, false, 2053
	golden(t, "all", Input{APIPort: 10085, Profiles: []Profile{hy2, xhttp, vision, off}, Users: users, Cert: certFiles()})
}

func TestGoldenNoUsers(t *testing.T) {
	ensureTestCert(t)
	golden(t, "empty", Input{APIPort: 10085, Profiles: []Profile{vision, hy2}, Cert: certFiles()})
}

func TestGenerateErrors(t *testing.T) {
	if _, err := Generate(Input{}); err == nil {
		t.Error("missing api port accepted")
	}
	bad := vision
	bad.Params.PrivateKey = ""
	if _, err := Generate(Input{APIPort: 1, Profiles: []Profile{bad}}); err == nil {
		t.Error("incomplete Reality params accepted")
	}
	if _, err := Generate(Input{APIPort: 1, Profiles: []Profile{hy2}}); err == nil {
		t.Error("hysteria2 without certificate accepted")
	}
}

func TestRealityKeys(t *testing.T) {
	priv, pub, err := RealityKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if len(priv) != 43 || len(pub) != 43 {
		t.Fatalf("unexpected key lengths %d %d", len(priv), len(pub))
	}
	derived, err := PublicKeyFor(priv)
	if err != nil || derived != pub {
		t.Fatalf("public key mismatch: %v", err)
	}
	ids := ShortIDs(3)
	if len(ids) != 3 || len(ids[0]) != 16 || ids[0] == ids[1] {
		t.Fatalf("short ids %v", ids)
	}
}
