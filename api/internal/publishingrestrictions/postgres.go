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
		), retained_objects AS (
			SELECT DISTINCT failure.restriction_cycle_id AS cycle_id, usage.media_id, media.size_bytes
			FROM post_failures failure
			JOIN media_post_usages usage
			  ON usage.post_id=failure.post_id
			 AND usage.retention_reason='publishing_restriction'
			 AND usage.cleanup_after_at > NOW()
			JOIN media ON media.id=usage.media_id
			WHERE failure.restriction_cycle_id IS NOT NULL
		), retention_metrics AS (
			SELECT cycle_id,
			       COUNT(*)::BIGINT AS current_cycle_retained_object_count,
			       COALESCE(SUM(size_bytes),0)::BIGINT AS current_cycle_retained_bytes
			FROM retained_objects
			GROUP BY cycle_id
		)
		SELECT `+aliasedRestrictionColumns+`,
		       COUNT(DISTINCT account_profile.workspace_id) FILTER (
		         WHERE COALESCE(cp.plan_id, 'free') = ANY(r.restricted_plan_ids)
		       )::INTEGER AS affected_workspaces,
		       COUNT(sa.id) FILTER (
		         WHERE COALESCE(cp.plan_id, 'free') = ANY(r.restricted_plan_ids)
		       )::INTEGER AS affected_accounts,
		       COALESCE(MAX(retention_metrics.current_cycle_retained_object_count),0)::BIGINT AS current_cycle_retained_object_count,
		       COALESCE(MAX(retention_metrics.current_cycle_retained_bytes),0)::BIGINT AS current_cycle_retained_bytes,
		       ROUND((COALESCE(MAX(retention_metrics.current_cycle_retained_bytes),0)::NUMERIC / 1000000000) * 0.015 * 2, 6)::DOUBLE PRECISION AS current_cycle_projected_60_day_storage_cost_usd
		FROM platform_publishing_restrictions r
		LEFT JOIN social_accounts sa
		  ON sa.platform = r.platform
		 AND sa.status = 'active'
		 AND sa.disconnected_at IS NULL
		LEFT JOIN profiles account_profile ON account_profile.id = sa.profile_id
		LEFT JOIN current_plans cp ON cp.workspace_id = account_profile.workspace_id
		LEFT JOIN retention_metrics ON retention_metrics.cycle_id = r.cycle_id
		GROUP BY r.id
		ORDER BY r.platform
	`)
	if err != nil {
		return nil, err
	}
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
			&restriction.CurrentCycleRetainedObjectCount, &restriction.CurrentCycleRetainedBytes,
			&restriction.CurrentCycleProjected60DayStorageCostUSD,
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
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range result {
		eventRows, err := s.pool.Query(ctx, `
			SELECT id, event_type, actor_user_id, expected_version, resulting_version, created_at
			FROM platform_publishing_restriction_events
			WHERE restriction_id=$1 ORDER BY created_at DESC LIMIT 10
		`, result[i].ID)
		if err != nil {
			return nil, err
		}
		for eventRows.Next() {
			var event RestrictionEvent
			var actor *string
			if err := eventRows.Scan(&event.ID, &event.EventType, &actor, &event.ExpectedVersion, &event.ResultingVersion, &event.CreatedAt); err != nil {
				eventRows.Close()
				return nil, err
			}
			if actor != nil {
				event.ActorUserID = *actor
			}
			result[i].RecentEvents = append(result[i].RecentEvents, event)
		}
		if err := eventRows.Err(); err != nil {
			eventRows.Close()
			return nil, err
		}
		eventRows.Close()
	}
	return result, nil
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

type campaignQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func scanCampaign(row pgx.Row) (Campaign, error) {
	var campaign Campaign
	var createdBy, confirmedBy *string
	err := row.Scan(
		&campaign.ID, &campaign.RestrictionID, &campaign.CycleID, &campaign.CampaignType,
		&campaign.Status, &campaign.SubjectSnapshot, &campaign.BodySnapshot,
		&campaign.RestrictionVersion, &campaign.PreviewedCount, &campaign.SnapshottedCount,
		&campaign.PendingCount, &campaign.SentCount, &campaign.FailedCount, &campaign.SkippedCount,
		&createdBy, &confirmedBy, &campaign.SnapshottedAt, &campaign.StartedAt,
		&campaign.CompletedAt, &campaign.CreatedAt, &campaign.UpdatedAt,
	)
	if createdBy != nil {
		campaign.CreatedByUserID = *createdBy
	}
	if confirmedBy != nil {
		campaign.ConfirmedByUserID = *confirmedBy
	}
	return campaign, err
}

const campaignColumns = `id, restriction_id, cycle_id, campaign_type, status, subject_snapshot, body_snapshot,
restriction_version, previewed_count, snapshotted_count, pending_count, sent_count, failed_count, skipped_count,
created_by_user_id, confirmed_by_user_id, snapshotted_at, started_at, completed_at, created_at, updated_at`
const aliasedCampaignColumns = `c.id, c.restriction_id, c.cycle_id, c.campaign_type, c.status, c.subject_snapshot, c.body_snapshot,
c.restriction_version, c.previewed_count, c.snapshotted_count, c.pending_count, c.sent_count, c.failed_count, c.skipped_count,
c.created_by_user_id, c.confirmed_by_user_id, c.snapshotted_at, c.started_at, c.completed_at, c.created_at, c.updated_at`

func (s *PostgresStore) PreviewCampaignRecipients(ctx context.Context, restriction Restriction, campaignType CampaignType) ([]RecipientSnapshot, error) {
	return previewCampaignRecipients(ctx, s.pool, restriction, campaignType)
}

func campaignAudienceQuery(restriction Restriction, campaignType CampaignType) (string, []any, error) {
	if campaignType == RestrictionNotice {
		return `
			WITH current_plans AS (
				SELECT DISTINCT ON (workspace_id) workspace_id, plan_id
				FROM subscriptions ORDER BY workspace_id, updated_at DESC
			), eligible_owners AS (
				SELECT u.id AS user_id, u.email, LOWER(TRIM(u.email)) AS normalized_email,
				       COALESCE(NULLIF(SPLIT_PART(TRIM(COALESCE(u.name, '')), ' ', 1), ''), 'there') AS first_name,
				       wm.workspace_id
				FROM users u
				JOIN workspace_members wm ON wm.user_id = u.id AND wm.role = 'owner' AND wm.status = 'active'
				LEFT JOIN current_plans cp ON cp.workspace_id = wm.workspace_id
				WHERE COALESCE(cp.plan_id, 'free') = 'free'
				  AND TRIM(u.email) <> ''
				  AND EXISTS (
					SELECT 1 FROM social_accounts sa
					JOIN profiles account_profile ON account_profile.id = sa.profile_id
					WHERE account_profile.workspace_id = wm.workspace_id AND sa.platform = $1
					  AND sa.status = 'active' AND sa.disconnected_at IS NULL
				  )
			)
			SELECT (ARRAY_AGG(user_id ORDER BY user_id))[1] AS user_id,
			       (ARRAY_AGG(email ORDER BY user_id))[1] AS email,
			       normalized_email,
			       (ARRAY_AGG(first_name ORDER BY user_id))[1] AS first_name,
			       ARRAY_AGG(workspace_id ORDER BY user_id, workspace_id) AS workspace_ids,
			       ARRAY_AGG(user_id ORDER BY user_id, workspace_id) AS owner_user_ids
			FROM eligible_owners
			GROUP BY normalized_email
			ORDER BY normalized_email`, []any{restriction.Platform}, nil
	} else if campaignType == RecoveryNotice {
		return `
			WITH current_plans AS (
				SELECT DISTINCT ON (workspace_id) workspace_id, plan_id
				FROM subscriptions ORDER BY workspace_id, updated_at DESC
			), prior_sent AS (
				SELECT prior.canonical_user_id, workspace.workspace_id, owner.owner_user_id
				FROM platform_publishing_restriction_email_recipients prior
				JOIN platform_publishing_restriction_email_campaigns sent_campaign
				  ON sent_campaign.id = prior.campaign_id AND sent_campaign.cycle_id = $2
				 AND sent_campaign.campaign_type = 'restriction_notice'
				CROSS JOIN LATERAL UNNEST(prior.represented_workspace_ids) WITH ORDINALITY
				  AS workspace(workspace_id, ordinality)
				JOIN LATERAL UNNEST(prior.represented_owner_user_ids) WITH ORDINALITY
				  AS owner(owner_user_id, ordinality)
				  ON owner.ordinality = workspace.ordinality
				WHERE prior.status = 'sent'
			), eligible AS (
				SELECT prior.canonical_user_id AS user_id,
				       canonical_user.email,
				       LOWER(TRIM(canonical_user.email)) AS normalized_email,
				       COALESCE(NULLIF(SPLIT_PART(TRIM(COALESCE(canonical_user.name, '')), ' ', 1), ''), 'there') AS first_name,
				       prior.workspace_id,
				       prior.owner_user_id
				FROM prior_sent prior
				JOIN users canonical_user ON canonical_user.id = prior.canonical_user_id
				JOIN users owner_user ON owner_user.id = prior.owner_user_id
				JOIN workspace_members owner_member
				  ON owner_member.workspace_id = prior.workspace_id
				 AND owner_member.user_id = prior.owner_user_id
				 AND owner_member.role = 'owner' AND owner_member.status = 'active'
				LEFT JOIN current_plans cp ON cp.workspace_id = prior.workspace_id
				WHERE COALESCE(cp.plan_id, 'free') = 'free'
				  AND TRIM(canonical_user.email) <> ''
				  AND LOWER(TRIM(owner_user.email)) = LOWER(TRIM(canonical_user.email))
				  AND EXISTS (
					SELECT 1 FROM social_accounts sa
					JOIN profiles account_profile ON account_profile.id = sa.profile_id
					WHERE account_profile.workspace_id = prior.workspace_id AND sa.platform = $1
					  AND sa.status = 'active' AND sa.disconnected_at IS NULL
				  )
			)
			SELECT (ARRAY_AGG(user_id ORDER BY user_id))[1] AS user_id,
			       (ARRAY_AGG(email ORDER BY user_id))[1] AS email,
			       normalized_email,
			       (ARRAY_AGG(first_name ORDER BY user_id))[1] AS first_name,
			       ARRAY_AGG(workspace_id ORDER BY user_id, workspace_id) AS workspace_ids,
			       ARRAY_AGG(owner_user_id ORDER BY user_id, workspace_id) AS owner_user_ids
			FROM eligible
			GROUP BY normalized_email
			ORDER BY normalized_email`, []any{restriction.Platform, restriction.CycleID}, nil
	}
	return "", nil, ErrCampaignPrecondition
}

func previewCampaignRecipients(ctx context.Context, q campaignQuerier, restriction Restriction, campaignType CampaignType) ([]RecipientSnapshot, error) {
	query, args, err := campaignAudienceQuery(restriction, campaignType)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]RecipientSnapshot, 0)
	for rows.Next() {
		var recipient RecipientSnapshot
		if err := rows.Scan(
			&recipient.CanonicalUserID, &recipient.RecipientEmail, &recipient.NormalizedEmail,
			&recipient.FirstName, &recipient.RepresentedWorkspaceIDs, &recipient.RepresentedOwnerUserIDs,
		); err != nil {
			return nil, err
		}
		result = append(result, recipient)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if campaignType == RecoveryNotice && len(result) == 0 {
		return nil, ErrCampaignPrecondition
	}
	return result, nil
}

func (s *PostgresStore) CreateCampaign(ctx context.Context, expected Restriction, campaignType CampaignType, copy CampaignCopy, previewedCount int, actorUserID string) (Campaign, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Campaign{}, err
	}
	defer tx.Rollback(ctx)
	current, err := scanRestriction(tx.QueryRow(ctx, `SELECT `+restrictionColumns+` FROM platform_publishing_restrictions WHERE platform = $1 FOR UPDATE`, expected.Platform))
	if err != nil {
		return Campaign{}, err
	}
	if current.CycleID != expected.CycleID || current.Version != expected.Version ||
		(campaignType == RestrictionNotice && !current.Enabled) || (campaignType == RecoveryNotice && current.Enabled) {
		return Campaign{}, ErrCampaignPrecondition
	}
	recipients, err := previewCampaignRecipients(ctx, tx, current, campaignType)
	if err != nil {
		return Campaign{}, err
	}
	campaign, err := scanCampaign(tx.QueryRow(ctx, `
		INSERT INTO platform_publishing_restriction_email_campaigns (
			restriction_id, cycle_id, campaign_type, status, subject_snapshot, body_snapshot,
			restriction_version, previewed_count, snapshotted_count, pending_count,
			created_by_user_id, confirmed_by_user_id, completed_at
		) VALUES ($1,$2,$3,CASE WHEN $8::int=0 THEN 'completed' ELSE 'queued' END,$4,$5,$6,$7,$8,$8,NULLIF($9,''),NULLIF($9,''),CASE WHEN $8::int=0 THEN NOW() ELSE NULL END)
		ON CONFLICT (cycle_id, campaign_type) DO NOTHING
		RETURNING `+campaignColumns,
		current.ID, current.CycleID, campaignType, copy.Subject, copy.Body,
		current.Version, previewedCount, len(recipients), actorUserID))
	if errors.Is(err, pgx.ErrNoRows) {
		campaign, err = scanCampaign(tx.QueryRow(ctx, `SELECT `+campaignColumns+` FROM platform_publishing_restriction_email_campaigns WHERE cycle_id=$1 AND campaign_type=$2`, current.CycleID, campaignType))
		if err != nil {
			return Campaign{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Campaign{}, err
		}
		return campaign, nil
	}
	if err != nil {
		return Campaign{}, err
	}
	for _, recipient := range recipients {
		_, err := tx.Exec(ctx, `
			INSERT INTO platform_publishing_restriction_email_recipients (
				campaign_id, canonical_user_id, recipient_email, normalized_email,
				first_name_snapshot, represented_workspace_ids, represented_owner_user_ids,
				idempotency_key
			) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		`, campaign.ID, recipient.CanonicalUserID, recipient.RecipientEmail, recipient.NormalizedEmail,
			recipient.FirstName, recipient.RepresentedWorkspaceIDs, recipient.RepresentedOwnerUserIDs,
			current.CycleID+":"+string(campaignType)+":"+recipient.CanonicalUserID)
		if err != nil {
			return Campaign{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Campaign{}, err
	}
	return campaign, nil
}

func (s *PostgresStore) ListCampaigns(ctx context.Context, platform string) ([]Campaign, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+aliasedCampaignColumns+`
		FROM platform_publishing_restriction_email_campaigns c
		JOIN platform_publishing_restrictions r ON r.id = c.restriction_id
		WHERE r.platform = $1 ORDER BY c.created_at DESC
	`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Campaign, 0)
	for rows.Next() {
		campaign, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, campaign)
	}
	return result, rows.Err()
}

func (s *PostgresStore) RetryFailedCampaign(ctx context.Context, platform, campaignID string) (Campaign, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Campaign{}, err
	}
	defer tx.Rollback(ctx)
	var retryWindowOpen bool
	err = tx.QueryRow(ctx, `
		SELECT campaign.created_at >= NOW() - INTERVAL '24 hours'
		FROM platform_publishing_restriction_email_campaigns campaign
		JOIN platform_publishing_restrictions restriction ON restriction.id=campaign.restriction_id
		WHERE campaign.id=$1 AND restriction.platform=$2
		FOR UPDATE OF campaign
	`, campaignID, platform).Scan(&retryWindowOpen)
	if errors.Is(err, pgx.ErrNoRows) {
		return Campaign{}, ErrCampaignPrecondition
	}
	if err != nil {
		return Campaign{}, err
	}
	if !retryWindowOpen {
		return Campaign{}, ErrCampaignRetryExpired
	}
	command, err := tx.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients recipient
		SET status='pending',
		    attempt_count=0,
		    attempt_generation=attempt_generation+1,
		    retryable=TRUE,
		    next_attempt_at=NOW(),
		    claimed_at=NULL,
		    last_error=NULL,
		    email_send_attempt_id=NULL,
		    updated_at=NOW()
		FROM platform_publishing_restriction_email_campaigns campaign
		JOIN platform_publishing_restrictions restriction ON restriction.id=campaign.restriction_id
		WHERE recipient.campaign_id=campaign.id AND campaign.id=$1 AND restriction.platform=$2
		  AND recipient.status='failed'
		  AND (
		    recipient.retryable=TRUE
		    OR (
		      recipient.retryable=FALSE
		      AND LOWER(COALESCE(recipient.last_error, '')) NOT LIKE '%manual review%'
		      AND LOWER(COALESCE(recipient.last_error, '')) NOT LIKE '%outcome unknown%'
		      AND (
		        recipient.email_send_attempt_id IS NULL
		        OR EXISTS (
		          SELECT 1
		          FROM email_send_attempts attempt
		          WHERE attempt.id=recipient.email_send_attempt_id
		            AND attempt.status='failed'
		            AND CASE
		              WHEN pg_input_is_valid(COALESCE(attempt.last_error, ''), 'jsonb')
		              THEN COALESCE((attempt.last_error::jsonb)->>'outcome_class', 'provider_unknown')
		              ELSE 'provider_unknown'
		            END IN ('provider_definitive_failure', 'pre_send_failure')
		        )
		      )
		    )
		  )
	`, campaignID, platform)
	if err != nil {
		return Campaign{}, err
	}
	if command.RowsAffected() == 0 {
		return Campaign{}, ErrCampaignPrecondition
	}
	_, err = tx.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_campaigns
		SET status='queued', pending_count=(SELECT COUNT(*) FROM platform_publishing_restriction_email_recipients WHERE campaign_id=$1 AND status='pending'),
		    failed_count=(SELECT COUNT(*) FROM platform_publishing_restriction_email_recipients WHERE campaign_id=$1 AND status='failed'),
		    completed_at=NULL, updated_at=NOW()
		WHERE id=$1
	`, campaignID)
	if err != nil {
		return Campaign{}, err
	}
	campaign, err := scanCampaign(tx.QueryRow(ctx, `SELECT `+campaignColumns+` FROM platform_publishing_restriction_email_campaigns WHERE id=$1`, campaignID))
	if err != nil {
		return Campaign{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Campaign{}, err
	}
	return campaign, nil
}
