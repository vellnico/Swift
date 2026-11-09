package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testServer() *Server {
	assets := fstest.MapFS{
		"index.html":        {Data: []byte("<title>Swift</title>")},
		"assets/app-1.js":   {Data: []byte("console.log(1)")},
		"fonts/onest.woff2": {Data: []byte("font")},
	}
	return New("abc123", assets, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestOutsidePanelPathIsEmpty404(t *testing.T) {
	h := testServer().Handler()
	for _, p := range []string{"/", "/index.html", "/abc12", "/abc1234/", "/api/health", "/../abc123/"} {
		rec := get(t, h, p)
		if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
			t.Errorf("%s: code %d body %q", p, rec.Code, rec.Body.String())
		}
	}
}

func TestSPAAndFallback(t *testing.T) {
	h := testServer().Handler()
	for _, p := range []string{"/abc123/", "/abc123/users/5", "/abc123/../abc123/x"} {
		rec := get(t, h, p)
		if rec.Code != http.StatusOK || rec.Body.String() != "<title>Swift</title>" {
			t.Errorf("%s: code %d body %q", p, rec.Code, rec.Body.String())
		}
	}
	rec := get(t, h, "/abc123/assets/app-1.js")
	if rec.Code != 200 || rec.Header().Get("Cache-Control") == "" {
		t.Errorf("asset: %d %v", rec.Code, rec.Header())
	}
	if rec := get(t, h, "/abc123"); rec.Code != http.StatusMovedPermanently {
		t.Errorf("redirect: %d", rec.Code)
	}
}

func TestAPI(t *testing.T) {
	h := testServer().Handler()
	rec := get(t, h, "/abc123/api/health")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("health: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "/abc123/api/nope"); rec.Code != 404 {
		t.Fatalf("unknown api: %d", rec.Code)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("security headers missing")
	}
}
