package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/store"
)

const (
	sessionCookie = "swift_session"
	csrfCookie    = "swift_csrf"
	csrfHeader    = "X-Swift-CSRF"
)

type ctxKey int

const adminKey ctxKey = 1

// AdminFrom returns the authenticated admin stored by requireSession.
func AdminFrom(ctx context.Context) (store.Admin, bool) {
	a, ok := ctx.Value(adminKey).(store.Admin)
	return a, ok
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, name, value string, httpOnly bool, expires time.Time) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     s.cookiePath(),
		HttpOnly: httpOnly,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	}
	if !expires.IsZero() {
		c.Expires = expires
		c.MaxAge = int(math.Max(1, time.Until(expires).Seconds()))
	}
	if value == "" {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
}

// ensureCSRF issues the double-submit token cookie if the browser has none.
func (s *Server) ensureCSRF(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(csrfCookie); err == nil && len(c.Value) >= 32 {
		return
	}
	s.setCookie(w, r, csrfCookie, auth.RandomToken(24), false, time.Time{})
}

// csrfOK requires the X-Swift-CSRF header to match the cookie for unsafe methods.
func (s *Server) csrfOK(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	c, err := r.Cookie(csrfCookie)
	if err != nil || c.Value == "" {
		return false
	}
	h := r.Header.Get(csrfHeader)
	return h != "" && subtle.ConstantTimeCompare([]byte(h), []byte(c.Value)) == 1
}

func (s *Server) requireSession(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id string
		if c, err := r.Cookie(sessionCookie); err == nil {
			id = c.Value
		}
		ses, admin, err := s.Auth.Validate(r.Context(), id)
		if err != nil {
			if !errors.Is(err, auth.ErrNoSession) {
				s.Log.Error("validate session", "err", err)
				writeError(w, http.StatusInternalServerError, "internal")
				return
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		s.setCookie(w, r, sessionCookie, ses.ID, true, ses.ExpiresAt)
		next(w, r.WithContext(context.WithValue(r.Context(), adminKey, admin)))
	})
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	s.ensureCSRF(w, r)
	var id string
	if c, err := r.Cookie(sessionCookie); err == nil {
		id = c.Value
	}
	_, admin, err := s.Auth.Validate(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": admin.Username})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if ok, wait := s.Limiter.Allow(ip); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited", "retry_after": int(wait.Seconds()) + 1})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	ses, err := s.Auth.Login(r.Context(), body.Username, body.Password, ip, r.UserAgent())
	if errors.Is(err, auth.ErrInvalidCredentials) {
		if s.Limiter.Fail(ip) {
			s.event(r.Context(), "warn", "auth", "auth.blocked", map[string]any{"ip": ip})
		}
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if err != nil {
		s.Log.Error("login", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.Limiter.Success(ip)
	s.event(r.Context(), "info", "auth", "auth.login", map[string]any{"ip": ip})
	s.setCookie(w, r, sessionCookie, ses.ID, true, ses.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": body.Username})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.Auth.Logout(r.Context(), c.Value); err != nil {
			s.Log.Error("logout", "err", err)
		}
	}
	s.setCookie(w, r, sessionCookie, "", true, time.Time{})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	admin, _ := AdminFrom(r.Context())
	var body struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if len([]rune(body.Next)) < 12 {
		writeError(w, http.StatusBadRequest, "password_too_short")
		return
	}
	err := s.Auth.ChangePassword(r.Context(), admin.ID, body.Current, body.Next)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusBadRequest, "wrong_password")
		return
	}
	if err != nil {
		s.Log.Error("change password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	// all sessions were dropped; issue a fresh one for this browser
	pw := body.Next
	ses, err := s.Auth.Login(r.Context(), admin.Username, pw, clientIP(r), r.UserAgent())
	if err == nil {
		s.setCookie(w, r, sessionCookie, ses.ID, true, ses.ExpiresAt)
	}
	s.event(r.Context(), "info", "auth", "auth.password_changed", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// event writes to the journal; failures are logged and otherwise ignored.
func (s *Server) event(ctx context.Context, level, source, key string, params map[string]any) {
	if s.Store == nil {
		return
	}
	ev, err := s.Store.AddEvent(ctx, level, source, key, params)
	if err != nil {
		s.Log.Error("journal", "err", err)
		return
	}
	if s.OnEvent != nil {
		s.OnEvent(ev)
	}
}
