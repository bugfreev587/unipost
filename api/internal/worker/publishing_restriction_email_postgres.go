package worker

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type PostgresPublishingRestrictionEmailStore struct{ pool *pgxpool.Pool }

const publishingRestrictionEmailMaxAttempts = 3

const publishingRestrictionEmailCampaignRefreshSQL = `
	WITH counts AS (
		SELECT
		  COUNT(*) FILTER (
		    WHERE status IN ('pending','sending')
		       OR (status='failed' AND retryable = TRUE AND attempt_count < $2)
		  )::int AS pending_count,
		  COUNT(*) FILTER (WHERE status='sent')::int AS sent_count,
		  COUNT(*) FILTER (
		    WHERE status='failed' AND (retryable = FALSE OR attempt_count >= $2)
		  )::int AS failed_count,
		  COUNT(*) FILTER (WHERE status='skipped_ineligible')::int AS skipped_count
		FROM platform_publishing_restriction_email_recipients WHERE campaign_id=$1
	)
	UPDATE platform_publishing_restriction_email_campaigns campaign
	SET pending_count=counts.pending_count, sent_count=counts.sent_count,
	    failed_count=counts.failed_count, skipped_count=counts.skipped_count,
	    status=CASE WHEN counts.pending_count > 0 THEN 'running'
	                WHEN counts.failed_count > 0 THEN 'completed_with_failures'
	                ELSE 'completed' END,
	    started_at=CASE WHEN counts.pending_count > 0 THEN COALESCE(started_at,NOW()) ELSE started_at END,
	    completed_at=CASE WHEN counts.pending_count=0 THEN NOW() ELSE NULL END,
	    updated_at=NOW()
	FROM counts WHERE campaign.id=$1`

func NewPostgresPublishingRestrictionEmailStore(pool *pgxpool.Pool) *PostgresPublishingRestrictionEmailStore {
	return &PostgresPublishingRestrictionEmailStore{pool: pool}
}

func (s *PostgresPublishingRestrictionEmailStore) ClaimPublishingRestrictionEmailRecipients(ctx context.Context, limit int) ([]PublishingRestrictionEmailWork, error) {
	if limit <= 0 {
		limit = 50
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	campaignIDs := map[string]struct{}{}
	staleRows, err := tx.Query(ctx, `
		WITH stale AS MATERIALIZED (
			SELECT recipient.id, recipient.email_send_attempt_id
			FROM platform_publishing_restriction_email_recipients recipient
			WHERE recipient.status = 'sending'
			  AND recipient.claimed_at < NOW() - INTERVAL '15 minutes'
			FOR UPDATE OF recipient SKIP LOCKED
		), failed_audits AS (
			UPDATE email_send_attempts attempt
			SET status = 'failed',
			    last_error = 'send outcome unknown after worker lease expired; not automatically retried',
			    updated_at = NOW()
			FROM stale
			WHERE attempt.id = stale.email_send_attempt_id
			  AND attempt.status = 'pending'
			RETURNING attempt.id
		)
		UPDATE platform_publishing_restriction_email_recipients recipient
		SET status = 'failed',
		    retryable = FALSE,
		    claimed_at = NULL,
		    last_error = 'send outcome unknown after worker lease expired; manual review required',
		    updated_at = NOW()
		FROM stale
		WHERE recipient.id = stale.id
		RETURNING recipient.campaign_id
	`)
	if err != nil {
		return nil, err
	}
	for staleRows.Next() {
		var campaignID string
		if err := staleRows.Scan(&campaignID); err != nil {
			staleRows.Close()
			return nil, err
		}
		campaignIDs[campaignID] = struct{}{}
	}
	if err := staleRows.Err(); err != nil {
		staleRows.Close()
		return nil, err
	}
	staleRows.Close()
	rows, err := tx.Query(ctx, `
		WITH candidates AS MATERIALIZED (
			SELECT recipient.id
			FROM platform_publishing_restriction_email_recipients recipient
			JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
			WHERE campaign.status IN ('queued','running')
			  AND (
				recipient.status='pending'
				OR (
					recipient.status='failed'
					AND recipient.retryable = TRUE
					AND recipient.attempt_count < $2
					AND recipient.next_attempt_at <= NOW()
				)
			  )
			ORDER BY recipient.created_at
			FOR UPDATE OF recipient SKIP LOCKED
			LIMIT $1
		), claimed AS (
			UPDATE platform_publishing_restriction_email_recipients recipient
			SET status='sending', attempt_count=attempt_count+1, retryable = FALSE,
			    email_send_attempt_id=NULL, claimed_at=NOW(), updated_at=NOW()
			FROM candidates WHERE recipient.id=candidates.id
			RETURNING recipient.*
		)
		SELECT claimed.id, claimed.campaign_id, campaign.cycle_id, campaign.campaign_type,
		       restriction.platform, claimed.canonical_user_id, claimed.recipient_email,
		       claimed.normalized_email, COALESCE(claimed.first_name_snapshot, ''), claimed.represented_workspace_ids,
		       claimed.idempotency_key, campaign.subject_snapshot, campaign.body_snapshot,
		       claimed.attempt_count, claimed.attempt_generation
		FROM claimed
		JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=claimed.campaign_id
		JOIN platform_publishing_restrictions restriction ON restriction.id=campaign.restriction_id
	`, limit, publishingRestrictionEmailMaxAttempts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	work := make([]PublishingRestrictionEmailWork, 0)
	for rows.Next() {
		var item PublishingRestrictionEmailWork
		if err := rows.Scan(
			&item.RecipientID, &item.CampaignID, &item.CycleID, &item.CampaignType,
			&item.Platform, &item.CanonicalUserID, &item.RecipientEmail, &item.NormalizedEmail, &item.FirstName,
			&item.RepresentedWorkspaceIDs, &item.IdempotencyKey, &item.SubjectSnapshot, &item.BodySnapshot,
			&item.AttemptCount, &item.AttemptGeneration,
		); err != nil {
			return nil, err
		}
		work = append(work, item)
		campaignIDs[item.CampaignID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for campaignID := range campaignIDs {
		_, err = tx.Exec(ctx, publishingRestrictionEmailCampaignRefreshSQL, campaignID, publishingRestrictionEmailMaxAttempts)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return work, nil
}

func (s *PostgresPublishingRestrictionEmailStore) LinkPublishingRestrictionEmailAttempt(
	ctx context.Context,
	recipientID string,
	attemptID string,
	attemptCount int,
	attemptGeneration int,
) error {
	var linkedID string
	err := s.pool.QueryRow(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET email_send_attempt_id = $2, updated_at = NOW()
		WHERE id = $1
		  AND status = 'sending'
		  AND attempt_count = $3
		  AND attempt_generation = $4
		  AND email_send_attempt_id IS NULL
		RETURNING id
	`, recipientID, attemptID, attemptCount, attemptGeneration).Scan(&linkedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("publishing restriction email send claim is no longer active")
	}
	return err
}

const publishingRestrictionRecipientEligibilitySQL = `
	WITH current_plans AS (
		SELECT DISTINCT ON (workspace_id) workspace_id, plan_id
		FROM subscriptions ORDER BY workspace_id, updated_at DESC
	), represented AS (
		SELECT UNNEST($5::text[]) AS workspace_id
	)
	SELECT EXISTS (
		SELECT 1
		FROM platform_publishing_restrictions restriction
		WHERE restriction.platform=$1 AND restriction.cycle_id=$2 AND restriction.enabled=$3
	) AND EXISTS (
		SELECT 1
		FROM represented
		LEFT JOIN current_plans plan ON plan.workspace_id=represented.workspace_id
		WHERE COALESCE(plan.plan_id,'free')='free'
		  AND EXISTS (
			SELECT 1
			FROM users owner_user
			JOIN workspace_members owner_member
			  ON owner_member.user_id=owner_user.id
			 AND owner_member.workspace_id=represented.workspace_id
			 AND owner_member.role='owner' AND owner_member.status='active'
			WHERE LOWER(TRIM(owner_user.email))=$4
		  )
		  AND EXISTS (
			SELECT 1 FROM social_accounts account
			WHERE account.workspace_id=represented.workspace_id AND account.platform=$1
			  AND account.status='active' AND account.disconnected_at IS NULL
		  )
	)`

func (s *PostgresPublishingRestrictionEmailStore) PublishingRestrictionEmailRecipientEligible(ctx context.Context, work PublishingRestrictionEmailWork) (bool, error) {
	wantEnabled := work.CampaignType == publishingrestrictions.RestrictionNotice
	var eligible bool
	err := s.pool.QueryRow(ctx, publishingRestrictionRecipientEligibilitySQL,
		work.Platform, work.CycleID, wantEnabled, work.NormalizedEmail, work.RepresentedWorkspaceIDs).Scan(&eligible)
	return eligible, err
}

func (s *PostgresPublishingRestrictionEmailStore) MarkPublishingRestrictionEmailRecipientSent(ctx context.Context, recipientID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_publishing_restriction_email_recipients SET status='sent', sent_at=NOW(), claimed_at=NULL, last_error=NULL, updated_at=NOW() WHERE id=$1 AND status='sending'`, recipientID)
	return err
}

func (s *PostgresPublishingRestrictionEmailStore) MarkPublishingRestrictionEmailRecipientFailed(ctx context.Context, recipientID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET status='failed', claimed_at=NULL, last_error=$2,
		    retryable = attempt_count < $3,
		    next_attempt_at=NOW()+CASE WHEN attempt_count < 2 THEN INTERVAL '1 minute' ELSE INTERVAL '5 minutes' END,
		    updated_at=NOW()
		WHERE id=$1 AND status='sending'
	`, recipientID, reason, publishingRestrictionEmailMaxAttempts)
	return err
}

func (s *PostgresPublishingRestrictionEmailStore) MarkPublishingRestrictionEmailRecipientTerminalFailed(ctx context.Context, recipientID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET status='failed', retryable=FALSE, claimed_at=NULL, last_error=$2, updated_at=NOW()
		WHERE id=$1 AND status='sending'
	`, recipientID, reason)
	return err
}

func (s *PostgresPublishingRestrictionEmailStore) MarkPublishingRestrictionEmailRecipientSkipped(ctx context.Context, recipientID, reason string) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_publishing_restriction_email_recipients SET status='skipped_ineligible', claimed_at=NULL, last_error=$2, updated_at=NOW() WHERE id=$1 AND status='sending'`, recipientID, reason)
	return err
}

func (s *PostgresPublishingRestrictionEmailStore) RefreshPublishingRestrictionEmailCampaign(ctx context.Context, campaignID string) error {
	_, err := s.pool.Exec(ctx, publishingRestrictionEmailCampaignRefreshSQL, campaignID, publishingRestrictionEmailMaxAttempts)
	return err
}
