package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
)

func (s *Server) registerCerts() {
	if s.Certs == nil {
		return
	}
	s.Handle("GET /api/certs", s.getCert)
	s.Handle("POST /api/certs/acme", s.issueACME)
	s.Handle("POST /api/certs/self-signed", s.issueSelfSigned)
	s.Handle("POST /api/certs/ip", s.switchToIP)
	s.Handle("POST /api/certs/renew", s.renewCert)
}

type certJSON struct {
	Kind      string    `json:"kind"`
	Domain    string    `json:"domain"`
	SHA256    string    `json:"sha256"`
	NotAfter  time.Time `json:"notAfter"`
	LastError string    `json:"lastError"`
}

func certView(c store.Certificate) certJSON {
	return certJSON{Kind: c.Kind, Domain: c.Domain, SHA256: c.SHA256Hex, NotAfter: c.NotAfter, LastError: c.LastError}
}

func (s *Server) certError(w http.ResponseWriter, err error) {
	var ie *service.IssueError
	if errors.As(err, &ie) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "acme_failed", "detail": ie.Detail})
		return
	}
	s.serviceError(w, err)
}

func (s *Server) getCert(w http.ResponseWriter, r *http.Request) {
	c, err := s.Certs.Ensure(r.Context())
	if err != nil {
		s.certError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, certView(c))
}

func (s *Server) issueACME(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain string `json:"domain"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	c, err := s.Certs.IssueACME(r.Context(), body.Domain)
	if err != nil {
		s.certError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, certView(c))
}

func (s *Server) issueSelfSigned(w http.ResponseWriter, r *http.Request) {
	c, err := s.Certs.IssueSelfSigned(r.Context())
	if err != nil {
		s.certError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, certView(c))
}

// switchToIP answers with a self-signed certificate and its lastError when Let's Encrypt refused.
func (s *Server) switchToIP(w http.ResponseWriter, r *http.Request) {
	c, err := s.Certs.SwitchToIP(r.Context())
	if err != nil {
		s.certError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, certView(c))
}

func (s *Server) renewCert(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Certs.RenewIfNeeded(r.Context(), true); err != nil {
		s.certError(w, err)
		return
	}
	c, err := s.Store.Certificate(r.Context())
	if err != nil {
		s.certError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, certView(c))
}
