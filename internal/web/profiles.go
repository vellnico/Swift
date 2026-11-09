package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

func (s *Server) registerProfiles() {
	if s.Profiles == nil {
		return
	}
	s.Handle("GET /api/profiles", s.listProfiles)
	s.Handle("GET /api/profiles/schema", s.profileSchema)
	s.Handle("POST /api/profiles", s.createProfile)
	s.Handle("PUT /api/profiles/{id}", s.updateProfile)
	s.Handle("DELETE /api/profiles/{id}", s.deleteProfile)
	s.Handle("POST /api/profiles/{id}/apply", s.applyProfiles)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// serviceError turns service errors into stable API error codes.
func (s *Server) serviceError(w http.ResponseWriter, err error) {
	var ve *service.ValidationError
	var rej *service.ConfigRejected
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation", "fields": ve.Fields, "detail": ve.Detail})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, store.ErrDuplicate):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "validation", "fields": map[string]string{"name": "duplicate"}})
	case errors.As(err, &rej):
		s.Log.Warn("xray rejected config", "detail", rej.Detail)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "xray_rejected", "detail": rej.Detail})
	default:
		s.Log.Error("request failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	ps, err := s.Store.Profiles(r.Context())
	if err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) profileSchema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, xray.Schema())
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind xray.Kind `json:"kind"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	p, err := s.Profiles.Create(r.Context(), body.Kind)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	s.event(r.Context(), "info", "profiles", "profiles.created", map[string]any{"name": p.Name})
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var u service.Update
	if err := DecodeJSON(r, &u); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	p, err := s.Profiles.Update(r.Context(), id, u)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	p, err := s.Store.Profile(r.Context(), id)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	if err := s.Profiles.Delete(r.Context(), id); err != nil {
		s.serviceError(w, err)
		return
	}
	s.event(r.Context(), "info", "profiles", "profiles.deleted", map[string]any{"name": p.Name})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) applyProfiles(w http.ResponseWriter, r *http.Request) {
	if err := s.Profiles.Xray.Apply(r.Context()); err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
