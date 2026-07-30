package requestevents

import (
	"encoding/json"
	"net/http"
)

// StatsHandler exposes aggregate recorder health to the Super Admin route.
// It intentionally contains no event, workspace, request, or payload data.
func StatsHandler(recorder *Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if recorder == nil {
			http.Error(w, "request-event recorder is unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]Stats{"data": recorder.Stats()})
	}
}
