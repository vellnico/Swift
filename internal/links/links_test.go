package links

import (
	"encoding/base64"
	"encoding/hex"
	"flag"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vellnico/swift/internal/xray"
)

var update = flag.Bool("update", false, "rewrite golden files")

var (
	user   = xray.User{ID: 1, Name: "Mom", VlessUUID: "6f1b0b8e-6d1f-4a7e-9a52-1b2f3c4d5e6f", Hy2Password: "R7mKp2VnX9qT4wZs8LcY1bFd", Enabled: true}
	vision = xray.Profile{ID: 1, Name: "VLESS Reality", Kind: xray.KindVision, Port: 443, Enabled: true, Params: xray.Params{
		PublicKey: "Zk3iT0P0bRx6Qv2nC8yL1uWm4sJ7hAeD9fGqXo5tNcI", ShortIDs: []string{"a1b2c3d4e5f60718"},
		ServerNames: []string{"www.samsung.com"}, Fingerprint: "chrome",
	}}
	xh = xray.Profile{ID: 2, Name: "VLESS XHTTP", Kind: xray.KindXHTTP, Port: 8443, Enabled: true, Params: xray.Params{
		PublicKey: "Zk3iT0P0bRx6Qv2nC8yL1uWm4sJ7hAeD9fGqXo5tNcI", ShortIDs: []string{"9988776655443322"},
		ServerNames: []string{"www.samsung.com"}, Fingerprint: "firefox", Path: "/k3j9x2m4", Mode: "auto",
	}}
	hy2 = xray.Profile{ID: 3, Name: "Hysteria2", Kind: xray.KindHysteria2, Port: 443, Enabled: true, Params: xray.Params{ObfsPassword: "Zt7Qw2Lp9Xk4Vn6Rb8Mc"}}
	pin = "3f2a9c1e7b5d4f8a0c6e2b9d1f7a3c5e8b0d2f4a6c9e1b3d5f7a9c0e2b4d6f8a"
)

func TestLinksGolden(t *testing.T) {
	var lines []string
	for _, e := range []Endpoint{
		{Host: "203.0.113.7", Profile: vision},
		{Host: "203.0.113.7", Profile: xh},
		{Host: "203.0.113.7", Profile: hy2, TLS: TLS{ServerName: "media-3.fastedge.io", PinSHA256: pin}},
		{Host: "vpn.example.com", Profile: hy2, TLS: TLS{ServerName: "vpn.example.com"}},
		{Host: "2001:db8::1", Profile: vision},
	} {
		l, err := Link(e, user)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, l)
	}
	got := strings.Join(lines, "\n") + "\n"
	path := filepath.Join("testdata", "links.txt")
	if *update {
		_ = os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run make golden)", err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != got {
		t.Fatalf("links differ from golden; run make golden and review\n%s", got)
	}
}

// pinSHA256 in links must be lowercase hex: Xray-based clients decode it as hex
// and fail on base64 (SPEC 6.4). Xray-JSON's pinnedPeerCertSha256 is hex too.
func TestPinIsHexInLinks(t *testing.T) {
	l, err := Link(Endpoint{Host: "1.2.3.4", Profile: hy2, TLS: TLS{ServerName: "x.example", PinSHA256: strings.ToUpper(pin)}}, user)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(l)
	if err != nil {
		t.Fatal(err)
	}
	got := u.Query().Get("pinSHA256")
	if got != pin {
		t.Fatalf("pinSHA256 = %q, want lowercase hex %q", got, pin)
	}
	raw, err := hex.DecodeString(got)
	if err != nil || len(raw) != 32 {
		t.Fatalf("pin is not 32 bytes of hex: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(got); err == nil && len(got) == 44 {
		t.Fatal("pin looks like base64")
	}
	if u.Query().Get("insecure") != "0" || u.Query().Get("obfs") != "salamander" {
		t.Fatalf("query %v", u.Query())
	}
}

func TestACMEHasNoPin(t *testing.T) {
	l, _ := Link(Endpoint{Host: "vpn.example.com", Profile: hy2, TLS: TLS{ServerName: "vpn.example.com"}}, user)
	if strings.Contains(l, "pinSHA256") {
		t.Fatal("ACME certificate link must not carry a pin")
	}
}

func TestNameAndEscaping(t *testing.T) {
	p := vision
	p.Name = "Дом & работа"
	l, _ := Link(Endpoint{Host: "1.2.3.4", Profile: p}, user)
	u, _ := url.Parse(l)
	if u.Fragment != "Swift · Дом & работа" {
		t.Fatalf("fragment %q", u.Fragment)
	}
	if strings.Contains(l, "+") {
		t.Fatal("spaces must not be encoded as +")
	}
}
