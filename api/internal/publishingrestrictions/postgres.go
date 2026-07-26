package publishingrestrictions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func scanRestriction(row pgx.Row) (Restriction, error) {
	var r Restriction
	var cycleID, actorID *string
	err := row.Scan(&r.ID, &r.Platform, &r.Enabled, &r.RestrictedPlanIDs, &r.ReasonCode, &r.UserMessage, &cycleID, &r.Version, &r.EnabledAt, &r.DisabledAt, &actorID, &r.CreatedAt, &r.UpdatedAt)
	if cycleID != nil {
		r.CycleID = *cycleID
	}
	if actorID != nil {
		r.UpdatedByUserID = *actorID
	}
	return r, err
}

const restrictionColumns = `id, platform, enabled, restricted_plan_ids, reason_code, user_message, cycle_id, version, enabled_at, disabled_at, updated_by_user_id, created_at, updated_at`
const aliasedRestrictionColumns = `r.id, r.platform, r.enabled, r.restricted_plan_ids, r.reason_code, r.user_message, r.cycle_id, r.version, r.enabled_at, r.disabled_at, r.updated_by_user_id, r.created_at, r.updated_at`

func (s *PostgresStore) RestrictionForPlatform(ctx context.Context, platform string) (Restriction, error) {
	r, err := scanRestriction(s.pool.QueryRow(ctx, `SELECT `+restrictionColumns+` FROM platform_publishing_restrictions WHERE platform = $1`, strings.ToLower(strings.TrimSpace(platform))))
	if errors.Is(err, pgx.ErrNoRows) {
		return Restriction{Platform: strings.ToLower(strings.TrimSpace(platform)), RestrictedPlanIDs: []string{}}, nil
	}
	return r, err
}

func (s *PostgresStore) ListRestrictions(ctx context.Context) ([]Restriction, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+restrictionColumns+` FROM platform_publishing_restrictions ORDER BY platform`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Restriction, 0)
	for rows.Next() {
		restriction, err := scanRestriction(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, restriction)
	}
	return result, rows.Err()
}

func (s *PostgresStore) ListAdminRestrictions(ctx context.Context) ([]Restriction, error) {
	rows, err := s.pool.Query(ctx, `
		WITH current_plans AS (
			SELECT DISTINCT ON (workspace_id) workspace_id, plan_id
			FROM subscriptions
			ORDER BY workspace_id, updated_at DESC
		)
		SELECT `+aliasedRestrictionColumns+`,
		       COUNT(DISTINCT sa.workspace_id) FILTER (
		         WHERE COALESCE(cp.plan_id, 'free') = ANY(r.restricted_plan_ids)
		       )::INTEGER AS affected_workspaces,
		       COUNT(sa.id) FILTER (
		         WHERE COALESCE(cp.plan_id, 'free') = ANY(r.restricted_plan_ids)
		       )::INTEGER AS affected_accounts
		FROM platform_publishing_restrictions r
		LEFT JOIN social_accounts sa
		  ON sa.platform = r.platform
		 AND sa.status = 'active'
		 AND sa.disconnected_at IS NULL
		LEFT JOIN current_plans cp ON cp.workspace_id = sa.workspace_id
		GROUP BY r.id
		ORDER BY r.platform
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Restriction, 0)
	for rows.Next() {
		var restriction Restriction
		var cycleID, actorID *string
		err := rows.Scan(
			&restriction.ID, &restriction.Platform, &restriction.Enabled,
			&restriction.RestrictedPlanIDs, &restriction.ReasonCode, &restriction.UserMessage,
			&cycleID, &restriction.Version, &restriction.EnabledAt, &restriction.DisabledAt,
			&actorID, &restriction.CreatedAt, &restriction.UpdatedAt,
			&restriction.AffectedWorkspaces, &restriction.AffectedAccounts,
		)
		if err != nil {
			return nil, err
		}
		if cycleID != nil {
			restriction.CycleID = *cycleID
		}
		if actorID != nil {
			restriction.UpdatedByUserID = *actorID
		}
		result = append(result, restriction)
	}
	return result, rows.Err()
}

func (s *PostgresStore) WorkspacePlanID(ctx context.Context, workspaceID string) (string, error) {
	var planID string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT plan_id FROM subscriptions
			WHERE workspace_id = $1
			ORDER BY updated_at DESC LIMIT 1
		), 'free')
	`, workspaceID).Scan(&planID)
	return planID, err
}

func (s *PostgresStore) SetEnabled(ctx context.Context, request TransitionRequest) (TransitionResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TransitionResult{}, err
	}
	defer tx.Rollback(ctx)

	current, err := scanRestriction(tx.QueryRow(ctx, `SELECT `+restrictionColumns+` FROM platform_publishing_restrictions WHERE platform = $1 FOR UPDATE`, request.Platform))
	if err != nil {
		return TransitionResult{}, err
	}
	if current.Version != request.ExpectedVersion {
		return TransitionResult{}, &VersionConflictError{Current: current}
	}
	if current.Enabled == request.Enabled {
		if err := tx.Commit(ctx); err != nil {
			return TransitionResult{}, err
		}
		return TransitionResult{Restriction: current, Changed: false}, nil
	}

	beforeJSON, err := json.Marshal(current)
	if err != nil {
		return TransitionResult{}, err
	}
	cycleID := current.CycleID
	if request.Enabled {
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::TEXT`).Scan(&cycleID); err != nil {
			return TransitionResult{}, err
		}
	}
	var updated Restriction
	now := time.Now().UTC()
	if request.Enabled {
		updated, err = scanRestriction(tx.QueryRow(ctx, `
			UPDATE platform_publishing_restrictions
			SET enabled = TRUE, cycle_id = $2, version = version + 1,
			    enabled_at = $3, disabled_at = NULL, updated_by_user_id = NULLIF($4, ''), updated_at = $3
			WHERE platform = $1 RETURNING `+restrictionColumns,
			request.Platform, cycleID, now, request.ActorUserID))
	} else {
		updated, err = scanRestriction(tx.QueryRow(ctx, `
			UPDATE platform_publishing_restrictions
			SET enabled = FALSE, version = version + 1,
			    disabled_at = $2, updated_by_user_id = NULLIF($3, ''), updated_at = $2
			WHERE platform = $1 RETURNING `+restrictionColumns,
			request.Platform, now, request.ActorUserID))
	}
	if err != nil {
		return TransitionResult{}, err
	}
	afterJSON, err := json.Marshal(updated)
	if err != nil {
		return TransitionResult{}, err
	}
	eventType := "disabled"
	if request.Enabled {
		eventType = "enabled"
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO platform_publishing_restriction_events (
			restriction_id, platform, cycle_id, event_type, actor_user_id,
			expected_version, resulting_version, before_state, after_state,
			request_id, actor_ip, actor_user_agent
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''),NULLIF($11,''),NULLIF($12,''))
	`, updated.ID, updated.Platform, updated.CycleID, eventType, request.ActorUserID,
		request.ExpectedVersion, updated.Version, beforeJSON, afterJSON,
		request.RequestID, request.ActorIP, request.ActorUserAgent)
	if err != nil {
		return TransitionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TransitionResult{}, err
	}
	return TransitionResult{Restriction: updated, Changed: true}, nil
}
