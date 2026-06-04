package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// webEvent is a small browser telemetry payload sent by the web app's beacon.
type webEvent struct {
	Type  string         `json:"type"`
	Props map[string]any `json:"props,omitempty"`
}

// eventsHandler accepts browser telemetry, records it as a structured log line
// (shipped to VictoriaLogs) and a Prometheus counter. Public by design — it only
// records, never reads protected data. Body is size-capped.
func (s *Server) eventsHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)

	var e webEvent
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil || e.Type == "" {
		http.Error(w, "invalid event", http.StatusBadRequest)
		return
	}

	slog.Info("web_event",
		"event", "web."+e.Type,
		"props", e.Props,
		"request_id", middleware.GetReqID(r.Context()),
	)
	webEventsTotal.WithLabelValues(e.Type).Inc()

	w.WriteHeader(http.StatusNoContent)
}
