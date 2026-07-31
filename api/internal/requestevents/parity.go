package requestevents

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

const compareRequestEventHourSQL = `
WITH legacy AS (
  SELECT
    workspace_id,
	COALESCE(api_key_id, '') AS api_key_id,
    method,
    path AS route_pattern,
    status_code,
    COUNT(*)::BIGINT AS event_count
  FROM api_metrics
  WHERE created_at >= $1 AND created_at < $2
	GROUP BY workspace_id, COALESCE(api_key_id, ''), method, path, status_code
), canonical AS (
  SELECT
    workspace_id,
	api_key_id,
    method,
    route_pattern,
    status_code,
    COUNT(*)::BIGINT AS event_count
  FROM api_request_events
	WHERE created_at >= $1 AND created_at < $2
	GROUP BY workspace_id, api_key_id, method, route_pattern, status_code
), compared AS (
  SELECT
    COALESCE(legacy.event_count, 0)::BIGINT AS legacy_count,
    COALESCE(canonical.event_count, 0)::BIGINT AS canonical_count
  FROM legacy
	FULL OUTER JOIN canonical USING (workspace_id, api_key_id, method, route_pattern, status_code)
)
SELECT
  COALESCE(SUM(legacy_count), 0)::BIGINT,
  COALESCE(SUM(canonical_count), 0)::BIGINT,
  COUNT(*) FILTER (WHERE legacy_count <> canonical_count)::BIGINT,
  COALESCE(SUM(ABS(legacy_count - canonical_count)), 0)::BIGINT
FROM compared`

const compareControlledCohortSQL = `
SELECT
  (
    SELECT COUNT(*)::BIGINT
    FROM api_metrics
    WHERE workspace_id = $1
      AND COALESCE(api_key_id, '') = $2
      AND method = $3
      AND path = $4
      AND status_code = $5
  ),
  (
    SELECT COUNT(*)::BIGINT
    FROM api_request_events
    WHERE workspace_id = $1
      AND api_key_id = $2
      AND method = $3
      AND route_pattern = $4
      AND status_code = $5
  )`

type ParityResult struct {
	LegacyCount      int64
	CanonicalCount   int64
	MismatchedGroups int64
	AbsoluteDelta    int64
}

type parityQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresParityStore struct {
	database parityQueryer
}

func NewPostgresParityStore(database parityQueryer) *PostgresParityStore {
	return &PostgresParityStore{database: database}
}

// CompareWindow is for controlled operational validation windows only. Both
// sides use database insertion time; it is intentionally not a background
// hour-bucket alert because independent async writers can straddle boundaries.
func (s *PostgresParityStore) CompareWindow(ctx context.Context, start, end time.Time) (ParityResult, error) {
	var result ParityResult
	err := s.database.QueryRow(ctx, compareRequestEventHourSQL, start, end).Scan(
		&result.LegacyCount,
		&result.CanonicalCount,
		&result.MismatchedGroups,
		&result.AbsoluteDelta,
	)
	return result, err
}

// CompareControlledCohort provides exact acceptance evidence without a time
// boundary. The caller must use a newly created, otherwise-unused API key and
// one known route/method/status cohort, then wait for both async writers to
// settle before comparing its lifetime counts.
func (s *PostgresParityStore) CompareControlledCohort(
	ctx context.Context,
	workspaceID, apiKeyID, method, routePattern string,
	statusCode int,
) (ParityResult, error) {
	var result ParityResult
	err := s.database.QueryRow(
		ctx,
		compareControlledCohortSQL,
		workspaceID,
		apiKeyID,
		method,
		routePattern,
		statusCode,
	).Scan(&result.LegacyCount, &result.CanonicalCount)
	if err != nil {
		return result, err
	}
	if result.LegacyCount != result.CanonicalCount {
		result.MismatchedGroups = 1
		delta := result.LegacyCount - result.CanonicalCount
		if delta < 0 {
			delta = -delta
		}
		result.AbsoluteDelta = delta
	}
	return result, nil
}
