// Package web serves the panel: the embedded SPA and the JSON API under /<panel_path>/.
package web

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

type Server struct {
	PanelPath string // without slashes
	Assets    fs.FS
	Log       *slog.Logger
	api       *http.ServeMux
}

func New(panelPath string, assets fs.FS, log *slog.Logger) *Server {
	s := &Server{PanelPath: strings.Trim(panelPath, "/"), Assets: assets, Log: log, api: http.NewServeMux()}
	s.api.HandleFunc("GET /api/health", s.health)
	return s
}

// API exposes the mux so other packages can register routes relative to the panel root.
func (s *Server) API() *http.ServeMux { return s.api }

func (s *Server) Handler() http.Handler {
	prefix := "/" + s.PanelPath
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == prefix {
			http.Redirect(w, r, prefix+"/", http.StatusMovedPermanently)
			return
		}
		rest, ok := strings.CutPrefix(r.URL.Path, prefix+"/")
		if !ok {
			notFound(w)
			return
		}
		if rest == "api" || strings.HasPrefix(rest, "api/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/" + rest
			r2.URL.RawPath = ""
			h, pattern := s.api.Handler(r2)
			if pattern == "" {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
				return
			}
			h.ServeHTTP(w, r2)
			return
		}
		s.serveSPA(w, r, rest)
	}))
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name = path.Clean("/" + name)[1:]
	if name != "" {
		if st, err := fs.Stat(s.Assets, name); err == nil && !st.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFileFS(w, r, s.Assets, name)
			return
		}
	}
	body, err := fs.ReadFile(s.Assets, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("Swift: frontend is not built, run make build\n"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// notFound mimics a server that has nothing here: no body, no hints.
func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
