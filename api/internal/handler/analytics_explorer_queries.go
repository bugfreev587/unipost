package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (h *AnalyticsExplorerHandler) queryAnalyticsPosts(ctx context.Context, filters analyticsPostFilters) ([]analyticsPostRow, error) {
	args := []any{filters.WorkspaceID, tsParam(filters.From), tsParam(filters.To)}
	where := []string{
		"sp.workspace_id = $1",
		"sp.deleted_at IS NULL",
		"sp.created_at >= $2",
		"sp.created_at < $3",
	}
	addTextFilter := func(column, value string) {
		if value == "" || value == "all" {
			return
		}
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addTextFilter("sa.platform", filters.Platform)
	addTextFilter("sa.profile_id", filters.ProfileID)
	addTextFilter("spr.status", filters.Status)
	addTextFilter("sa.id", filters.AccountID)
	addTextFilter("sp.id", filters.PostID)

	args = append(args, filters.Limit+1)
	limitParam := len(args)
	args = append(args, filters.Offset)
	offsetParam := len(args)

	query := fmt.Sprintf(`
		SELECT
			sp.id,
			spr.id,
			sa.id,
			sa.profile_id,
			sa.platform,
			COALESCE(spr.external_id, ''),
			COALESCE(sa.external_user_id, ''),
			spr.status,
			sp.status,
			COALESCE(sp.caption, ''),
			COALESCE(spr.url, ''),
			sp.created_at,
			COALESCE(spr.published_at, sp.published_at),
			COALESCE(pa.impressions, 0)::BIGINT,
			COALESCE(pa.reach, 0)::BIGINT,
			COALESCE(pa.likes, 0)::BIGINT,
			COALESCE(pa.comments, 0)::BIGINT,
			COALESCE(pa.shares, 0)::BIGINT,
			COALESCE(pa.saves, 0)::BIGINT,
			COALESCE(pa.clicks, 0)::BIGINT,
			COALESCE(pa.video_views, 0)::BIGINT,
			COALESCE(pa.engagement_rate, 0)::DOUBLE PRECISION,
			pa.platform_specific,
			pa.fetched_at,
			COALESCE(pa.consecutive_failures, 0)::INTEGER,
			COALESCE(pa.last_failure_reason, '')
		FROM social_post_results spr
		JOIN social_posts sp ON sp.id = spr.post_id
		JOIN social_accounts sa ON sa.id = spr.social_account_id
		LEFT JOIN post_analytics pa ON pa.social_post_result_id = spr.id
		WHERE %s
		ORDER BY %s %s, spr.id %s
		LIMIT $%d
		OFFSET $%d
	`, strings.Join(where, " AND "), filters.Sort.Expression, filters.Sort.Direction, filters.Sort.Direction, limitParam, offsetParam)

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []analyticsPostRow{}
	for rows.Next() {
		var row analyticsPostRow
		var createdAt pgtype.Timestamptz
		var publishedAt pgtype.Timestamptz
		var fetchedAt pgtype.Timestamptz
		var platformSpecific []byte
		if err := rows.Scan(
			&row.PostID,
			&row.SocialPostResultID,
			&row.SocialAccountID,
			&row.ProfileID,
			&row.Platform,
			&row.ExternalID,
			&row.ExternalUserID,
			&row.ResultStatus,
			&row.PostStatus,
			&row.Caption,
			&row.URL,
			&createdAt,
			&publishedAt,
			&row.Impressions,
			&row.Reach,
			&row.Likes,
			&row.Comments,
			&row.Shares,
			&row.Saves,
			&row.Clicks,
			&row.VideoViews,
			&row.EngagementRate,
			&platformSpecific,
			&fetchedAt,
			&row.ConsecutiveFailures,
			&row.LastFailureReason,
		); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			row.CreatedAt = createdAt.Time.UTC().Format(time.RFC3339)
		}
		if publishedAt.Valid {
			row.PublishedAt = publishedAt.Time.UTC().Format(time.RFC3339)
		}
		if fetchedAt.Valid {
			row.FetchedAt = fetchedAt.Time.UTC().Format(time.RFC3339)
		}
		if len(platformSpecific) > 0 {
			row.PlatformSpecific = map[string]any{}
			_ = jsonUnmarshal(platformSpecific, &row.PlatformSpecific)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (h *AnalyticsExplorerHandler) queryPlatformAvailability(ctx context.Context, workspaceID, profileID string, start, end time.Time) ([]analyticsPlatformAvailability, error) {
	args := []any{workspaceID, tsParam(start), tsParam(end)}
	profileClause := ""
	if profileID != "" && profileID != "all" {
		args = append(args, profileID)
		profileClause = fmt.Sprintf("AND sa.profile_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
			sa.platform,
			COUNT(DISTINCT sa.id)::BIGINT AS account_count,
			COUNT(DISTINCT sa.id) FILTER (WHERE sa.status = 'active')::BIGINT AS active_account_count,
			COUNT(DISTINCT sa.id) FILTER (WHERE sa.status = 'reconnect_required')::BIGINT AS needs_reconnect_count,
			COUNT(pa.id)::BIGINT AS analytics_row_count,
			MAX(pa.fetched_at) FILTER (WHERE COALESCE(pa.consecutive_failures, 0) = 0) AS last_successful_fetch_at,
			MAX(pa.last_failure_reason) FILTER (WHERE pa.last_failure_reason IS NOT NULL) AS last_failure_reason
		FROM social_accounts sa
		JOIN profiles p ON p.id = sa.profile_id
		LEFT JOIN social_post_results spr ON spr.social_account_id = sa.id
		LEFT JOIN social_posts sp ON sp.id = spr.post_id
			AND sp.deleted_at IS NULL
			AND sp.created_at >= $2
			AND sp.created_at < $3
		LEFT JOIN post_analytics pa ON pa.social_post_result_id = spr.id AND sp.id IS NOT NULL
		WHERE p.workspace_id = $1
		  AND sa.disconnected_at IS NULL
		  %s
		GROUP BY sa.platform
	`, profileClause)

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byPlatform := map[string]analyticsPlatformAvailability{}
	for rows.Next() {
		var row analyticsPlatformAvailability
		var lastSuccess pgtype.Timestamptz
		var lastFailure pgtype.Text
		if err := rows.Scan(
			&row.Platform,
			&row.AccountCount,
			&row.ActiveAccountCount,
			&row.NeedsReconnectCount,
			&row.AnalyticsRowCount,
			&lastSuccess,
			&lastFailure,
		); err != nil {
			return nil, err
		}
		if lastSuccess.Valid {
			row.LastSuccessfulFetchAt = lastSuccess.Time.UTC().Format(time.RFC3339)
		}
		if lastFailure.Valid {
			row.LastFailureReason = lastFailure.String
		}
		byPlatform[row.Platform] = row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	caps := analyticsPlatformCapabilities()
	out := make([]analyticsPlatformAvailability, 0, len(caps))
	for platformName, capability := range caps {
		row := byPlatform[platformName]
		row.Platform = platformName
		row.SupportedMetrics = capability.Metrics
		row.RefreshSupported = capability.RefreshSupported
		row.Notes = capability.Notes
		row.Health = analyticsPlatformHealth(row)
		out = append(out, row)
	}
	return out, nil
}

func (h *AnalyticsExplorerHandler) queryPlatformSummary(ctx context.Context, workspaceID, platformName, profileID string, start, end time.Time) (analyticsPlatformSummary, error) {
	args := []any{workspaceID, platformName, tsParam(start), tsParam(end)}
	profileClause := ""
	if profileID != "" && profileID != "all" {
		args = append(args, profileID)
		profileClause = fmt.Sprintf("AND sa.profile_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
			COUNT(DISTINCT sp.id)::BIGINT,
			COUNT(DISTINCT sa.id)::BIGINT,
			COALESCE(SUM(pa.impressions), 0)::BIGINT,
			COALESCE(SUM(pa.reach), 0)::BIGINT,
			COALESCE(SUM(pa.likes), 0)::BIGINT,
			COALESCE(SUM(pa.comments), 0)::BIGINT,
			COALESCE(SUM(pa.shares), 0)::BIGINT,
			COALESCE(SUM(pa.saves), 0)::BIGINT,
			COALESCE(SUM(pa.clicks), 0)::BIGINT,
			COALESCE(SUM(pa.video_views), 0)::BIGINT
		FROM social_post_results spr
		JOIN social_posts sp ON sp.id = spr.post_id
		JOIN social_accounts sa ON sa.id = spr.social_account_id
		LEFT JOIN post_analytics pa ON pa.social_post_result_id = spr.id
		WHERE sp.workspace_id = $1
		  AND sa.platform = $2
		  AND sp.deleted_at IS NULL
		  AND sp.created_at >= $3
		  AND sp.created_at < $4
		  %s
	`, profileClause)

	var summary analyticsPlatformSummary
	if err := h.pool.QueryRow(ctx, query, args...).Scan(
		&summary.Posts,
		&summary.Accounts,
		&summary.Impressions,
		&summary.Reach,
		&summary.Likes,
		&summary.Comments,
		&summary.Shares,
		&summary.Saves,
		&summary.Clicks,
		&summary.VideoViews,
	); err != nil {
		return analyticsPlatformSummary{}, err
	}
	summary.EngagementRate = rollupEngagementRate(summary.Impressions, summary.Likes, summary.Comments, summary.Shares, summary.Saves, summary.Clicks)
	return summary, nil
}

func (h *AnalyticsExplorerHandler) queryPlatformTrend(ctx context.Context, workspaceID, platformName, profileID string, start, end time.Time) ([]analyticsPlatformTrendRow, error) {
	args := []any{workspaceID, platformName, tsParam(start), tsParam(end)}
	profileClause := ""
	if profileID != "" && profileID != "all" {
		args = append(args, profileID)
		profileClause = fmt.Sprintf("AND sa.profile_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
			date_trunc('day', sp.created_at)::TIMESTAMPTZ AS day,
			COUNT(DISTINCT sp.id)::BIGINT,
			COALESCE(SUM(pa.impressions), 0)::BIGINT,
			COALESCE(SUM(pa.reach), 0)::BIGINT,
			COALESCE(SUM(pa.likes), 0)::BIGINT,
			COALESCE(SUM(pa.comments), 0)::BIGINT,
			COALESCE(SUM(pa.shares), 0)::BIGINT,
			COALESCE(SUM(pa.saves), 0)::BIGINT,
			COALESCE(SUM(pa.clicks), 0)::BIGINT,
			COALESCE(SUM(pa.video_views), 0)::BIGINT
		FROM social_post_results spr
		JOIN social_posts sp ON sp.id = spr.post_id
		JOIN social_accounts sa ON sa.id = spr.social_account_id
		LEFT JOIN post_analytics pa ON pa.social_post_result_id = spr.id
		WHERE sp.workspace_id = $1
		  AND sa.platform = $2
		  AND sp.deleted_at IS NULL
		  AND sp.created_at >= $3
		  AND sp.created_at < $4
		  %s
		GROUP BY day
		ORDER BY day ASC
	`, profileClause)

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []analyticsPlatformTrendRow{}
	for rows.Next() {
		var row analyticsPlatformTrendRow
		var day pgtype.Timestamptz
		if err := rows.Scan(
			&day,
			&row.Posts,
			&row.Impressions,
			&row.Reach,
			&row.Likes,
			&row.Comments,
			&row.Shares,
			&row.Saves,
			&row.Clicks,
			&row.VideoViews,
		); err != nil {
			return nil, err
		}
		if day.Valid {
			row.Date = day.Time.UTC().Format("2006-01-02")
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (h *AnalyticsExplorerHandler) queryPlatformAccounts(ctx context.Context, workspaceID, platformName, profileID string, start, end time.Time) ([]analyticsAccountAvailability, error) {
	args := []any{workspaceID, platformName, tsParam(start), tsParam(end)}
	profileClause := ""
	if profileID != "" && profileID != "all" {
		args = append(args, profileID)
		profileClause = fmt.Sprintf("AND sa.profile_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
			sa.id,
			sa.profile_id,
			COALESCE(sa.account_name, ''),
			COALESCE(sa.external_user_id, ''),
			sa.status,
			COUNT(DISTINCT spr.id) FILTER (WHERE sp.id IS NOT NULL)::BIGINT,
			MAX(pa.fetched_at) FILTER (WHERE COALESCE(pa.consecutive_failures, 0) = 0),
			MAX(pa.last_failure_reason) FILTER (WHERE pa.last_failure_reason IS NOT NULL)
		FROM social_accounts sa
		JOIN profiles p ON p.id = sa.profile_id
		LEFT JOIN social_post_results spr ON spr.social_account_id = sa.id
		LEFT JOIN social_posts sp ON sp.id = spr.post_id
			AND sp.deleted_at IS NULL
			AND sp.created_at >= $3
			AND sp.created_at < $4
		LEFT JOIN post_analytics pa ON pa.social_post_result_id = spr.id AND sp.id IS NOT NULL
		WHERE p.workspace_id = $1
		  AND sa.platform = $2
		  AND sa.disconnected_at IS NULL
		  %s
		GROUP BY sa.id, sa.profile_id, sa.account_name, sa.external_user_id, sa.status
		ORDER BY sa.account_name ASC NULLS LAST, sa.id ASC
	`, profileClause)

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []analyticsAccountAvailability{}
	for rows.Next() {
		var row analyticsAccountAvailability
		var lastSuccess pgtype.Timestamptz
		var lastFailure pgtype.Text
		if err := rows.Scan(
			&row.SocialAccountID,
			&row.ProfileID,
			&row.AccountName,
			&row.ExternalUserID,
			&row.Status,
			&row.PostCount,
			&lastSuccess,
			&lastFailure,
		); err != nil {
			return nil, err
		}
		if lastSuccess.Valid {
			row.LastSuccessfulFetchAt = lastSuccess.Time.UTC().Format(time.RFC3339)
		}
		if lastFailure.Valid {
			row.LastFailureReason = lastFailure.String
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (h *AnalyticsExplorerHandler) requestAnalyticsRefresh(ctx context.Context, workspaceID string, req analyticsRefreshRequest, start, end time.Time) (int64, int64, error) {
	args := []any{workspaceID, tsParam(start), tsParam(end)}
	where := []string{
		"sp.workspace_id = $1",
		"sp.deleted_at IS NULL",
		"sp.created_at >= $2",
		"sp.created_at < $3",
		"spr.status = 'published'",
		"spr.external_id IS NOT NULL",
		"sa.disconnected_at IS NULL",
		"sa.status = 'active'",
	}
	addTextFilter := func(column, value string) {
		if value == "" || value == "all" {
			return
		}
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addTextFilter("sa.platform", req.Platform)
	addTextFilter("sa.profile_id", req.ProfileID)
	addTextFilter("sa.id", req.AccountID)
	addTextFilter("sp.id", req.PostID)
	args = append(args, req.Limit)
	limitParam := len(args)

	query := fmt.Sprintf(`
		WITH matched AS (
			SELECT spr.id
			FROM social_post_results spr
			JOIN social_posts sp ON sp.id = spr.post_id
			JOIN social_accounts sa ON sa.id = spr.social_account_id
			WHERE %s
			ORDER BY COALESCE(spr.published_at, sp.published_at, sp.created_at) DESC
			LIMIT $%d
		),
		upserted AS (
			INSERT INTO post_analytics (social_post_result_id, fetched_at, consecutive_failures, last_failure_reason)
			SELECT id, TIMESTAMPTZ '1970-01-01 00:00:00+00', 0, NULL
			FROM matched
			ON CONFLICT (social_post_result_id) DO UPDATE
			SET fetched_at = TIMESTAMPTZ '1970-01-01 00:00:00+00',
			    last_failure_reason = NULL
			RETURNING social_post_result_id
		)
		SELECT
			(SELECT COUNT(*) FROM matched)::BIGINT,
			(SELECT COUNT(*) FROM upserted)::BIGINT
	`, strings.Join(where, " AND "), limitParam)

	var matched, requested int64
	if err := h.pool.QueryRow(ctx, query, args...).Scan(&matched, &requested); err != nil {
		return 0, 0, err
	}
	return matched, requested, nil
}

func analyticsPlatformHealth(row analyticsPlatformAvailability) string {
	if row.AccountCount == 0 {
		return "not_connected"
	}
	if row.NeedsReconnectCount > 0 && row.ActiveAccountCount == 0 {
		return "needs_reconnect"
	}
	if row.NeedsReconnectCount > 0 {
		return "partial_reconnect_required"
	}
	if row.AnalyticsRowCount == 0 {
		return "pending"
	}
	if row.LastFailureReason != "" {
		return "degraded"
	}
	return "ready"
}

func analyticsRefreshRange(fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	start := end.Add(-30 * 24 * time.Hour)
	if strings.TrimSpace(toStr) != "" {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(toStr))
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("to must be YYYY-MM-DD")
		}
		end = t.UTC().Add(24 * time.Hour)
	}
	if strings.TrimSpace(fromStr) != "" {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(fromStr))
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("from must be YYYY-MM-DD")
		}
		start = t.UTC()
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, errors.New("from must be before to")
	}
	return start, end, nil
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
