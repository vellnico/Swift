package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// events streams bus messages as Server-Sent Events; the UI never polls.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok || s.Bus == nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n\n")
	fl.Flush()

	ch, cancel := s.Bus.Subscribe()
	defer cancel()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		case m := <-ch:
			body, err := json.Marshal(m.Data)
			if err != nil {
				s.Log.Error("sse encode", "type", m.Type, "err", err)
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", m.Type, body)
		}
		fl.Flush()
	}
}
