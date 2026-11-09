package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/vellnico/swift/internal/service"
)

func (s *Server) registerSettings() {
	if s.Panel != nil {
		s.Handle("GET /api/settings/panel", s.getPanel)
		s.Handle("PUT /api/settings/panel", s.putPanel)
		s.Handle("PUT /api/settings/address", s.putAddress)
	}
	if s.Updater != nil {
		s.Handle("GET /api/xray/release", s.xrayRelease)
		s.Handle("POST /api/xray/check", s.xrayCheck)
		s.Handle("POST /api/xray/update", s.xrayUpdate)
	}
	if s.Store != nil {
		s.Handle("GET /api/settings/subscriptions", s.getSubOptions)
		s.Handle("PUT /api/settings/subscriptions", s.putSubOptions)
	}
	if s.Health != nil {
		s.Handle("GET /api/diagnostics", s.diagnostics)
		s.Handle("POST /api/diagnostics/check", s.diagnosticsCheck)
		s.Handle("POST /api/diagnostics/fix", s.diagnosticsFix)
	}
	if s.Swift != nil {
		s.Handle("GET /api/swift/release", s.swiftRelease)
		s.Handle("POST /api/swift/check", s.swiftCheck)
		s.Handle("POST /api/swift/update", s.swiftUpdate)
		s.Handle("PUT /api/swift/auto", s.swiftAuto)
	}
}

func (s *Server) currentPort() int {
	if s.PanelPort == nil {
		return 0
	}
	return s.PanelPort()
}

func (s *Server) getPanel(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Panel.Info(r.Context(), s.currentPort(), s.panelPath()))
}

func (s *Server) putPanel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Port int    `json:"port"`
		Path string `json:"path"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := s.Panel.Change(r.Context(), s.currentPort(), s.panelPath(), body.Port, body.Path); err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.Panel.Info(r.Context(), body.Port, body.Path))
}

func (s *Server) putAddress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address string `json:"address"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := s.Panel.SetAddress(r.Context(), body.Address); err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.Panel.Info(r.Context(), s.currentPort(), s.panelPath()))
}

func (s *Server) journal(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	level := q.Get("level")
	switch level {
	case "", "info", "warn", "error":
	default:
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	evs, err := s.Store.Events(r.Context(), before, limit, level)
	if err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, evs)
}

func (s *Server) xrayRelease(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Updater.Info(r.Context()))
}

func (s *Server) xrayCheck(w http.ResponseWriter, r *http.Request) {
	info, err := s.Updater.Check(r.Context())
	if err != nil {
		s.Log.Warn("xray release check", "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "check_failed", "release": info})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) xrayUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Version string `json:"version"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	// the download outlives a closed browser tab
	err := s.Updater.Update(context.WithoutCancel(r.Context()), body.Version)
	var ue *service.UpdateError
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, s.Updater.Info(r.Context()))
	case errors.Is(err, service.ErrUpdateBusy):
		writeError(w, http.StatusConflict, "update_busy")
	case errors.As(err, &ue):
		s.Log.Warn("xray update failed", "code", ue.Code, "detail", ue.Detail)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "update_" + ue.Code, "detail": ue.Detail})
	default:
		s.serviceError(w, err)
	}
}

func (s *Server) swiftRelease(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Swift.Info(r.Context()))
}

func (s *Server) swiftCheck(w http.ResponseWriter, r *http.Request) {
	info, err := s.Swift.Check(r.Context())
	if err != nil {
		s.Log.Warn("swift release check", "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "check_failed", "release": info})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) swiftUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Version string `json:"version"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	// the download outlives a closed browser tab
	err := s.Swift.Update(context.WithoutCancel(r.Context()), body.Version)
	var ue *service.UpdateError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, s.Swift.Info(r.Context()))
	case errors.Is(err, service.ErrUpdateBusy):
		writeError(w, http.StatusConflict, "update_busy")
	case errors.As(err, &ue):
		s.Log.Warn("swift update failed", "code", ue.Code, "detail", ue.Detail)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "update_" + ue.Code, "detail": ue.Detail})
	default:
		s.serviceError(w, err)
	}
}

func (s *Server) swiftAuto(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := s.Swift.SetAuto(r.Context(), body.Enabled); err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.Swift.Info(r.Context()))
}

type subOptions struct {
	AutoSelect   bool `json:"autoSelect"`
	HealthChecks bool `json:"healthChecks"`
}

func (s *Server) subOptions(ctx context.Context) subOptions {
	return subOptions{AutoSelect: service.AutoSelect(ctx, s.Store), HealthChecks: service.HealthChecks(ctx, s.Store)}
}

func (s *Server) getSubOptions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.subOptions(r.Context()))
}

func (s *Server) putSubOptions(w http.ResponseWriter, r *http.Request) {
	var body subOptions
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := service.SetSubscriptionOptions(r.Context(), s.Store, body.AutoSelect, body.HealthChecks); err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.subOptions(r.Context()))
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Health.Report(r.Context()))
}

func (s *Server) diagnosticsCheck(w http.ResponseWriter, r *http.Request) {
	// the check outlives a closed tab; the result also arrives over SSE
	rep, err := s.Health.Check(context.WithoutCancel(r.Context()))
	switch {
	case errors.Is(err, service.ErrHealthBusy):
		writeError(w, http.StatusConflict, "check_busy")
	case err != nil:
		s.serviceError(w, err)
	default:
		writeJSON(w, http.StatusOK, rep)
	}
}

func (s *Server) diagnosticsFix(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProfileID int64  `json:"profileId"`
		Action    string `json:"action"`
		Value     string `json:"value"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if _, err := s.Health.ApplyFix(r.Context(), body.ProfileID, service.HealthFix{Action: body.Action, Value: body.Value}); err != nil {
		s.serviceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.Health.Report(r.Context()))
}
