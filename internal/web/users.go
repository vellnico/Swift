package web

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/vellnico/swift/internal/qr"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
)

func (s *Server) registerUsers() {
	if s.Users == nil || s.Subs == nil {
		return
	}
	s.Handle("GET /api/users", s.listUsers)
	s.Handle("POST /api/users", s.createUser)
	s.Handle("PUT /api/users/{id}", s.updateUser)
	s.Handle("DELETE /api/users/{id}", s.deleteUser)
	s.Handle("POST /api/users/{id}/reissue", s.reissueUser)
	s.Handle("GET /api/users/{id}/links", s.userLinks)
	s.Handle("GET /api/users/{id}/qr.svg", s.userQR)
	s.Handle("GET /api/users/{id}/links/{pid}/qr.svg", s.userLinkQR)
}

type userJSON struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Note      string    `json:"note"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	SubURL    string    `json:"subUrl"`
}

func (s *Server) userView(ctx context.Context, u store.User, host string) (userJSON, error) {
	url, err := s.Subs.URL(ctx, u.SubToken, host)
	if err != nil {
		return userJSON{}, err
	}
	return userJSON{ID: u.ID, Name: u.Name, Note: u.Note, Enabled: u.Enabled, CreatedAt: u.CreatedAt, SubURL: url}, nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	us, err := s.Store.Users(r.Context())
	if err != nil {
		s.serviceError(w, err)
		return
	}
	out := make([]userJSON, 0, len(us))
	for _, u := range us {
		v, err := s.userView(r.Context(), u, r.Host)
		if err != nil {
			s.serviceError(w, err)
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) respondUser(w http.ResponseWriter, r *http.Request, code int, u store.User) {
	v, err := s.userView(r.Context(), u, r.Host)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, code, v)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Note string `json:"note"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	u, err := s.Users.Create(r.Context(), body.Name, body.Note)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	s.event(r.Context(), "info", "users", "users.created", map[string]any{"name": u.Name})
	s.respondUser(w, r, http.StatusCreated, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var upd service.UserUpdate
	if err := DecodeJSON(r, &upd); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	u, err := s.Users.Update(r.Context(), id, upd)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	s.respondUser(w, r, http.StatusOK, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	u, err := s.Users.Delete(r.Context(), id)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	s.event(r.Context(), "info", "users", "users.deleted", map[string]any{"name": u.Name})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) reissueUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	u, err := s.Users.Reissue(r.Context(), id)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	s.event(r.Context(), "info", "users", "users.reissued", map[string]any{"name": u.Name})
	s.respondUser(w, r, http.StatusOK, u)
}

func (s *Server) userLinks(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	u, err := s.Store.User(r.Context(), id)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	ls, err := s.Subs.ProfileLinks(r.Context(), u, r.Host)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ls)
}

func writeSVG(w http.ResponseWriter, data string) {
	svg, err := qr.SVG(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write(svg)
}

func (s *Server) userQR(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	u, err := s.Store.User(r.Context(), id)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	url, err := s.Subs.URL(r.Context(), u.SubToken, r.Host)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	writeSVG(w, url)
}

func (s *Server) userLinkQR(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	pid, err := strconv.ParseInt(r.PathValue("pid"), 10, 64)
	if !ok || err != nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	u, err := s.Store.User(r.Context(), id)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	ls, err := s.Subs.ProfileLinks(r.Context(), u, r.Host)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	for _, l := range ls {
		if l.ProfileID == pid {
			writeSVG(w, l.URL)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found")
}
