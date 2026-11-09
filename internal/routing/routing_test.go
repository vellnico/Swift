package routing

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHappHeaderIsBase64JSON(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(RUDirect{}.HappHeader())
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p["GlobalProxy"] != "true" || p["Geoipurl"] != GeoIPURL || p["Geositeurl"] != GeoSiteURL {
		t.Fatalf("profile %v", p)
	}
	if !strings.Contains(string(raw), "geoip:ru") || !strings.Contains(string(raw), "geosite:category-ru") {
		t.Fatalf("direct lists missing: %s", raw)
	}
}

func TestSingBoxAndXrayHooks(t *testing.T) {
	sb := map[string]any{"route": map[string]any{"final": "proxy"}}
	RUDirect{}.SingBox(sb)
	route := sb["route"].(map[string]any)
	if route["final"] != "proxy" || len(route["rule_set"].([]any)) != 3 || sb["dns"] == nil {
		t.Fatalf("sing-box: %v", sb)
	}
	x := map[string]any{}
	RUDirect{}.Xray(x)
	b, _ := json.Marshal(x)
	if !strings.Contains(string(b), `"geoip:ru"`) || strings.Contains(string(b), "ru-available-only-inside") {
		t.Fatalf("xray routing must only use stock categories: %s", b)
	}
}

func TestGeoUpdaterVerifiesChecksums(t *testing.T) {
	good := []byte("geoip-bytes")
	sum := sha256.Sum256(good)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/geoip.dat":
			_, _ = w.Write(good)
		case "/geoip.dat.sha256sum":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  geoip.dat\n"))
		case "/geosite.dat":
			_, _ = w.Write([]byte("tampered"))
		case "/geosite.dat.sha256sum":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  geosite.dat\n"))
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "geosite.dat"), []byte("old"), 0o644)
	var events []string
	g := &GeoUpdater{Dir: dir, Base: srv.URL + "/", Event: func(_, key string, _ map[string]any) { events = append(events, key) }}
	if err := g.Update(context.Background()); err == nil {
		t.Fatal("tampered geosite must fail")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "geoip.dat")); string(b) != "geoip-bytes" {
		t.Fatal("verified file not installed")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "geosite.dat")); string(b) != "old" {
		t.Fatal("unverified file replaced the old one")
	}
	if len(events) != 1 || events[0] != "geo.update_failed" {
		t.Fatalf("events %v", events)
	}
}
