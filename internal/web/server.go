// Package web serves the panel: the embedded SPA and the JSON API under /<panel_path>/.
package web

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync/atomic"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/bus"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
)

type Deps struct {
	PanelPath string // without slashes
	Assets    fs.FS
	Log       *slog.Logger
	Store     *store.Store
	Auth      *auth.Manager
	Limiter   *auth.Limiter
	Bus       *bus.Bus
	Profiles  *service.Profiles
	Users     *service.Users
	Subs      *service.Subscriptions
	Certs     *service.Certs
	Panel     *service.Panel
	Updater   *service.XrayUpdater
	Swift     *service.SwiftUpdater
	Health    *service.Health
	// PanelPort returns the port the panel listens on now.
	PanelPort func() int
	// OnEvent is called after a journal entry is written (used to push it over SSE).
	OnEvent func(store.Event)
}

type Server struct {
	Deps
	api  *http.ServeMux
	path atomic.Pointer[string]
	gz   gzipCache
}

func New(d Deps) *Server {
	d.PanelPath = strings.Trim(d.PanelPath, "/")
	if d.Log == nil {
		d.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if d.Limiter == nil {
		d.Limiter = auth.NewLimiter()
	}
	s := &Server{Deps: d, api: http.NewServeMux()}
	s.SetPanelPath(d.PanelPath)
	s.api.HandleFunc("GET /api/health", s.health)
	s.api.HandleFunc("GET /api/auth/session", s.session)
	s.api.HandleFunc("POST /api/auth/login", s.login)
	s.Handle("POST /api/auth/logout", s.logout)
	s.Handle("POST /api/auth/password", s.changePassword)
	s.Handle("GET /api/events", s.events)
	s.Handle("GET /api/journal", s.journal)
	s.registerProfiles()
	s.registerUsers()
	s.registerCerts()
	s.registerSettings()
	if d.Store != nil {
		s.registerRouting()
	}
	return s
}

// Handle registers a route that requires a valid admin session.
func (s *Server) Handle(pattern string, h http.HandlerFunc) {
	s.api.Handle(pattern, s.requireSession(h))
}

// SetPanelPath moves the panel to a new secret path while serving.
func (s *Server) SetPanelPath(p string) {
	p = strings.Trim(p, "/")
	s.path.Store(&p)
}

func (s *Server) panelPath() string { return *s.path.Load() }

func (s *Server) cookiePath() string { return "/" + s.panelPath() + "/" }

func (s *Server) Handler() http.Handler {
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/" + s.panelPath()
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
			if _, pattern := s.api.Handler(r2); pattern == "" {
				writeError(w, http.StatusNotFound, "not_found")
				return
			}
			if !s.csrfOK(r2) {
				writeError(w, http.StatusForbidden, "csrf")
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			// ServeHTTP (not the handler from Handler) fills r.PathValue
			s.api.ServeHTTP(w, r2)
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
	// index.html always goes through the SPA branch below, which sets the language
	if name != "" && name != "index.html" {
		if st, err := fs.Stat(s.Assets, name); err == nil && !st.IsDir() {
			switch {
			case strings.HasPrefix(name, "assets/"):
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			case strings.HasPrefix(name, "fonts/"):
				w.Header().Set("Cache-Control", "public, max-age=2592000")
			}
			s.gz.serveFile(w, r, s.Assets, name)
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
	// the UI starts in the language chosen at install time unless the browser saved another
	body = setHTMLLang(body, service.Language(r.Context(), s.Store))
	s.ensureCSRF(w, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	writeBody(w, r, body)
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

// WriteJSON is exported for handlers registered from other packages.
func WriteJSON(w http.ResponseWriter, code int, v any) { writeJSON(w, code, v) }

func writeError(w http.ResponseWriter, code int, key string) {
	writeJSON(w, code, map[string]string{"error": key})
}

// WriteError sends {"error": key}; key is an i18n suffix the UI understands.
func WriteError(w http.ResponseWriter, code int, key string) { writeError(w, code, key) }

// DecodeJSON reads a small JSON body into v.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.Join(errBadRequest, err)
	}
	return nil
}

var errBadRequest = errors.New("bad request")

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
