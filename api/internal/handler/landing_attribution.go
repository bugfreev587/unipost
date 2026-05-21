package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
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
		{Code: "google", Label: "Google", Domains: []string{"google.com", "www.google.com"}},
		{Code: "meta", Label: "Meta", Domains: []string{"facebook.com", "www.facebook.com", "instagram.com", "www.instagram.com"}},
		{Code: "microsoft", Label: "Microsoft", Domains: []string{"bing.com", "www.bing.com"}},
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
	Path        string            `json:"path"`
	Source      string            `json:"source"`
	SessionID   string            `json:"session_id"`
	Referrer    string            `json:"referrer"`
	CountryCode string            `json:"country_code"`
	Attribution map[string]string `json:"attribution"`
	RawQuery    string            `json:"raw_query"`
}

type adminLandingSourceRow struct {
	SourceCode         string  `json:"source_code"`
	Label              string  `json:"label"`
	Visits             int64   `json:"visits"`
	UniqueVisitors     int64   `json:"unique_visitors"`
	Signups            int64   `json:"signups"`
	PaidUsers          int64   `json:"paid_users"`
	SignupRate         float64 `json:"signup_rate"`
	PaidConversionRate float64 `json:"paid_conversion_rate"`
	TopCampaign        *string `json:"top_campaign"`
	LastVisitAt        *string `json:"last_visit_at"`
}

type adminLandingSourcesResponse struct {
	RangeDays      int64                   `json:"range_days"`
	TotalVisits    int64                   `json:"total_visits"`
	UniqueVisitors int64                   `json:"unique_visitors"`
	Rows           []adminLandingSourceRow `json:"rows"`
}

type adminLandingVisitorRow struct {
	ID          int64             `json:"id"`
	CreatedAt   string            `json:"created_at"`
	Path        string            `json:"path"`
	SourceCode  string            `json:"source_code"`
	Label       string            `json:"label"`
	Referrer    string            `json:"referrer"`
	SessionID   string            `json:"session_id"`
	CountryCode string            `json:"country_code"`
	UserID      *string           `json:"user_id"`
	UserEmail   *string           `json:"user_email"`
	RawQuery    string            `json:"raw_query"`
	Attribution map[string]string `json:"attribution"`
}

type adminLandingVisitorTrendRow struct {
	Date           string `json:"date"`
	Visits         int64  `json:"visits"`
	UniqueVisitors int64  `json:"unique_visitors"`
	Signups        int64  `json:"signups"`
}

type adminLandingVisitorsResponse struct {
	RangeDays       int64                         `json:"range_days"`
	TotalVisits     int64                         `json:"total_visits"`
	UniqueVisitors  int64                         `json:"unique_visitors"`
	Signups         int64                         `json:"signups"`
	Rows            []adminLandingVisitorRow      `json:"rows"`
	Trend           []adminLandingVisitorTrendRow `json:"trend"`
	Countries       []adminCountryBreakdownRow    `json:"countries"`
	SourceOptions   []string                      `json:"source_options"`
	CampaignOptions []string                      `json:"campaign_options"`
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
	if referer == "" {
		referer = strings.TrimSpace(r.Header.Get("Referer"))
	}
	if len(referer) > 512 {
		referer = referer[:512]
	}
	userAgent := strings.TrimSpace(r.UserAgent())
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}

	if isLandingBotUserAgent(userAgent) {
		writeSuccess(w, map[string]bool{"recorded": false})
		return
	}

	countryCode := landingCountryCodeFromRequest(r, body.CountryCode)

	utmEnabled := featureflags.Enabled(r.Context(), featureflags.AttributionUTMSignupBindingV1, featureflags.Target{
		SessionID: sessionID,
	})

	attribution := map[string]string{}
	rawQuery := ""
	sourceCode := h.resolveSource(body.Source, referer)
	if utmEnabled {
		attribution = sanitizeLandingAttribution(body.Attribution)
		rawQuery = sanitizeLandingText(body.RawQuery, 1024)
		sourceCode = h.resolveSourceWithAttribution(body.Source, referer, attribution)
	}
	attributionJSON, err := json.Marshal(attribution)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid attribution")
		return
	}

	_, err = h.pool.Exec(r.Context(), `
INSERT INTO landing_visits (path, source_code, referer, session_id, user_agent, attribution, raw_query, country_code)
SELECT $1, $2, $3, $4, $5, $6::jsonb, $7, $8
WHERE NOT EXISTS (
  SELECT 1
  FROM landing_visits
  WHERE session_id = $4
    AND path = $1
    AND source_code = $2
    AND created_at >= NOW() - INTERVAL '30 minutes'
)`,
		path, sourceCode, referer, sessionID, userAgent, string(attributionJSON), rawQuery, countryCode,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to record landing visit")
		return
	}

	writeSuccess(w, map[string]bool{"recorded": true})
}

type bindLandingSessionRequest struct {
	SessionID string `json:"session_id"`
}

func (h *LandingAttributionHandler) BindSessionToUser(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	if !featureflags.Enabled(r.Context(), featureflags.AttributionUTMSignupBindingV1, featureflags.Target{
		UserID: userID,
	}) {
		writeSuccess(w, map[string]bool{"bound": false})
		return
	}

	var body bindLandingSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	sessionID := strings.TrimSpace(body.SessionID)
	if sessionID == "" {
		writeSuccess(w, map[string]bool{"bound": false})
		return
	}
	if len(sessionID) > 128 {
		sessionID = sessionID[:128]
	}

	_, err := h.pool.Exec(r.Context(), `
INSERT INTO landing_session_users (session_id, user_id)
VALUES ($1, $2)
ON CONFLICT (session_id, user_id)
DO UPDATE SET last_seen_at = NOW()`,
		sessionID, userID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to bind landing session")
		return
	}

	writeSuccess(w, map[string]bool{"bound": true})
}

func (h *LandingAttributionHandler) GetAdminSources(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	resp := adminLandingSourcesResponse{
		RangeDays: int64(days),
		Rows:      []adminLandingSourceRow{},
	}

	if err := h.pool.QueryRow(r.Context(), `
SELECT
  COUNT(*)::BIGINT,
  COUNT(DISTINCT session_id)::BIGINT
FROM landing_visits
WHERE created_at >= NOW() - make_interval(days => $1)`, days).Scan(&resp.TotalVisits, &resp.UniqueVisitors); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load landing source totals")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
WITH filtered AS (
  SELECT *
  FROM landing_visits
  WHERE created_at >= NOW() - make_interval(days => $1)
)
SELECT
  f.source_code,
  COUNT(*)::BIGINT AS visits,
  COUNT(DISTINCT f.session_id)::BIGINT AS unique_visitors,
  COUNT(DISTINCT lsu.user_id)::BIGINT AS signups,
  COUNT(DISTINCT CASE WHEN pl.price_cents > 0 THEN lsu.user_id END)::BIGINT AS paid_users,
  (
    SELECT NULLIF(f2.attribution->>'utm_campaign', '')
    FROM filtered f2
    WHERE f2.source_code = f.source_code
      AND NULLIF(f2.attribution->>'utm_campaign', '') IS NOT NULL
    GROUP BY 1
    ORDER BY COUNT(*) DESC, 1 ASC
    LIMIT 1
  ) AS top_campaign,
  MAX(f.created_at)
FROM filtered f
LEFT JOIN landing_session_users lsu ON lsu.session_id = f.session_id
LEFT JOIN workspaces w ON w.user_id = lsu.user_id
LEFT JOIN subscriptions s ON s.workspace_id = w.id AND s.status = 'active'
LEFT JOIN plans pl ON pl.id = s.plan_id
GROUP BY f.source_code
ORDER BY visits DESC, source_code ASC`, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load landing source stats")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var visits int64
		var uniqueVisitors int64
		var signups int64
		var paidUsers int64
		var topCampaign *string
		var lastVisit *time.Time
		if err := rows.Scan(&code, &visits, &uniqueVisitors, &signups, &paidUsers, &topCampaign, &lastVisit); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan landing source stats")
			return
		}
		var lastVisitAt *string
		if lastVisit != nil {
			formatted := lastVisit.UTC().Format(time.RFC3339)
			lastVisitAt = &formatted
		}
		signupRate := 0.0
		if uniqueVisitors > 0 {
			signupRate = float64(signups) / float64(uniqueVisitors)
		}
		paidConversionRate := 0.0
		if signups > 0 {
			paidConversionRate = float64(paidUsers) / float64(signups)
		}
		resp.Rows = append(resp.Rows, adminLandingSourceRow{
			SourceCode:         code,
			Label:              h.labelFor(code),
			Visits:             visits,
			UniqueVisitors:     uniqueVisitors,
			Signups:            signups,
			PaidUsers:          paidUsers,
			SignupRate:         signupRate,
			PaidConversionRate: paidConversionRate,
			TopCampaign:        topCampaign,
			LastVisitAt:        lastVisitAt,
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

func (h *LandingAttributionHandler) GetAdminVisitors(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	source := normalizeLandingCode(r.URL.Query().Get("source"))
	if source == "direct" {
		source = "direct"
	}
	campaign := sanitizeLandingText(r.URL.Query().Get("campaign"), 128)

	resp := adminLandingVisitorsResponse{
		RangeDays: int64(days),
		Rows:      []adminLandingVisitorRow{},
		Trend:     []adminLandingVisitorTrendRow{},
		Countries: []adminCountryBreakdownRow{},
	}

	if err := h.pool.QueryRow(r.Context(), `
WITH filtered AS (
  SELECT *
  FROM landing_visits
  WHERE created_at >= NOW() - make_interval(days => $1)
    AND ($2 = '' OR source_code = $2)
    AND ($3 = '' OR attribution->>'utm_campaign' = $3)
)
SELECT
  COUNT(*)::BIGINT,
  COUNT(DISTINCT f.session_id)::BIGINT,
  COUNT(DISTINCT lsu.user_id)::BIGINT
FROM filtered f
LEFT JOIN landing_session_users lsu ON lsu.session_id = f.session_id`,
		days, source, campaign,
	).Scan(&resp.TotalVisits, &resp.UniqueVisitors, &resp.Signups); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load visitor totals")
		return
	}

	trendRows, err := h.pool.Query(r.Context(), `
WITH filtered AS (
  SELECT *
  FROM landing_visits
  WHERE created_at >= NOW() - make_interval(days => $1)
    AND ($2 = '' OR source_code = $2)
    AND ($3 = '' OR attribution->>'utm_campaign' = $3)
)
SELECT
  to_char(date_trunc('day', f.created_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD') AS day,
  COUNT(*)::BIGINT,
  COUNT(DISTINCT f.session_id)::BIGINT,
  COUNT(DISTINCT lsu.user_id)::BIGINT
FROM filtered f
LEFT JOIN landing_session_users lsu ON lsu.session_id = f.session_id
GROUP BY day
ORDER BY day ASC`,
		days, source, campaign,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load visitor trend")
		return
	}
	defer trendRows.Close()
	for trendRows.Next() {
		var row adminLandingVisitorTrendRow
		if err := trendRows.Scan(&row.Date, &row.Visits, &row.UniqueVisitors, &row.Signups); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan visitor trend")
			return
		}
		resp.Trend = append(resp.Trend, row)
	}
	if err := trendRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read visitor trend")
		return
	}

	countryRows, err := h.pool.Query(r.Context(), `
WITH filtered AS (
  SELECT *
  FROM landing_visits
  WHERE created_at >= NOW() - make_interval(days => $1)
    AND ($2 = '' OR source_code = $2)
    AND ($3 = '' OR attribution->>'utm_campaign' = $3)
),
session_countries AS (
  SELECT DISTINCT ON (session_id)
    session_id,
    COALESCE(NULLIF(country_code, ''), '') AS country_code
  FROM filtered
  ORDER BY session_id, created_at ASC
)
SELECT country_code, COUNT(*)::BIGINT
FROM session_countries
GROUP BY country_code
ORDER BY COUNT(*) DESC, country_code ASC`,
		days, source, campaign,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load visitor countries")
		return
	}
	defer countryRows.Close()
	for countryRows.Next() {
		var row adminCountryBreakdownRow
		if err := countryRows.Scan(&row.CountryCode, &row.Count); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan visitor countries")
			return
		}
		resp.Countries = append(resp.Countries, row)
	}
	if err := countryRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read visitor countries")
		return
	}

	visitRows, err := h.pool.Query(r.Context(), `
SELECT
  lv.id,
  lv.created_at,
  lv.path,
  lv.source_code,
  lv.referer,
  lv.session_id,
  COALESCE(lv.country_code, ''),
  lv.raw_query,
  lv.attribution,
  lsu.user_id,
  u.email
FROM landing_visits lv
LEFT JOIN LATERAL (
  SELECT user_id
  FROM landing_session_users
  WHERE session_id = lv.session_id
  ORDER BY last_seen_at DESC
  LIMIT 1
) lsu ON TRUE
LEFT JOIN users u ON u.id = lsu.user_id
WHERE lv.created_at >= NOW() - make_interval(days => $1)
  AND ($2 = '' OR lv.source_code = $2)
  AND ($3 = '' OR lv.attribution->>'utm_campaign' = $3)
ORDER BY lv.created_at DESC, lv.id DESC
LIMIT $4`,
		days, source, campaign, limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load visitor rows")
		return
	}
	defer visitRows.Close()
	for visitRows.Next() {
		var id int64
		var createdAt time.Time
		var path, sourceCode, referer, sessionID, countryCode, rawQuery string
		var attributionBytes []byte
		var userID, userEmail *string
		if err := visitRows.Scan(&id, &createdAt, &path, &sourceCode, &referer, &sessionID, &countryCode, &rawQuery, &attributionBytes, &userID, &userEmail); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan visitor rows")
			return
		}
		attribution := map[string]string{}
		if len(attributionBytes) > 0 {
			_ = json.Unmarshal(attributionBytes, &attribution)
		}
		resp.Rows = append(resp.Rows, adminLandingVisitorRow{
			ID:          id,
			CreatedAt:   createdAt.UTC().Format(time.RFC3339),
			Path:        path,
			SourceCode:  sourceCode,
			Label:       h.labelFor(sourceCode),
			Referrer:    referer,
			SessionID:   sessionID,
			CountryCode: countryCode,
			UserID:      userID,
			UserEmail:   userEmail,
			RawQuery:    rawQuery,
			Attribution: attribution,
		})
	}
	if err := visitRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read visitor rows")
		return
	}

	sourceRows, err := h.pool.Query(r.Context(), `
SELECT DISTINCT source_code
FROM landing_visits
WHERE created_at >= NOW() - make_interval(days => $1)
ORDER BY source_code ASC`, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load visitor source options")
		return
	}
	defer sourceRows.Close()
	for sourceRows.Next() {
		var option string
		if err := sourceRows.Scan(&option); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan visitor source options")
			return
		}
		resp.SourceOptions = append(resp.SourceOptions, option)
	}
	if err := sourceRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read visitor source options")
		return
	}

	campaignRows, err := h.pool.Query(r.Context(), `
SELECT DISTINCT attribution->>'utm_campaign'
FROM landing_visits
WHERE created_at >= NOW() - make_interval(days => $1)
  AND NULLIF(attribution->>'utm_campaign', '') IS NOT NULL
ORDER BY 1 ASC`, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load visitor campaign options")
		return
	}
	defer campaignRows.Close()
	for campaignRows.Next() {
		var option string
		if err := campaignRows.Scan(&option); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan visitor campaign options")
			return
		}
		resp.CampaignOptions = append(resp.CampaignOptions, option)
	}
	if err := campaignRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to read visitor campaign options")
		return
	}

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

func (h *LandingAttributionHandler) resolveSourceWithAttribution(explicitSource string, referrer string, attribution map[string]string) string {
	if code := normalizeLandingCode(attribution["utm_source"]); code != "" {
		if _, ok := h.sources[code]; ok && code != "direct" {
			return code
		}
		if _, ok := h.sources[code]; !ok {
			return "o"
		}
	}
	if code := normalizeLandingCode(attribution["r"]); code != "" {
		if _, ok := h.sources[code]; ok && code != "direct" {
			return code
		}
	}
	return h.resolveSource(explicitSource, referrer)
}

func (h *LandingAttributionHandler) labelFor(code string) string {
	if cfg, ok := h.sources[code]; ok && cfg.Label != "" {
		return cfg.Label
	}
	return strings.ToUpper(code)
}

var landingCodePattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

var landingSourceAliases = map[string]string{
	"producthunt":   "ph",
	"product_hunt":  "ph",
	"ph":            "ph",
	"twitter":       "x",
	"x":             "x",
	"t.co":          "x",
	"reddit":        "rd",
	"redd.it":       "rd",
	"rd":            "rd",
	"indiehackers":  "ih",
	"indie_hackers": "ih",
	"ih":            "ih",
	"google":        "google",
	"googleads":     "google",
	"google_ads":    "google",
	"meta":          "meta",
	"facebook":      "meta",
	"instagram":     "meta",
	"microsoft":     "microsoft",
	"bing":          "microsoft",
	"direct":        "direct",
}

func normalizeLandingCode(raw string) string {
	code := strings.ToLower(strings.TrimSpace(raw))
	if code == "" {
		return ""
	}
	aliasKey := strings.NewReplacer(" ", "_", "-", "_").Replace(code)
	if canonical, ok := landingSourceAliases[aliasKey]; ok {
		return canonical
	}
	if canonical, ok := landingSourceAliases[code]; ok {
		return canonical
	}
	if !landingCodePattern.MatchString(code) {
		return ""
	}
	return code
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

func sanitizeLandingAttribution(raw map[string]string) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}
	allowed := map[string]bool{
		"r":            true,
		"utm_source":   true,
		"utm_medium":   true,
		"utm_campaign": true,
		"s":            true,
		"m":            true,
		"c":            true,
	}
	aliases := map[string]string{
		"s": "utm_source",
		"m": "utm_medium",
		"c": "utm_campaign",
	}
	out := make(map[string]string, len(raw))
	for key, value := range raw {
		key = strings.ToLower(strings.TrimSpace(key))
		if !allowed[key] {
			continue
		}
		if alias, ok := aliases[key]; ok {
			key = alias
		}
		value = sanitizeLandingText(value, 128)
		if value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func sanitizeLandingText(raw string, max int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if r < 32 || r == 127 {
			continue
		}
		b.WriteRune(r)
		if b.Len() >= max {
			break
		}
	}
	return b.String()
}

func landingCountryCodeFromRequest(r *http.Request, bodyCode string) string {
	for _, raw := range []string{
		bodyCode,
		r.Header.Get("X-Vercel-IP-Country"),
		r.Header.Get("CF-IPCountry"),
		r.Header.Get("CloudFront-Viewer-Country"),
		r.Header.Get("X-Country-Code"),
	} {
		if code := normalizeLandingCountryCode(raw); code != "" {
			return code
		}
	}
	return ""
}

func normalizeLandingCountryCode(raw string) string {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if len(code) != 2 || code == "XX" || code == "T1" {
		return ""
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return ""
		}
	}
	return code
}

func isLandingBotUserAgent(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	if ua == "" {
		return false
	}
	for _, needle := range []string{
		"bot",
		"crawler",
		"spider",
		"facebookexternalhit",
		"linkedinbot",
		"whatsapp",
		"telegrambot",
	} {
		if strings.Contains(ua, needle) {
			return true
		}
	}
	return false
}
