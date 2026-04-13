package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type landingSourceConfig struct {
	Code    string
	Label   string
	Domains []string
}

type LandingAttributionHandler struct {
	pool    *pgxpool.Pool
	sources map[string]landingSourceConfig
	order   []string
}

func NewLandingAttributionHandler(pool *pgxpool.Pool) *LandingAttributionHandler {
	defaults := []landingSourceConfig{
		{Code: "x", Label: "X", Domains: []string{"x.com", "twitter.com", "t.co"}},
		{Code: "rd", Label: "Reddit", Domains: []string{"reddit.com", "www.reddit.com", "old.reddit.com", "redd.it"}},
		{Code: "ih", Label: "Indie Hackers", Domains: []string{"indiehackers.com", "www.indiehackers.com"}},
		{Code: "ph", Label: "Product Hunt", Domains: []string{"producthunt.com", "www.producthunt.com"}},
		{Code: "o", Label: "Other"},
		{Code: "direct", Label: "Direct"},
	}

	sources := make(map[string]landingSourceConfig, len(defaults))
	order := make([]string, 0, len(defaults))
	for _, src := range defaults {
		sources[src.Code] = src
		order = append(order, src.Code)
	}

	rawExtra := strings.TrimSpace(os.Getenv("LANDING_EXTRA_SOURCES"))
	if rawExtra != "" {
		for _, item := range strings.Split(rawExtra, ";") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			parts := strings.SplitN(item, ":", 3)
			if len(parts) < 2 {
				continue
			}
			code := normalizeLandingCode(parts[0])
			label := strings.TrimSpace(parts[1])
			if code == "" || label == "" {
				continue
			}
			cfg := landingSourceConfig{Code: code, Label: label}
			if len(parts) == 3 && strings.TrimSpace(parts[2]) != "" {
				for _, domain := range strings.Split(parts[2], ",") {
					domain = normalizeHost(domain)
					if domain != "" {
						cfg.Domains = append(cfg.Domains, domain)
					}
				}
			}
			if _, exists := sources[code]; !exists {
				order = append(order, code)
			}
			sources[code] = cfg
		}
	}

	return &LandingAttributionHandler{
		pool:    pool,
		sources: sources,
		order:   order,
	}
}

type recordLandingVisitRequest struct {
	Path      string `json:"path"`
	Source    string `json:"source"`
	SessionID string `json:"session_id"`
	Referrer  string `json:"referrer"`
}

type adminLandingSourceRow struct {
	SourceCode     string  `json:"source_code"`
	Label          string  `json:"label"`
	Visits         int64   `json:"visits"`
	UniqueVisitors int64   `json:"unique_visitors"`
	LastVisitAt    *string `json:"last_visit_at"`
}

type adminLandingSourcesResponse struct {
	RangeDays      int64                   `json:"range_days"`
	TotalVisits    int64                   `json:"total_visits"`
	UniqueVisitors int64                   `json:"unique_visitors"`
	Rows           []adminLandingSourceRow `json:"rows"`
}

func (h *LandingAttributionHandler) RecordVisit(w http.ResponseWriter, r *http.Request) {
	var body recordLandingVisitRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	sessionID := strings.TrimSpace(body.SessionID)
	if sessionID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "session_id is required")
		return
	}
	if len(sessionID) > 128 {
		sessionID = sessionID[:128]
	}

	path := strings.TrimSpace(body.Path)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 255 {
		path = path[:255]
	}

	referer := strings.TrimSpace(body.Referrer)
	if len(referer) > 512 {
		referer = referer[:512]
	}
	userAgent := strings.TrimSpace(r.UserAgent())
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}

	sourceCode := h.resolveSource(body.Source, referer)

	_, err := h.pool.Exec(r.Context(), `
INSERT INTO landing_visits (path, source_code, referer, session_id, user_agent)
SELECT $1, $2, $3, $4, $5
WHERE NOT EXISTS (
  SELECT 1
  FROM landing_visits
  WHERE session_id = $4
    AND path = $1
    AND source_code = $2
    AND created_at >= NOW() - INTERVAL '30 minutes'
)`,
		path, sourceCode, referer, sessionID, userAgent,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to record landing visit")
		return
	}

	writeSuccess(w, map[string]bool{"recorded": true})
}

func (h *LandingAttributionHandler) GetAdminSources(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	rows, err := h.pool.Query(r.Context(), `
SELECT
  source_code,
  COUNT(*)::BIGINT AS visits,
  COUNT(DISTINCT session_id)::BIGINT AS unique_visitors,
  MAX(created_at)
FROM landing_visits
WHERE created_at >= NOW() - make_interval(days => $1)
GROUP BY source_code
ORDER BY visits DESC, source_code ASC`, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load landing source stats")
		return
	}
	defer rows.Close()

	resp := adminLandingSourcesResponse{
		RangeDays: int64(days),
		Rows:      []adminLandingSourceRow{},
	}

	for rows.Next() {
		var code string
		var visits int64
		var uniqueVisitors int64
		var lastVisit *time.Time
		if err := rows.Scan(&code, &visits, &uniqueVisitors, &lastVisit); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan landing source stats")
			return
		}
		var lastVisitAt *string
		if lastVisit != nil {
			formatted := lastVisit.UTC().Format(time.RFC3339)
			lastVisitAt = &formatted
		}
		resp.TotalVisits += visits
		resp.UniqueVisitors += uniqueVisitors
		resp.Rows = append(resp.Rows, adminLandingSourceRow{
			SourceCode:     code,
			Label:          h.labelFor(code),
			Visits:         visits,
			UniqueVisitors: uniqueVisitors,
			LastVisitAt:    lastVisitAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read landing source stats")
		return
	}

	sort.SliceStable(resp.Rows, func(i, j int) bool {
		if resp.Rows[i].Visits == resp.Rows[j].Visits {
			return resp.Rows[i].Label < resp.Rows[j].Label
		}
		return resp.Rows[i].Visits > resp.Rows[j].Visits
	})

	writeSuccess(w, resp)
}

func (h *LandingAttributionHandler) resolveSource(explicitSource string, referrer string) string {
	code := normalizeLandingCode(explicitSource)
	if _, ok := h.sources[code]; ok && code != "direct" {
		return code
	}

	host := normalizeHost(referrer)
	if host != "" {
		for _, code := range h.order {
			cfg := h.sources[code]
			for _, domain := range cfg.Domains {
				if host == domain || strings.HasSuffix(host, "."+domain) {
					return cfg.Code
				}
			}
		}
		return "o"
	}

	return "direct"
}

func (h *LandingAttributionHandler) labelFor(code string) string {
	if cfg, ok := h.sources[code]; ok && cfg.Label != "" {
		return cfg.Label
	}
	return strings.ToUpper(code)
}

func normalizeLandingCode(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(u.Hostname()))
}
