//go:build e2e

package sub_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/certs"
	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/sub"
	"github.com/vellnico/swift/internal/xray"
)

// The end-to-end test sends real traffic: server xray with the generated
// config, clients built from the subscription formats (sing-box is the core
// of Hiddify, xray JSON is what v2rayNG imports). It needs network access,
// xray (make xray) and sing-box ($SWIFT_SINGBOX_BIN or .cache/sing-box).

const probeURL = "https://www.gstatic.com/generate_204"

func freeTCP(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func freeUDP(t *testing.T) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

func singBoxBinary(t *testing.T) string {
	if p := os.Getenv("SWIFT_SINGBOX_BIN"); p != "" {
		return p
	}
	x := xray.TestBinary()
	if x == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	p := filepath.Join(filepath.Dir(filepath.Dir(x)), "sing-box", "sing-box")
	if _, err := os.Stat(p); err != nil {
		t.Fatal("sing-box binary not found: run `make e2e`")
	}
	return p
}

// start runs a process until the test ends and keeps its output for failures.
func start(t *testing.T, name string, args ...string) *strings.Builder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	out := &strings.Builder{}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})
	return out
}

// fetch requests probeURL through an HTTP proxy, retrying while the client starts.
func fetch(proxyPort int) error {
	proxy, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
	c := &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}}
	var last error
	for deadline := time.Now().Add(25 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		res, err := c.Get(probeURL)
		if err != nil {
			last = err
			continue
		}
		res.Body.Close()
		if res.StatusCode == http.StatusNoContent {
			return nil
		}
		last = fmt.Errorf("status %d", res.StatusCode)
	}
	return last
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTrafficThroughEveryProfile(t *testing.T) {
	xrayBin := xray.TestBinary()
	if xrayBin == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	singBox := singBoxBinary(t)
	ctx := context.Background()
	dir := t.TempDir()

	sni, err := xray.PickSNI(ctx, xray.TLSProber{Timeout: 3 * time.Second}, xray.SNICandidates)
	if err != nil {
		t.Fatal(err)
	}
	priv, pub, err := xray.RealityKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	reality := func(fp string) xray.Params {
		return xray.Params{PrivateKey: priv, PublicKey: pub, ShortIDs: xray.ShortIDs(2), Target: sni + ":443", ServerNames: []string{sni}, Fingerprint: fp}
	}
	cn := certs.RandomCN()
	is, err := certs.SelfSigned(cn, net.ParseIP("127.0.0.1"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath, err := certs.WriteFiles(dir, "server", is)
	if err != nil {
		t.Fatal(err)
	}

	xhttp := reality(xray.DefaultFingerprint)
	xhttp.Path, xhttp.Mode = "/e2e", "auto"
	profiles := []xray.Profile{
		{ID: 1, Name: "VLESS Reality", Kind: xray.KindVision, Port: freeTCP(t), Enabled: true, Params: reality(xray.DefaultFingerprint), Position: 1},
		{ID: 2, Name: "VLESS XHTTP", Kind: xray.KindXHTTP, Port: freeTCP(t), Enabled: true, Params: xhttp, Position: 2},
		{ID: 3, Name: "Hysteria2", Kind: xray.KindHysteria2, Port: freeUDP(t), Enabled: true, Params: xray.Params{ObfsPassword: "e2e-obfs-password"}, Position: 3},
		{ID: 4, Name: "Hysteria2 plain", Kind: xray.KindHysteria2, Port: freeUDP(t), Enabled: true, Position: 4},
	}
	user := xray.User{ID: 1, Name: "E2E", VlessUUID: "6f1b0b8e-6d1f-4a7e-9a52-1b2f3c4d5e6f", Hy2Password: "e2e-password-0123456789", Enabled: true}

	cfg, err := xray.Generate(xray.Input{APIPort: freeTCP(t), Profiles: profiles, Users: []xray.User{user}, Cert: &xray.CertFiles{CertFile: certPath, KeyFile: keyPath}, LogLevel: "info"})
	if err != nil {
		t.Fatal(err)
	}
	serverCfg := filepath.Join(dir, "server.json")
	if err := os.WriteFile(serverCfg, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	serverLog := start(t, xrayBin, "run", "-c", serverCfg)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("server xray log:\n%s", serverLog)
		}
	})

	tls := links.TLS{ServerName: is.CN, PinSHA256: is.SHA256Hex, CertPEM: string(is.CertPEM)}
	req := sub.Request{User: user}
	for _, p := range profiles {
		e := links.Endpoint{Host: "127.0.0.1", Profile: p}
		if p.Kind == xray.KindHysteria2 {
			e.TLS = tls
		}
		req.Endpoints = append(req.Endpoints, e)
	}

	t.Run("sing-box", func(t *testing.T) {
		raw, err := sub.SingBox(req)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range profiles {
			if p.Kind == xray.KindXHTTP {
				continue // sing-box has no XHTTP; the subscription leaves it out
			}
			t.Run(p.Name, func(t *testing.T) {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					t.Fatal(err)
				}
				port := freeTCP(t)
				c["inbounds"] = []any{map[string]any{"type": "mixed", "tag": "in", "listen": "127.0.0.1", "listen_port": port}}
				c["route"].(map[string]any)["final"] = links.Name(p)
				c["log"] = map[string]any{"level": "debug"}
				path := filepath.Join(t.TempDir(), "client.json")
				writeJSON(t, path, c)
				out := start(t, singBox, "run", "-c", path)
				if err := fetch(port); err != nil {
					t.Fatalf("no traffic through %s: %v\nsing-box log:\n%s", p.Name, err, out)
				}
			})
		}
	})

	// auto-select: the first profile is dead (nothing listens on its port), clients must move on
	dead := profiles[0]
	dead.ID, dead.Name, dead.Port = 9, "Dead", freeTCP(t)
	auto := sub.Request{User: user, AutoSelect: true, Endpoints: append([]links.Endpoint{{Host: "127.0.0.1", Profile: dead}}, req.Endpoints...)}
	t.Run("auto-select", func(t *testing.T) {
		raw, err := sub.SingBox(auto)
		if err != nil {
			t.Fatal(err)
		}
		var c map[string]any
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatal(err)
		}
		port := freeTCP(t)
		c["inbounds"] = []any{map[string]any{"type": "mixed", "tag": "in", "listen": "127.0.0.1", "listen_port": port}}
		path := filepath.Join(t.TempDir(), "sing-box.json")
		writeJSON(t, path, c)
		out := start(t, singBox, "run", "-c", path)
		if err := fetch(port); err != nil {
			t.Fatalf("sing-box urltest did not leave the dead profile: %v
%s", err, out)
		}

		raw, err = sub.XrayJSON(auto)
		if err != nil {
			t.Fatal(err)
		}
		var configs []map[string]any
		if err := json.Unmarshal(raw, &configs); err != nil {
			t.Fatal(err)
		}
		x := configs[0]
		if x["remarks"] != sub.AutoName {
			t.Fatalf("first config is %v", x["remarks"])
		}
		httpPort := freeTCP(t)
		ins := x["inbounds"].([]any)
		ins[0].(map[string]any)["port"] = freeTCP(t)
		ins[1].(map[string]any)["port"] = httpPort
		path = filepath.Join(t.TempDir(), "xray-auto.json")
		writeJSON(t, path, x)
		out = start(t, xrayBin, "run", "-c", path)
		if err := fetch(httpPort); err != nil {
			t.Fatalf("xray balancer did not leave the dead profile: %v
%s", err, out)
		}
	})

	t.Run("xray", func(t *testing.T) {
		raw, err := sub.XrayJSON(req)
		if err != nil {
			t.Fatal(err)
		}
		var configs []map[string]any
		if err := json.Unmarshal(raw, &configs); err != nil {
			t.Fatal(err)
		}
		for i, c := range configs {
			p := profiles[i]
			t.Run(p.Name, func(t *testing.T) {
				httpPort := freeTCP(t)
				ins := c["inbounds"].([]any)
				ins[0].(map[string]any)["port"] = freeTCP(t)
				ins[1].(map[string]any)["port"] = httpPort
				c["log"] = map[string]any{"loglevel": "debug"}
				path := filepath.Join(t.TempDir(), "client.json")
				writeJSON(t, path, c)
				out := start(t, xrayBin, "run", "-c", path)
				if err := fetch(httpPort); err != nil {
					t.Fatalf("no traffic through %s: %v\nxray client log:\n%s", p.Name, err, out)
				}
			})
		}
	})
}
