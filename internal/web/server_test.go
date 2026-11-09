package web

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/store"
)

var assets = fstest.MapFS{
	"index.html":        {Data: []byte("<title>Swift</title>")},
	"assets/app-1.js":   {Data: []byte("console.log(1)")},
	"fonts/onest.woff2": {Data: []byte("font")},
}

type env struct {
	srv  *Server
	st   *store.Store
	user string
	pass string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	m := auth.NewManager(st)
	m.Params = auth.Params{Memory: 1024, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}
	user, pass, err := m.ResetAdmin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Deps{PanelPath: "abc123", Assets: assets, Store: st, Auth: m})
	return &env{srv: srv, st: st, user: user, pass: pass}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestOutsidePanelPathIsEmpty404(t *testing.T) {
	h := newEnv(t).srv.Handler()
	for _, p := range []string{"/", "/index.html", "/abc12", "/abc1234/", "/api/health", "/api/auth/session", "/../abc123/"} {
		rec := get(t, h, p)
		if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
			t.Errorf("%s: code %d body %q", p, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{}")))
	if rec.Code != 404 || rec.Body.Len() != 0 {
		t.Errorf("POST outside path: %d %q", rec.Code, rec.Body.String())
	}
}

func TestSPAAndFallback(t *testing.T) {
	h := newEnv(t).srv.Handler()
	for _, p := range []string{"/abc123/", "/abc123/users/5", "/abc123/../abc123/x"} {
		rec := get(t, h, p)
		if rec.Code != http.StatusOK || rec.Body.String() != "<title>Swift</title>" {
			t.Errorf("%s: code %d body %q", p, rec.Code, rec.Body.String())
		}
	}
	if rec := get(t, h, "/abc123/assets/app-1.js"); rec.Code != 200 || rec.Header().Get("Cache-Control") == "" {
		t.Errorf("asset: %d %v", rec.Code, rec.Header())
	}
	if rec := get(t, h, "/abc123"); rec.Code != http.StatusMovedPermanently {
		t.Errorf("redirect: %d", rec.Code)
	}
	rec := get(t, h, "/abc123/api/health")
	if rec.Code != 200 || rec.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatalf("health/headers: %d %v", rec.Code, rec.Header())
	}
}

// client drives the API like the browser: cookie jar plus the CSRF header.
type client struct {
	t    *testing.T
	base string
	hc   *http.Client
}

func newClient(t *testing.T, e *env) *client {
	ts := httptest.NewServer(e.srv.Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: ts.URL + "/abc123", hc: &http.Client{Jar: jar}}
}

func (c *client) csrf() string {
	u, _ := url.Parse(c.base + "/")
	for _, ck := range c.hc.Jar.Cookies(u) {
		if ck.Name == csrfCookie {
			return ck.Value
		}
	}
	return ""
}

func (c *client) do(method, path, body string, withCSRF bool) (int, map[string]any) {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if withCSRF {
		req.Header.Set(csrfHeader, c.csrf())
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestLoginLogoutFlow(t *testing.T) {
	e := newEnv(t)
	c := newClient(t, e)
	if code, body := c.do("GET", "/api/auth/session", "", false); code != 200 || body["authenticated"] != false {
		t.Fatalf("session before login: %d %v", code, body)
	}
	if c.csrf() == "" {
		t.Fatal("csrf cookie not issued")
	}
	if code, _ := c.do("POST", "/api/auth/logout", "", true); code != 401 {
		t.Fatalf("logout without session: %d", code)
	}
	creds := `{"username":"` + e.user + `","password":"` + e.pass + `"}`
	if code, body := c.do("POST", "/api/auth/login", creds, true); code != 200 || body["username"] != e.user {
		t.Fatalf("login: %d %v", code, body)
	}
	if code, body := c.do("GET", "/api/auth/session", "", false); body["authenticated"] != true {
		t.Fatalf("session after login: %d %v", code, body)
	}
	if code, _ := c.do("POST", "/api/auth/logout", "", true); code != 200 {
		t.Fatalf("logout: %d", code)
	}
	if _, body := c.do("GET", "/api/auth/session", "", false); body["authenticated"] != false {
		t.Fatal("still authenticated after logout")
	}
	evs, _ := e.st.Events(context.Background(), 0, 10, "")
	if len(evs) == 0 || evs[0].Key != "auth.login" {
		t.Fatalf("login not journaled: %+v", evs)
	}
}

func TestCSRFRequiredForUnsafeMethods(t *testing.T) {
	e := newEnv(t)
	c := newClient(t, e)
	c.do("GET", "/api/auth/session", "", false)
	creds := `{"username":"` + e.user + `","password":"` + e.pass + `"}`
	if code, body := c.do("POST", "/api/auth/login", creds, false); code != 403 || body["error"] != "csrf" {
		t.Fatalf("login without csrf header: %d %v", code, body)
	}
	req, _ := http.NewRequest("POST", c.base+"/api/auth/login", strings.NewReader(creds))
	req.Header.Set(csrfHeader, "forged-token-forged-token-forged-1234")
	resp, err := c.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("mismatched csrf: %d", resp.StatusCode)
	}
	if code, _ := c.do("POST", "/api/auth/login", creds, true); code != 200 {
		t.Fatalf("login with csrf: %d", code)
	}
	if code, _ := c.do("POST", "/api/auth/logout", "", false); code != 403 {
		t.Fatalf("logout without csrf: %d", code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	c := newClient(t, e)
	c.do("GET", "/api/auth/session", "", false)
	bad := `{"username":"` + e.user + `","password":"wrong"}`
	for i := 0; i < 5; i++ {
		if code, _ := c.do("POST", "/api/auth/login", bad, true); code != 401 {
			t.Fatalf("attempt %d: %d", i, code)
		}
	}
	good := `{"username":"` + e.user + `","password":"` + e.pass + `"}`
	code, body := c.do("POST", "/api/auth/login", good, true)
	if code != 429 || body["error"] != "rate_limited" {
		t.Fatalf("expected block, got %d %v", code, body)
	}
	evs, _ := e.st.Events(context.Background(), 0, 10, "warn")
	if len(evs) != 1 || evs[0].Key != "auth.blocked" {
		t.Fatalf("block not journaled: %+v", evs)
	}
}

func TestProtectedRoutesNeedSession(t *testing.T) {
	e := newEnv(t)
	c := newClient(t, e)
	c.do("GET", "/api/auth/session", "", false)
	if code, _ := c.do("POST", "/api/auth/password", `{"current":"x","next":"y"}`, true); code != 401 {
		t.Fatalf("password change without session: %d", code)
	}
	if code, _ := c.do("GET", "/api/nope", "", false); code != 404 {
		t.Fatalf("unknown api route: %d", code)
	}
}

func TestPathValuesReachHandlers(t *testing.T) {
	e := newEnv(t)
	got := ""
	e.srv.Handle("PUT /api/things/{id}", func(w http.ResponseWriter, r *http.Request) {
		got = r.PathValue("id")
		writeJSON(w, 200, map[string]string{})
	})
	c := newClient(t, e)
	c.do("GET", "/api/auth/session", "", false)
	c.do("POST", "/api/auth/login", `{"username":"`+e.user+`","password":"`+e.pass+`"}`, true)
	if code, _ := c.do("PUT", "/api/things/42", "{}", true); code != 200 || got != "42" {
		t.Fatalf("path value %q, code %d", got, code)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	e := newEnv(t)
	c := newClient(t, e)
	c.do("GET", "/api/auth/session", "", false)
	c.do("POST", "/api/auth/login", `{"username":"`+e.user+`","password":"`+e.pass+`"}`, true)
	if _, err := e.st.DB.Exec(`UPDATE sessions SET expires_at = 1`); err != nil {
		t.Fatal(err)
	}
	if _, body := c.do("GET", "/api/auth/session", "", false); body["authenticated"] != false {
		t.Fatal("expired session accepted")
	}
	if code, _ := c.do("POST", "/api/auth/logout", "", true); code != 401 {
		t.Fatalf("protected route with expired session: %d", code)
	}
}

func TestAssetsAreGzipped(t *testing.T) {
	big := strings.Repeat("console.log('swift');\n", 200)
	e := newEnv(t)
	e.srv.Assets = fstest.MapFS{"index.html": {Data: []byte("<title>Swift</title>")}, "assets/app.js": {Data: []byte(big)}, "fonts/a.woff2": {Data: []byte("font")}}
	h := e.srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/abc123/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") || rec.Body.Len() >= len(big)/4 {
		t.Fatalf("gzip: %v, %d bytes", rec.Header(), rec.Body.Len())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := io.ReadAll(zr); string(body) != big {
		t.Fatal("gzipped body differs")
	}
	if rec := get(t, h, "/abc123/assets/app.js"); rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != big {
		t.Fatal("plain client got a compressed body")
	}
	req = httptest.NewRequest(http.MethodGet, "/abc123/fonts/a.woff2", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("fonts are already compressed")
	}
	req = httptest.NewRequest(http.MethodGet, "/abc123/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip;q=0")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("q=0 ignored")
	}
}

func TestIndexCarriesInstallLanguage(t *testing.T) {
	e := newEnv(t)
	e.srv.Assets = fstest.MapFS{"index.html": {Data: []byte(`<!doctype html><html lang="en"><head></head></html>`)}}
	h := e.srv.Handler()
	for _, tc := range []struct{ setting, want string }{{"", "en"}, {"ru", "ru"}, {"xx", "en"}} {
		if tc.setting != "" {
			if err := e.st.SetSetting(context.Background(), "language", tc.setting); err != nil {
				t.Fatal(err)
			}
		}
		for _, p := range []string{"/abc123/", "/abc123/index.html", "/abc123/users"} {
			if body := get(t, h, p).Body.String(); !strings.Contains(body, `<html lang="`+tc.want+`">`) {
				t.Fatalf("setting %q, %s: %s", tc.setting, p, body)
			}
		}
	}
}
