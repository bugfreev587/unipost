package socialconnectioncutover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type PreflightReader interface {
	ReadCounts(context.Context) (Counts, error)
	ReadConflicts(context.Context) ([]Conflict, error)
	ReadAliasWarnings(context.Context) ([]AliasWarning, error)
	ReadRelationSizes(context.Context) ([]RelationSize, error)
}

type Preflight struct {
	reader PreflightReader
	now    func() time.Time
}

func NewPreflight(reader PreflightReader, now func() time.Time) *Preflight {
	if now == nil {
		now = time.Now
	}
	return &Preflight{reader: reader, now: now}
}

func (p *Preflight) Run(ctx context.Context, mode string) (Report, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "expand" && mode != "cutover" {
		return Report{}, fmt.Errorf("unsupported social connection preflight mode %q", mode)
	}
	if p == nil || p.reader == nil {
		return Report{}, errors.New("social connection preflight reader is required")
	}

	counts, err := p.reader.ReadCounts(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("read social connection preflight counts: %w", err)
	}
	conflicts, err := p.reader.ReadConflicts(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("classify social connection conflicts: %w", err)
	}
	warnings, err := p.reader.ReadAliasWarnings(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("read social connection alias warnings: %w", err)
	}
	relations, err := p.reader.ReadRelationSizes(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("read social connection relation sizes: %w", err)
	}
	counts.ConflictGroups = int64(len(conflicts))
	counts.AliasWarnings = int64(len(warnings))

	blockers := make([]Blocker, 0, 2)
	if mode == "cutover" && counts.ActiveDeliveryJobs > 0 {
		blockers = append(blockers, Blocker{
			Code: "active_delivery_jobs", Count: counts.ActiveDeliveryJobs,
			Message: "active delivery jobs must drain before authority cutover",
		})
	}
	if mode == "cutover" && counts.PublishableNull > 0 {
		blockers = append(blockers, Blocker{
			Code: "publishable_null_bindings", Count: counts.PublishableNull,
			Message: "publishable bindings still lack connection authority",
		})
	}

	return Report{
		GeneratedAt:   p.now().UTC(),
		Mode:          mode,
		Counts:        counts,
		Conflicts:     conflicts,
		AliasWarnings: warnings,
		Relations:     relations,
		Blockers:      blockers,
	}, nil
}

type Queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresPreflightReader struct {
	db Queryer
}

func NewPostgresPreflightReader(db Queryer) *PostgresPreflightReader {
	return &PostgresPreflightReader{db: db}
}

func (r *PostgresPreflightReader) ReadCounts(ctx context.Context) (Counts, error) {
	var counts Counts
	err := r.db.QueryRow(ctx, preflightCountsSQL).Scan(
		&counts.Accounts,
		&counts.Connections,
		&counts.PublishableNull,
		&counts.MissingIGIdentity,
		&counts.ActiveDeliveryJobs,
	)
	return counts, err
}

func (r *PostgresPreflightReader) ReadConflicts(ctx context.Context) ([]Conflict, error) {
	rows, err := r.db.Query(ctx, preflightConflictsSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	conflicts := make([]Conflict, 0)
	for rows.Next() {
		var conflict Conflict
		var providerIdentity *string
		var classificationJSON []byte
		if err := rows.Scan(
			&conflict.WorkspaceID,
			&conflict.Platform,
			&providerIdentity,
			&conflict.Reason,
			&conflict.AccountIDs,
			&conflict.ProfileIDs,
			&conflict.ExternalUserIDs,
			&conflict.ConnectionTypes,
			&classificationJSON,
		); err != nil {
			return nil, err
		}
		if providerIdentity != nil {
			conflict.ProviderIdentity = *providerIdentity
		}
		if len(classificationJSON) > 0 {
			if err := json.Unmarshal(classificationJSON, &conflict.Classification); err != nil {
				return nil, fmt.Errorf("decode conflict classification counts: %w", err)
			}
		}
		conflicts = append(conflicts, conflict)
	}
	return conflicts, rows.Err()
}

func (r *PostgresPreflightReader) ReadAliasWarnings(ctx context.Context) ([]AliasWarning, error) {
	rows, err := r.db.Query(ctx, preflightAliasWarningsSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	warnings := make([]AliasWarning, 0)
	for rows.Next() {
		var warning AliasWarning
		if err := rows.Scan(
			&warning.WorkspaceID,
			&warning.Platform,
			&warning.ConnectionIDs,
			&warning.ProviderIdentities,
			&warning.SharedIdentifiers,
		); err != nil {
			return nil, err
		}
		warnings = append(warnings, warning)
	}
	return warnings, rows.Err()
}

func (r *PostgresPreflightReader) ReadRelationSizes(ctx context.Context) ([]RelationSize, error) {
	rows, err := r.db.Query(ctx, preflightRelationSizesSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	relations := make([]RelationSize, 0)
	for rows.Next() {
		var relation RelationSize
		if err := rows.Scan(&relation.Name, &relation.Bytes, &relation.Rows); err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

const preflightCountsSQL = `
SELECT
  (SELECT COUNT(*) FROM social_accounts),
  (SELECT COUNT(*) FROM social_connections),
  (SELECT COUNT(*) FROM social_accounts
    WHERE connection_id IS NULL
      AND binding_status = 'active'
      AND status = 'active'
      AND disconnected_at IS NULL),
  (SELECT COUNT(*) FROM social_accounts
    WHERE platform = 'instagram'
      AND NULLIF(BTRIM(metadata->>'instagram_webhook_user_id'), '') IS NULL),
  (SELECT COUNT(*) FROM post_delivery_jobs
    WHERE state IN ('pending', 'running', 'retrying'))
`

const preflightConflictsSQL = `
WITH source AS (
  SELECT
    sa.id AS account_id,
    sa.profile_id,
    p.workspace_id,
    sa.platform,
    sa.connection_type,
    sa.external_user_id,
    COALESCE(sa.x_app_mode, '') AS x_app_mode,
    sa.access_token,
    COALESCE(sa.refresh_token, '') AS refresh_token,
    sa.token_expires_at,
    TO_JSONB(ARRAY(
      SELECT DISTINCT item
      FROM UNNEST(COALESCE(sa.scope, ARRAY[]::TEXT[])) AS item
      ORDER BY item
    )) AS authority_scope,
    COALESCE(sa.metadata, '{}'::JSONB)
      - 'dismissed_at'
      - 'disconnect_notified_at'
      - 'reconnect_required_at' AS authority_metadata,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      ELSE NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
), missing AS (
  SELECT
    workspace_id,
    platform,
    NULL::TEXT AS provider_identity,
    'missing_provider_identity'::TEXT AS reason,
    ARRAY[account_id] AS account_ids,
    ARRAY[profile_id] AS profile_ids,
    CASE WHEN external_user_id IS NULL THEN ARRAY[]::TEXT[] ELSE ARRAY[external_user_id] END AS external_user_ids,
    ARRAY[connection_type] AS connection_types,
    '{}'::JSONB AS classification_counts
  FROM source
  WHERE provider_identity IS NULL
), grouped AS (
  SELECT
    workspace_id,
    platform,
    provider_identity,
    COUNT(*) AS account_count,
    COUNT(DISTINCT profile_id) AS profile_count,
    COUNT(DISTINCT connection_type) AS connection_type_count,
    COUNT(DISTINCT external_user_id) FILTER (WHERE external_user_id IS NOT NULL) AS managed_owner_count,
    COUNT(*) FILTER (WHERE connection_type = 'managed' AND external_user_id IS NULL) AS missing_managed_owner_count,
    COUNT(*) FILTER (WHERE connection_type = 'byo' AND external_user_id IS NOT NULL) AS byo_owner_count,
    COUNT(DISTINCT x_app_mode) AS app_mode_count,
    COUNT(DISTINCT access_token) AS access_token_count,
    COUNT(DISTINCT refresh_token) AS refresh_token_count,
    COUNT(DISTINCT COALESCE(token_expires_at::TEXT, '')) AS token_expiry_count,
    COUNT(DISTINCT authority_scope) AS scope_count,
    COUNT(DISTINCT authority_metadata) AS routing_metadata_count,
    BOOL_OR(connection_type = 'managed') AS has_managed,
    BOOL_OR(connection_type = 'byo') AS has_byo,
    ARRAY_AGG(account_id ORDER BY account_id) AS account_ids,
    ARRAY_AGG(profile_id ORDER BY account_id) AS profile_ids,
    COALESCE(ARRAY_AGG(DISTINCT external_user_id) FILTER (WHERE external_user_id IS NOT NULL), ARRAY[]::TEXT[]) AS external_user_ids,
    ARRAY_AGG(DISTINCT connection_type ORDER BY connection_type) AS connection_types
  FROM source
  WHERE provider_identity IS NOT NULL
  GROUP BY workspace_id, platform, provider_identity
), classified AS (
  SELECT
    *,
    CASE
      WHEN connection_type_count > 1 OR (has_managed AND has_byo) THEN 'mixed_ownership'
      WHEN has_managed AND managed_owner_count > 1 THEN 'cross_managed_user'
      WHEN has_managed AND missing_managed_owner_count > 0 THEN 'missing_managed_owner'
      WHEN has_byo AND byo_owner_count > 0 THEN 'owner_on_byo_connection'
      WHEN account_count <> profile_count THEN 'duplicate_profile_binding'
      WHEN app_mode_count > 1 THEN 'incompatible_app_mode'
      WHEN access_token_count > 1 OR refresh_token_count > 1 OR token_expiry_count > 1 THEN 'incompatible_credential_state'
      WHEN scope_count > 1 THEN 'incompatible_scope'
      WHEN routing_metadata_count > 1 THEN 'incompatible_routing_metadata'
    END AS reason
  FROM grouped
), unsafe AS (
  SELECT
    workspace_id,
    platform,
    provider_identity,
    reason,
    account_ids,
    profile_ids,
    external_user_ids,
    connection_types,
    JSONB_BUILD_OBJECT(
      'account_count', account_count,
      'profile_count', profile_count,
      'managed_owner_count', managed_owner_count,
      'app_mode_count', app_mode_count,
      'access_token_count', access_token_count,
      'refresh_token_count', refresh_token_count,
      'token_expiry_count', token_expiry_count,
      'scope_count', scope_count,
      'routing_metadata_count', routing_metadata_count
    ) AS classification_counts
  FROM classified
  WHERE reason IS NOT NULL
)
SELECT * FROM missing
UNION ALL
SELECT * FROM unsafe
ORDER BY workspace_id, platform, provider_identity NULLS FIRST, reason
`

const preflightAliasWarningsSQL = `
WITH source AS (
  SELECT
    p.workspace_id,
    sa.platform,
    sa.connection_id,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      ELSE NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity,
    ARRAY(
      SELECT DISTINCT identifier
      FROM UNNEST(ARRAY[
        NULLIF(BTRIM(sa.external_account_id), ''),
        NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), ''),
        NULLIF(BTRIM(sa.metadata->>'instagram_business_account_id'), ''),
        NULLIF(BTRIM(sa.metadata->>'business_account_id'), ''),
        NULLIF(BTRIM(sa.metadata->>'page_id'), '')
      ]) identifier
      WHERE identifier IS NOT NULL
      ORDER BY identifier
    ) AS identifiers
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
), pairs AS (
  SELECT
    left_source.workspace_id,
    left_source.platform,
    ARRAY_REMOVE(ARRAY[left_source.connection_id, right_source.connection_id], NULL) AS connection_ids,
    ARRAY[left_source.provider_identity, right_source.provider_identity] AS provider_identities,
    ARRAY(
      SELECT identifier
      FROM UNNEST(left_source.identifiers) identifier
      WHERE identifier = ANY(right_source.identifiers)
      ORDER BY identifier
    ) AS shared_identifiers
  FROM source left_source
  JOIN source right_source
    ON right_source.workspace_id = left_source.workspace_id
   AND right_source.platform = left_source.platform
   AND right_source.provider_identity > left_source.provider_identity
  WHERE left_source.provider_identity IS NOT NULL
    AND right_source.provider_identity IS NOT NULL
    AND left_source.identifiers && right_source.identifiers
)
SELECT DISTINCT
  workspace_id,
  platform,
  connection_ids,
  provider_identities,
  shared_identifiers
FROM pairs
ORDER BY workspace_id, platform, provider_identities
`

const preflightRelationSizesSQL = `
WITH requested(name) AS (
  SELECT * FROM UNNEST(ARRAY[
    'social_accounts',
    'social_connections',
    'post_delivery_jobs',
    'social_post_results',
    'social_post_physical_targets',
    'inbox_items',
    'physical_daily_publish_reservations',
    'physical_daily_publish_operations'
  ]::TEXT[])
)
SELECT
  requested.name,
  PG_TOTAL_RELATION_SIZE(TO_REGCLASS(requested.name))::BIGINT,
  GREATEST(COALESCE(pg_class.reltuples, 0), 0)::BIGINT
FROM requested
JOIN pg_class ON pg_class.oid = TO_REGCLASS(requested.name)
ORDER BY requested.name
`
