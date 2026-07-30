package requesteventpartitions

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Inspector interface {
	Inspect(context.Context, time.Time, int) (Inspection, error)
}

func StatusHandler(inspector Inspector, worker *Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		inspection, err := inspector.Inspect(ctx, time.Now(), MinimumCoverageDays)
		if err != nil {
			http.Error(w, "request-event partition inspection is unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"inspection": inspection,
				"worker":     worker.Stats(),
			},
		})
	}
}
