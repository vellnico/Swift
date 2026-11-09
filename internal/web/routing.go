package web

import (
	"net/http"

	"github.com/vellnico/swift/internal/service"
)

func (s *Server) registerRouting() {
	s.Handle("GET /api/routing", func(w http.ResponseWriter, r *http.Request) {
		p, err := service.Preset(r.Context(), s.Store)
		if err != nil {
			s.serviceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"preset": p})
	})
	s.Handle("PUT /api/routing", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Preset string `json:"preset"`
		}
		if err := DecodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request")
			return
		}
		if err := service.SetPreset(r.Context(), s.Store, body.Preset); err != nil {
			s.serviceError(w, err)
			return
		}
		s.event(r.Context(), "info", "routing", "routing.changed", map[string]any{"preset": body.Preset})
		writeJSON(w, http.StatusOK, map[string]string{"preset": body.Preset})
	})
}
