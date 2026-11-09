package web

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

func subEnv(t *testing.T) (*SubServer, store.User, store.User) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	_ = st.SetSetting(ctx, "sub_port", "31000")
	_ = st.SetSetting(ctx, "server_address", "203.0.113.7")
	priv, pub, _ := xray.RealityKeyPair()
	if _, err := st.CreateProfile(ctx, xray.Profile{Name: "VLESS Reality", Kind: xray.KindVision, Port: 443, Enabled: true, Params: xray.Params{
		PrivateKey: priv, PublicKey: pub, ShortIDs: []string{"aabb"}, Target: "www.samsung.com:443", ServerNames: []string{"www.samsung.com"}, Fingerprint: "chrome",
	}}); err != nil {
		t.Fatal(err)
	}
	on, _ := st.CreateUser(ctx, store.User{Name: "Mom", SubToken: "tok-on-aaaaaaaaaaaaaaaaaaaaaaaaaaaa", VlessUUID: "6f1b0b8e-6d1f-4a7e-9a52-1b2f3c4d5e6f", Hy2Password: "p", Enabled: true})
	off, _ := st.CreateUser(ctx, store.User{Name: "Off <b>", SubToken: "tok-off-aaaaaaaaaaaaaaaaaaaaaaaaaaa", VlessUUID: "0a9c8b7d-5e4f-4321-8abc-def012345678", Hy2Password: "q", Enabled: false})
	s := &SubServer{
		Store:  st,
		Subs:   &service.Subscriptions{Store: st},
		Assets: fstest.MapFS{"index.html": {Data: []byte(`<!doctype html><html lang="ru"><head><!--swift-data--></head></html>`)}, "assets/a.js": {Data: []byte("x")}},
		Dict:   func(lang string) map[string]string { return map[string]string{"sub.copy": lang, "nav.users": "x"} },
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return s, on, off
}

func subGet(h http.Handler, path, ua string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("User-Agent", ua)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSubscriptionFormatsAndHeaders(t *testing.T) {
	s, on, _ := subEnv(t)
	h := s.Handler()
	path := "/s/" + on.SubToken

	rec := subGet(h, path, "v2RayTun/5.1", nil)
	raw, _ := base64.StdEncoding.DecodeString(rec.Body.String())
	if rec.Code != 200 || !strings.HasPrefix(string(raw), "vless://"+on.VlessUUID+"@203.0.113.7:443") {
		t.Fatalf("base64: %d %q", rec.Code, raw)
	}
	for k, want := range map[string]string{
		"profile-title":           "base64:" + base64.StdEncoding.EncodeToString([]byte("Swift")),
		"profile-update-interval": "12",
		"Content-Disposition":     `attachment; filename="swift"`,
		"profile-web-page-url":    "http://203.0.113.7:31000/s/" + on.SubToken + "?format=html",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if rec.Header().Get("subscription-userinfo") != "" {
		t.Error("subscription-userinfo must not be sent")
	}

	if rec := subGet(h, path, "HiddifyNext/2.5", nil); !strings.Contains(rec.Body.String(), `"type": "vless"`) {
		t.Fatalf("sing-box for Hiddify: %s", rec.Body.String())
	}
	if rec := subGet(h, path, "v2rayNG/1.10", nil); !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "[") {
		t.Fatalf("xray json for v2rayNG: %s", rec.Body.String())
	}
	if rec := subGet(h, path+"?format=base64", "HiddifyNext/2.5", nil); strings.HasPrefix(rec.Body.String(), "{") {
		t.Fatal("?format= must override the User-Agent")
	}
	if rec := subGet(h, path+"?format=qr", "curl", nil); rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatal("qr format")
	}
}

func TestSubscriptionHTMLPage(t *testing.T) {
	s, on, off := subEnv(t)
	h := s.Handler()
	rec := subGet(h, "/s/"+on.SubToken, "Mozilla/5.0 (iPhone)", map[string]string{"Accept-Language": "en-US,en;q=0.9"})
	body := rec.Body.String()
	if !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, `"sub.copy":"en"`) || strings.Contains(body, "nav.users") {
		t.Fatalf("page: %s", body)
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatal("csp")
	}
	rec = subGet(h, "/s/"+off.SubToken, "Mozilla/5.0", map[string]string{"Accept-Language": "ru"})
	if strings.Contains(rec.Body.String(), "<b>") {
		t.Fatal("user name must be escaped inside the JSON block")
	}
	if PageLang("de-DE,ru;q=0.8") != "ru" || PageLang("fr") != "en" {
		t.Fatal("language detection")
	}
}

func TestDisabledAndUnknown(t *testing.T) {
	s, _, off := subEnv(t)
	h := s.Handler()
	rec := subGet(h, "/s/"+off.SubToken, "v2RayTun/5.1", nil)
	if rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("disabled user must get an empty valid subscription: %d %q", rec.Code, rec.Body.String())
	}
	if rec := subGet(h, "/s/"+off.SubToken, "HiddifyNext", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "outbounds") {
		t.Fatalf("disabled sing-box: %d", rec.Code)
	}
	for _, p := range []string{"/s/unknown-token", "/s/", "/", "/s/a/b", "/x/" + off.SubToken} {
		if rec := subGet(h, p, "v2RayTun", nil); rec.Code != 404 || rec.Body.Len() != 0 {
			t.Errorf("%s: %d %q", p, rec.Code, rec.Body.String())
		}
	}
	if rec := subGet(h, "/s/assets/a.js", "Mozilla", nil); rec.Code != 200 {
		t.Fatalf("assets: %d", rec.Code)
	}
}
