package observabilityreads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type PublicEvaluator interface {
	Public(context.Context, string) (bool, error)
}

type ReadSelector struct {
	evaluator PublicEvaluator
	logger    *slog.Logger
}

func NewReadSelector(evaluator PublicEvaluator, logger *slog.Logger) *ReadSelector {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReadSelector{evaluator: evaluator, logger: logger}
}

func (s *ReadSelector) Evaluate(ctx context.Context) (bool, error) {
	if s == nil || interfaceIsNil(s.evaluator) {
		return false, nil
	}
	return s.evaluator.Public(ctx, featureflags.ObservabilityReadsV2)
}

func (s *ReadSelector) UseV2(ctx context.Context) bool {
	enabled, err := s.Evaluate(ctx)
	if err != nil {
		s.logger.WarnContext(ctx, "observability reads v2 evaluation failed; using legacy reads",
			"event", "observability_reads_v2_evaluation_failed",
			"feature_flag", featureflags.ObservabilityReadsV2,
			"error", err,
		)
		return false
	}
	return enabled
}

func interfaceIsNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

type LogReader interface {
	ListWorkspace(context.Context, string, LogFilters, *LogCursor, int) (LogPage, error)
	ListAdmin(context.Context, LogFilters, *LogCursor, int) (LogPage, error)
	GetWorkspace(context.Context, string, string) (LogDetail, error)
	GetAdmin(context.Context, string) (LogDetail, error)
}

type ReadPathSelector interface {
	UseV2(context.Context) bool
}

// SelectLogReader returns nil for every unavailable or disabled optional
// dependency, including interfaces that contain typed-nil pointers.
func SelectLogReader(ctx context.Context, selector ReadPathSelector, reader LogReader) LogReader {
	if interfaceIsNil(selector) || interfaceIsNil(reader) || !selector.UseV2(ctx) {
		return nil
	}
	return reader
}

type postgresLogDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresLogStore struct {
	db postgresLogDB
}

func NewPostgresLogStore(db postgresLogDB) *PostgresLogStore {
	return &PostgresLogStore{db: db}
}

const requestLogProjectionSQL = `
SELECT
  e.id,
  e.workspace_id,
  COALESCE(w.name, ''),
  COALESCE(u.email, ''),
  COALESCE((
    SELECT s.plan_id
    FROM subscriptions s
    WHERE s.workspace_id = e.workspace_id
    ORDER BY s.updated_at DESC
    LIMIT 1
  ), ''),
  e.occurred_at,
  CASE WHEN e.status_code >= 500 THEN 'error' WHEN e.status_code >= 400 THEN 'warn' ELSE 'info' END,
  CASE WHEN e.outcome = 'success' THEN 'success' ELSE 'error' END,
  'api_request',
  CASE
    WHEN e.outcome = 'success' THEN 'api.request.succeeded'
    WHEN e.outcome = 'validation_error' THEN 'api.request.validation_failed'
    WHEN e.outcome = 'rate_limited' THEN 'api.request.rate_limited'
    ELSE 'api.request.failed'
  END,
  'api',
  e.method || ' ' || e.route_pattern || ' returned ' || e.status_code::TEXT || '.',
  COALESCE(e.request_id, ''),
  COALESCE(e.trace_id, ''),
  '',
  e.api_key_id,
  COALESCE(e.profile_id, ''),
  COALESCE(e.social_account_id, ''),
  COALESCE(e.post_id, ''),
  '',
  '',
  e.route_pattern,
  e.method,
  e.status_code,
  e.duration_ms,
  COALESCE(e.error_code, ''),
  '{}'::JSONB,
  e.has_error_detail
FROM api_request_events e
LEFT JOIN workspaces w ON w.id = e.workspace_id
LEFT JOIN users u ON u.id = w.user_id
`

const integrationLogProjectionSQL = `
SELECT
  l.id,
  l.workspace_id,
  COALESCE(w.name, ''),
  COALESCE(u.email, ''),
  COALESCE((
    SELECT s.plan_id
    FROM subscriptions s
    WHERE s.workspace_id = l.workspace_id
    ORDER BY s.updated_at DESC
    LIMIT 1
  ), ''),
  l.ts,
  l.level,
  l.status,
  l.category,
  l.action,
  l.source,
  l.message,
  COALESCE(l.request_id, ''),
  COALESCE(l.trace_id, ''),
  COALESCE(l.actor_user_id, ''),
  COALESCE(l.actor_api_key_id, ''),
  COALESCE(l.profile_id, ''),
  COALESCE(l.social_account_id, ''),
  COALESCE(l.post_id, ''),
  COALESCE(l.platform_post_id, ''),
  COALESCE(l.platform, ''),
  COALESCE(l.endpoint, ''),
  COALESCE(l.method, ''),
  l.http_status_code,
  l.remote_status_code,
  l.duration_ms,
  COALESCE(l.error_code, ''),
  l.metadata
FROM integration_logs l
LEFT JOIN workspaces w ON w.id = l.workspace_id
LEFT JOIN users u ON u.id = w.user_id
`

const requestLogListSQL = requestLogProjectionSQL + `
WHERE ($1::TEXT = '' OR e.workspace_id = $1)
  AND ($2::TEXT = '' OR u.email ILIKE '%' || $2 || '%')
  AND ($3::TEXT = '' OR $3 = 'api_request')
  AND ($4::TEXT = '' OR CASE
    WHEN e.outcome = 'success' THEN 'api.request.succeeded'
    WHEN e.outcome = 'validation_error' THEN 'api.request.validation_failed'
    WHEN e.outcome = 'rate_limited' THEN 'api.request.rate_limited'
    ELSE 'api.request.failed'
  END = $4)
  AND ($5::TEXT = '' OR $5 = 'api')
  AND ($6::TEXT = '' OR CASE WHEN e.status_code >= 500 THEN 'error' WHEN e.status_code >= 400 THEN 'warn' ELSE 'info' END = $6)
  AND ($7::TEXT = '' OR CASE WHEN e.outcome = 'success' THEN 'success' ELSE 'error' END = $7)
  AND $8::TEXT = ''
  AND ($9::TEXT = '' OR e.profile_id = $9)
  AND ($10::TEXT = '' OR e.social_account_id = $10)
  AND ($11::TEXT = '' OR e.post_id = $11)
  AND ($12::TEXT = '' OR e.request_id = $12)
  AND ($13::TEXT = '' OR e.error_code = $13)
  AND (
    $14::TEXT = ''
    OR e.route_pattern ILIKE '%' || $14 || '%'
    OR CASE
      WHEN e.outcome = 'success' THEN 'api.request.succeeded'
      WHEN e.outcome = 'validation_error' THEN 'api.request.validation_failed'
      WHEN e.outcome = 'rate_limited' THEN 'api.request.rate_limited'
      ELSE 'api.request.failed'
    END ILIKE '%' || $14 || '%'
    OR e.request_id ILIKE '%' || $14 || '%'
    OR e.post_id ILIKE '%' || $14 || '%'
    OR e.error_code ILIKE '%' || $14 || '%'
	OR u.email ILIKE '%' || $14 || '%'
  )
  AND ($15::TEXT = '' OR e.method = $15)
  AND ($16::TEXT = '' OR e.route_pattern = $16)
  AND e.occurred_at >= $17
  AND e.occurred_at <= $18
  AND (
    $19::TIMESTAMPTZ IS NULL
    OR e.occurred_at < $19
    OR (e.occurred_at = $19 AND $20::TEXT = 'request' AND e.id < $21::TEXT)
  )
ORDER BY e.occurred_at DESC, e.id DESC
LIMIT $22`

const integrationLogListSQL = integrationLogProjectionSQL + `
WHERE l.category <> 'api_request'
  AND ($1::TEXT = '' OR l.workspace_id = $1)
  AND ($2::TEXT = '' OR u.email ILIKE '%' || $2 || '%')
  AND ($3::TEXT = '' OR l.category = $3)
  AND ($4::TEXT = '' OR l.action = $4)
  AND ($5::TEXT = '' OR l.source = $5)
  AND ($6::TEXT = '' OR l.level = $6)
  AND ($7::TEXT = '' OR l.status = $7)
  AND ($8::TEXT = '' OR l.platform = $8)
  AND ($9::TEXT = '' OR l.profile_id = $9)
  AND ($10::TEXT = '' OR l.social_account_id = $10)
  AND ($11::TEXT = '' OR l.post_id = $11)
  AND ($12::TEXT = '' OR l.request_id = $12)
  AND ($13::TEXT = '' OR l.error_code = $13)
  AND (
    $14::TEXT = ''
    OR l.message ILIKE '%' || $14 || '%'
    OR l.action ILIKE '%' || $14 || '%'
    OR l.request_id ILIKE '%' || $14 || '%'
    OR l.post_id ILIKE '%' || $14 || '%'
    OR l.error_code ILIKE '%' || $14 || '%'
    OR l.metadata->>'connect_session_id' = $14
    OR l.metadata->>'external_user_id' = $14
	OR u.email ILIKE '%' || $14 || '%'
  )
  AND ($15::TEXT = '' OR l.method = $15)
  AND ($16::TEXT = '' OR l.endpoint = $16)
  AND l.ts >= $17
  AND l.ts <= $18
  AND (
    $19::TIMESTAMPTZ IS NULL
    OR l.ts < $19
    OR (
      l.ts = $19
      AND (
        $20::TEXT = 'request'
        OR ($20::TEXT = 'integration' AND l.id < $21::BIGINT)
      )
    )
  )
ORDER BY l.ts DESC, l.id DESC
LIMIT $22`

func (s *PostgresLogStore) ListWorkspace(ctx context.Context, workspaceID string, filters LogFilters, after *LogCursor, limit int) (LogPage, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return LogPage{}, errors.New("workspace ID is required")
	}
	filters.WorkspaceID = workspaceID
	filters.OwnerEmail = ""
	return s.list(ctx, filters, after, limit)
}

func (s *PostgresLogStore) ListAdmin(ctx context.Context, filters LogFilters, after *LogCursor, limit int) (LogPage, error) {
	return s.list(ctx, filters, after, limit)
}

func (s *PostgresLogStore) list(ctx context.Context, filters LogFilters, after *LogCursor, limit int) (LogPage, error) {
	if s == nil || s.db == nil {
		return LogPage{}, errors.New("observability log store is not configured")
	}
	if filters.From.IsZero() || filters.To.IsZero() || filters.From.After(filters.To) {
		return LogPage{}, errors.New("valid log time range is required")
	}
	if limit <= 0 {
		return LogPage{}, errors.New("page limit must be positive")
	}
	if after != nil {
		if after.Timestamp.IsZero() {
			return LogPage{}, errors.New("cursor timestamp is required")
		}
		if err := validateSourceIdentity(after.SourceKind, after.SourceID); err != nil {
			return LogPage{}, err
		}
	}

	requestArgs := logListArgs(filters, after, limit+1, LogSourceRequest)
	requests, err := s.queryRequests(ctx, requestLogListSQL, requestArgs...)
	if err != nil {
		return LogPage{}, err
	}
	integrationArgs := logListArgs(filters, after, limit+1, LogSourceIntegration)
	integrations, err := s.queryIntegrations(ctx, integrationLogListSQL, integrationArgs...)
	if err != nil {
		return LogPage{}, err
	}
	return MergeLogPage(requests, integrations, after, limit)
}

func logListArgs(filters LogFilters, after *LogCursor, candidateLimit int, querySource LogSourceKind) []any {
	var cursorTime any
	cursorKind := ""
	var cursorID any = ""
	if after != nil {
		cursorTime = after.Timestamp.UTC()
		cursorKind = string(after.SourceKind)
		if querySource == LogSourceIntegration {
			cursorID = int64(0)
			if after.SourceKind == LogSourceIntegration {
				cursorID, _ = strconv.ParseInt(after.SourceID, 10, 64)
			}
		} else if after.SourceKind == LogSourceRequest {
			cursorID = after.SourceID
		}
	} else if querySource == LogSourceIntegration {
		cursorID = int64(0)
	}
	return []any{
		strings.TrimSpace(filters.WorkspaceID),
		strings.TrimSpace(filters.OwnerEmail),
		strings.TrimSpace(filters.Category),
		strings.TrimSpace(filters.Action),
		strings.TrimSpace(filters.Source),
		strings.TrimSpace(filters.Level),
		strings.TrimSpace(filters.Status),
		strings.TrimSpace(filters.Platform),
		strings.TrimSpace(filters.ProfileID),
		strings.TrimSpace(filters.SocialAccountID),
		strings.TrimSpace(filters.PostID),
		strings.TrimSpace(filters.RequestID),
		strings.TrimSpace(filters.ErrorCode),
		strings.TrimSpace(filters.Query),
		strings.TrimSpace(filters.Method),
		strings.TrimSpace(filters.Endpoint),
		filters.From.UTC(),
		filters.To.UTC(),
		cursorTime,
		cursorKind,
		cursorID,
		candidateLimit,
	}
}

func (s *PostgresLogStore) queryRequests(ctx context.Context, query string, args ...any) ([]LogProjection, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LogProjection, 0)
	for rows.Next() {
		item, err := scanRequestProjection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresLogStore) queryIntegrations(ctx context.Context, query string, args ...any) ([]LogProjection, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LogProjection, 0)
	for rows.Next() {
		item, err := scanIntegrationProjection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type rowScanner interface {
	Scan(...any) error
}

func scanRequestProjection(row rowScanner) (LogProjection, error) {
	var item LogProjection
	var httpStatus, duration int32
	if err := row.Scan(
		&item.SourceID, &item.WorkspaceID, &item.WorkspaceName, &item.OwnerEmail, &item.PlanID,
		&item.Timestamp, &item.Level, &item.Status, &item.Category, &item.Action, &item.Source, &item.Message,
		&item.RequestID, &item.TraceID, &item.ActorUserID, &item.ActorAPIKeyID,
		&item.ProfileID, &item.SocialAccountID, &item.PostID, &item.PlatformPostID, &item.Platform,
		&item.Endpoint, &item.Method, &httpStatus, &duration, &item.ErrorCode, &item.Metadata, &item.HasErrorDetail,
	); err != nil {
		return LogProjection{}, err
	}
	item.SourceKind = LogSourceRequest
	item.HTTPStatusCode = &httpStatus
	item.DurationMs = &duration
	return item, nil
}

func scanIntegrationProjection(row rowScanner) (LogProjection, error) {
	var item LogProjection
	var id int64
	if err := row.Scan(
		&id, &item.WorkspaceID, &item.WorkspaceName, &item.OwnerEmail, &item.PlanID,
		&item.Timestamp, &item.Level, &item.Status, &item.Category, &item.Action, &item.Source, &item.Message,
		&item.RequestID, &item.TraceID, &item.ActorUserID, &item.ActorAPIKeyID,
		&item.ProfileID, &item.SocialAccountID, &item.PostID, &item.PlatformPostID, &item.Platform,
		&item.Endpoint, &item.Method, &item.HTTPStatusCode, &item.RemoteStatusCode, &item.DurationMs,
		&item.ErrorCode, &item.Metadata,
	); err != nil {
		return LogProjection{}, err
	}
	item.SourceKind = LogSourceIntegration
	item.SourceID = strconv.FormatInt(id, 10)
	return item, nil
}

const requestLogWorkspaceBaseSQL = requestLogProjectionSQL + `
WHERE e.occurred_at = $1 AND e.id = $2 AND e.workspace_id = $3
LIMIT 1`

const requestLogAdminBaseSQL = requestLogProjectionSQL + `
WHERE e.occurred_at = $1 AND e.id = $2
ORDER BY e.workspace_id
LIMIT 1`

const integrationLogWorkspaceBaseSQL = integrationLogProjectionSQL + `
WHERE l.id = $1 AND l.workspace_id = $2 AND l.category <> 'api_request'
LIMIT 1`

const integrationLogAdminBaseSQL = integrationLogProjectionSQL + `
WHERE l.id = $1 AND l.category <> 'api_request'
LIMIT 1`

const requestLogDetailSQL = `
SELECT
  d.request_headers,
  d.response_headers,
  d.query_keys,
  d.request_payload,
  d.response_payload,
  d.request_original_bytes,
  d.request_stored_bytes,
  d.response_original_bytes,
  d.response_stored_bytes,
  COALESCE(d.request_content_type, ''),
  COALESCE(d.response_content_type, ''),
  COALESCE(d.request_sha256, ''),
  COALESCE(d.response_sha256, ''),
  d.request_truncated,
  d.response_truncated,
  COALESCE(d.request_omission_reason, ''),
  COALESCE(d.response_omission_reason, ''),
  d.redaction_policy_version
FROM api_request_error_details d
WHERE d.occurred_at = $1 AND d.event_id = $2 AND d.workspace_id = $3
LIMIT 1`

const integrationLogDetailSQL = `
SELECT request_payload, response_payload
FROM integration_logs
WHERE id = $1 AND workspace_id = $2 AND category <> 'api_request'
LIMIT 1`

func (s *PostgresLogStore) GetWorkspace(ctx context.Context, workspaceID, id string) (LogDetail, error) {
	if s == nil || s.db == nil {
		return LogDetail{}, errors.New("observability log store is not configured")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return LogDetail{}, pgx.ErrNoRows
	}
	kind, sourceID, err := DecodeLogLookupID(id)
	if err != nil {
		return LogDetail{}, pgx.ErrNoRows
	}
	switch kind {
	case LogSourceRequest:
		occurredAt, err := requestIDTimestamp(sourceID)
		if err != nil {
			return LogDetail{}, pgx.ErrNoRows
		}
		base, err := scanRequestProjection(s.db.QueryRow(ctx, requestLogWorkspaceBaseSQL, occurredAt, sourceID, workspaceID))
		if err != nil {
			return LogDetail{}, err
		}
		return s.loadRequestDetail(ctx, base)
	case LogSourceIntegration:
		localID, _ := strconv.ParseInt(sourceID, 10, 64)
		base, err := scanIntegrationProjection(s.db.QueryRow(ctx, integrationLogWorkspaceBaseSQL, localID, workspaceID))
		if err != nil {
			return LogDetail{}, err
		}
		return s.loadIntegrationDetail(ctx, base)
	default:
		return LogDetail{}, pgx.ErrNoRows
	}
}

func (s *PostgresLogStore) GetAdmin(ctx context.Context, id string) (LogDetail, error) {
	if s == nil || s.db == nil {
		return LogDetail{}, errors.New("observability log store is not configured")
	}
	kind, sourceID, err := DecodeLogLookupID(id)
	if err != nil {
		return LogDetail{}, pgx.ErrNoRows
	}
	switch kind {
	case LogSourceRequest:
		occurredAt, err := requestIDTimestamp(sourceID)
		if err != nil {
			return LogDetail{}, pgx.ErrNoRows
		}
		base, err := scanRequestProjection(s.db.QueryRow(ctx, requestLogAdminBaseSQL, occurredAt, sourceID))
		if err != nil {
			return LogDetail{}, err
		}
		return s.loadRequestDetail(ctx, base)
	case LogSourceIntegration:
		localID, _ := strconv.ParseInt(sourceID, 10, 64)
		base, err := scanIntegrationProjection(s.db.QueryRow(ctx, integrationLogAdminBaseSQL, localID))
		if err != nil {
			return LogDetail{}, err
		}
		return s.loadIntegrationDetail(ctx, base)
	default:
		return LogDetail{}, pgx.ErrNoRows
	}
}

func (s *PostgresLogStore) loadRequestDetail(ctx context.Context, base LogProjection) (LogDetail, error) {
	if err := normalizeLogProjection(&base, LogSourceRequest); err != nil {
		return LogDetail{}, err
	}
	detail := LogDetail{LogProjection: base}
	if base.Status == "success" {
		return detail, nil
	}
	var requestPayload, responsePayload *string
	err := s.db.QueryRow(ctx, requestLogDetailSQL, base.Timestamp, base.SourceID, base.WorkspaceID).Scan(
		&detail.RequestHeaders,
		&detail.ResponseHeaders,
		&detail.QueryKeys,
		&requestPayload,
		&responsePayload,
		&detail.RequestOriginalBytes,
		&detail.RequestStoredBytes,
		&detail.ResponseOriginalBytes,
		&detail.ResponseStoredBytes,
		&detail.RequestContentType,
		&detail.ResponseContentType,
		&detail.RequestSHA256,
		&detail.ResponseSHA256,
		&detail.RequestTruncated,
		&detail.ResponseTruncated,
		&detail.RequestOmissionReason,
		&detail.ResponseOmissionReason,
		&detail.RedactionPolicyVersion,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		detail.DetailStatus = "expired"
		return detail, nil
	}
	if err != nil {
		return LogDetail{}, err
	}
	detail.DetailStatus = "available"
	detail.RequestPayload = textPayloadJSON(requestPayload)
	detail.ResponsePayload = textPayloadJSON(responsePayload)
	return detail, nil
}

func (s *PostgresLogStore) loadIntegrationDetail(ctx context.Context, base LogProjection) (LogDetail, error) {
	if err := normalizeLogProjection(&base, LogSourceIntegration); err != nil {
		return LogDetail{}, err
	}
	detail := LogDetail{LogProjection: base}
	localID, _ := strconv.ParseInt(base.SourceID, 10, 64)
	if err := s.db.QueryRow(ctx, integrationLogDetailSQL, localID, base.WorkspaceID).Scan(&detail.RequestPayload, &detail.ResponsePayload); err != nil {
		return LogDetail{}, err
	}
	return detail, nil
}

func requestIDTimestamp(id string) (time.Time, error) {
	prefix, _, ok := strings.Cut(id, "_")
	if !ok || len(prefix) != 20 {
		return time.Time{}, errors.New("request event ID has no timestamp prefix")
	}
	micros, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse request event timestamp: %w", err)
	}
	return time.UnixMicro(micros).UTC(), nil
}

func textPayloadJSON(value *string) json.RawMessage {
	if value == nil {
		return nil
	}
	bytes := []byte(*value)
	if json.Valid(bytes) {
		return json.RawMessage(bytes)
	}
	encoded, _ := json.Marshal(*value)
	return json.RawMessage(encoded)
}
