package sub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/routing"
	"github.com/vellnico/swift/internal/xray"
)

var update = flag.Bool("update", false, "rewrite golden files")

const testCertPEM = "-----BEGIN CERTIFICATE-----\nMIIBszCCAVmgAwIBAgIQTEST\n-----END CERTIFICATE-----\n"

var (
	user = xray.User{ID: 1, Name: "Mom", VlessUUID: "6f1b0b8e-6d1f-4a7e-9a52-1b2f3c4d5e6f", Hy2Password: "R7mKp2VnX9qT4wZs8LcY1bFd", Enabled: true}
	tls  = links.TLS{ServerName: "media-3.fastedge.io", PinSHA256: "3f2a9c1e7b5d4f8a0c6e2b9d1f7a3c5e8b0d2f4a6c9e1b3d5f7a9c0e2b4d6f8a", CertPEM: testCertPEM}
	eps  = []links.Endpoint{
		{Host: "203.0.113.7", Profile: xray.Profile{ID: 1, Name: "VLESS Reality", Kind: xray.KindVision, Port: 443, Enabled: true, Params: xray.Params{
			PublicKey: "Zk3iT0P0bRx6Qv2nC8yL1uWm4sJ7hAeD9fGqXo5tNcI", ShortIDs: []string{"a1b2c3d4e5f60718"}, ServerNames: []string{"www.samsung.com"}, Fingerprint: "chrome"}}},
		{Host: "203.0.113.7", Profile: xray.Profile{ID: 2, Name: "VLESS XHTTP", Kind: xray.KindXHTTP, Port: 8443, Enabled: true, Params: xray.Params{
			PublicKey: "Zk3iT0P0bRx6Qv2nC8yL1uWm4sJ7hAeD9fGqXo5tNcI", ShortIDs: []string{"9988776655443322"}, ServerNames: []string{"www.samsung.com"}, Fingerprint: "chrome", Path: "/k3j9x2m4", Mode: "auto"}}},
		{Host: "203.0.113.7", Profile: xray.Profile{ID: 3, Name: "Hysteria2", Kind: xray.KindHysteria2, Port: 443, Enabled: true, Params: xray.Params{ObfsPassword: "Zt7Qw2Lp9Xk4Vn6Rb8Mc"}}, TLS: tls},
	}
)

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
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
	if strings.ReplaceAll(string(want), "\r\n", "\n") != string(got) {
		t.Fatalf("%s differs from golden; run make golden and review\n%s", path, got)
	}
}

func disabled() Request {
	u := user
	u.Enabled = false
	return Request{User: u, Endpoints: eps}
}

func TestBase64(t *testing.T) {
	b, err := Base64(Request{User: user, Endpoints: eps})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "base64.txt", b)
	raw, _ := base64.StdEncoding.DecodeString(string(b))
	if n := len(strings.Split(string(raw), "\n")); n != 3 {
		t.Fatalf("%d links", n)
	}
	empty, _ := Base64(disabled())
	if string(empty) != "" {
		t.Fatalf("disabled user got %q", empty)
	}
}

func TestSingBox(t *testing.T) {
	b, err := SingBox(Request{User: user, Endpoints: eps})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "singbox.json", b)
	if strings.Contains(string(b), "xhttp") {
		t.Fatal("sing-box has no XHTTP transport; the profile must be skipped")
	}
	off, _ := SingBox(disabled())
	golden(t, "singbox_disabled.json", off)
	var v map[string]any
	if err := json.Unmarshal(off, &v); err != nil || v["outbounds"] == nil {
		t.Fatalf("disabled config must be valid JSON with outbounds: %v", err)
	}
}

func TestXrayJSON(t *testing.T) {
	b, err := XrayJSON(Request{User: user, Endpoints: eps})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "xray.json", b)
	var configs []json.RawMessage
	if err := json.Unmarshal(b, &configs); err != nil || len(configs) != 3 {
		t.Fatalf("expected 3 configs: %v", err)
	}
	bin := xray.TestBinary()
	if bin == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	for i, c := range configs {
		p := filepath.Join(t.TempDir(), "client.json")
		if err := os.WriteFile(p, c, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := (xray.ExecRunner{}).Test(context.Background(), bin, p); err != nil {
			t.Fatalf("client config %d rejected by xray: %v", i, err)
		}
	}
	off, _ := XrayJSON(disabled())
	if strings.TrimSpace(string(off)) != "[]" {
		t.Fatalf("disabled user: %s", off)
	}
}

func TestPresetGolden(t *testing.T) {
	req := Request{User: user, Endpoints: eps, Routing: routing.RUDirect{}}
	sb, err := SingBox(req)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "singbox_ru_direct.json", sb)
	xj, err := XrayJSON(req)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "xray_ru_direct.json", xj)
	var configs []json.RawMessage
	if err := json.Unmarshal(xj, &configs); err != nil {
		t.Fatal(err)
	}
	bin := xray.TestBinary()
	if bin == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	for i, c := range configs {
		p := filepath.Join(t.TempDir(), "client.json")
		if err := os.WriteFile(p, c, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := (xray.ExecRunner{}).Test(context.Background(), bin, p); err != nil {
			t.Fatalf("client config %d with the preset rejected by xray: %v", i, err)
		}
	}
	// without the preset everything goes through the VPN
	plain, _ := SingBox(Request{User: user, Endpoints: eps})
	if strings.Contains(string(plain), "rule_set") || strings.Contains(string(plain), "dns-direct") {
		t.Fatal("preset rules leaked into the plain config")
	}
}

func TestDetect(t *testing.T) {
	cases := map[string]Format{
		"HiddifyNext/2.5.7 (android) like ClashMeta v2ray sing-box": FormatSingBox,
		"SFA/1.11.4 (Android 14)":                                   FormatSingBox,
		"sing-box 1.12.0":                                           FormatSingBox,
		"Happ/2.8.1":                                                FormatBase64,
		"v2rayNG/1.10.16":                                           FormatXray,
		"v2RayTun/5.1 (iOS)":                                        FormatBase64,
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0) Safari/605.1.15": FormatHTML,
		"curl/8.5.0": FormatBase64,
		"":           FormatBase64,
	}
	for ua, want := range cases {
		if got := Detect(ua, ""); got != want {
			t.Errorf("%q: got %s want %s", ua, got, want)
		}
	}
	if Detect("v2rayNG/1.10", "singbox") != FormatSingBox || Detect("Mozilla/5.0", "XRAY") != FormatXray {
		t.Error("?format= must win over the User-Agent")
	}
	if Detect("Mozilla/5.0", "nonsense") != FormatHTML {
		t.Error("unknown ?format= falls back to detection")
	}
	if Client("Happ/2.8.1") != "happ" || Client("curl") != "" {
		t.Error("client name")
	}
}

func TestAutoSelect(t *testing.T) {
	req := Request{User: user, Endpoints: eps, AutoSelect: true, Routing: routing.RUDirect{}}
	sb, err := SingBox(req)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "singbox_auto.json", sb)
	if !strings.Contains(string(sb), `"urltest"`) {
		t.Fatal("no urltest with auto-select")
	}
	plain, _ := SingBox(Request{User: user, Endpoints: eps})
	if strings.Contains(string(plain), `"urltest"`) {
		t.Fatal("urltest without auto-select")
	}

	xj, err := XrayJSON(req)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "xray_auto.json", xj)
	var configs []map[string]any
	if err := json.Unmarshal(xj, &configs); err != nil || len(configs) != 4 || configs[0]["remarks"] != AutoName {
		t.Fatalf("want the Auto config first and then 3 profiles: %v", err)
	}
	rules := configs[0]["routing"].(map[string]any)["rules"].([]any)
	if last := rules[len(rules)-1].(map[string]any); last["balancerTag"] != "auto" || len(rules) < 3 {
		t.Fatalf("balancer must come after the preset rules: %v", rules)
	}
	bin := xray.TestBinary()
	if bin == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	raw, _ := json.Marshal(configs[0])
	p := filepath.Join(t.TempDir(), "auto.json")
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (xray.ExecRunner{}).Test(context.Background(), bin, p); err != nil {
		t.Fatalf("Auto config rejected by xray: %v", err)
	}

	// one profile: nothing to choose from
	one, _ := XrayJSON(Request{User: user, Endpoints: eps[:1], AutoSelect: true})
	if strings.Contains(string(one), AutoName) {
		t.Fatal("Auto config with a single profile")
	}
}
