package worker

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type PostgresPublishingRestrictionEmailStore struct{ pool *pgxpool.Pool }

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
	rows, err := tx.Query(ctx, `
		WITH candidates AS MATERIALIZED (
			SELECT recipient.id
			FROM platform_publishing_restriction_email_recipients recipient
			JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
			WHERE campaign.status IN ('queued','running')
			  AND (
				recipient.status='pending'
				OR (recipient.status='failed' AND recipient.attempt_count < 3 AND recipient.next_attempt_at <= NOW())
				OR (recipient.status='sending' AND recipient.claimed_at < NOW() - INTERVAL '15 minutes')
			  )
			ORDER BY recipient.created_at
			FOR UPDATE OF recipient SKIP LOCKED
			LIMIT $1
		), claimed AS (
			UPDATE platform_publishing_restriction_email_recipients recipient
			SET status='sending', attempt_count=attempt_count+1, claimed_at=NOW(), updated_at=NOW()
			FROM candidates WHERE recipient.id=candidates.id
			RETURNING recipient.*
		)
		SELECT claimed.id, claimed.campaign_id, campaign.cycle_id, campaign.campaign_type,
		       restriction.platform, claimed.canonical_user_id, claimed.recipient_email,
		       claimed.normalized_email, COALESCE(claimed.first_name_snapshot, ''), claimed.represented_workspace_ids,
		       claimed.idempotency_key, campaign.subject_snapshot, campaign.body_snapshot
		FROM claimed
		JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=claimed.campaign_id
		JOIN platform_publishing_restrictions restriction ON restriction.id=campaign.restriction_id
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	work := make([]PublishingRestrictionEmailWork, 0)
	campaignIDs := map[string]struct{}{}
	for rows.Next() {
		var item PublishingRestrictionEmailWork
		if err := rows.Scan(
			&item.RecipientID, &item.CampaignID, &item.CycleID, &item.CampaignType,
			&item.Platform, &item.CanonicalUserID, &item.RecipientEmail, &item.NormalizedEmail, &item.FirstName,
			&item.RepresentedWorkspaceIDs, &item.IdempotencyKey, &item.SubjectSnapshot, &item.BodySnapshot,
		); err != nil {
			return nil, err
		}
		work = append(work, item)
		campaignIDs[item.CampaignID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for campaignID := range campaignIDs {
		_, err = tx.Exec(ctx, `UPDATE platform_publishing_restriction_email_campaigns SET status='running', started_at=COALESCE(started_at,NOW()), updated_at=NOW() WHERE id=$1 AND status='queued'`, campaignID)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return work, nil
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
		    next_attempt_at=NOW()+CASE WHEN attempt_count < 2 THEN INTERVAL '1 minute' ELSE INTERVAL '5 minutes' END,
		    updated_at=NOW()
		WHERE id=$1 AND status='sending'
	`, recipientID, reason)
	return err
}

func (s *PostgresPublishingRestrictionEmailStore) MarkPublishingRestrictionEmailRecipientSkipped(ctx context.Context, recipientID, reason string) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_publishing_restriction_email_recipients SET status='skipped_ineligible', claimed_at=NULL, last_error=$2, updated_at=NOW() WHERE id=$1 AND status='sending'`, recipientID, reason)
	return err
}

func (s *PostgresPublishingRestrictionEmailStore) RefreshPublishingRestrictionEmailCampaign(ctx context.Context, campaignID string) error {
	_, err := s.pool.Exec(ctx, `
		WITH counts AS (
			SELECT
			  COUNT(*) FILTER (WHERE status IN ('pending','sending') OR (status='failed' AND attempt_count < 3))::int AS pending_count,
			  COUNT(*) FILTER (WHERE status='sent')::int AS sent_count,
			  COUNT(*) FILTER (WHERE status='failed' AND attempt_count >= 3)::int AS failed_count,
			  COUNT(*) FILTER (WHERE status='skipped_ineligible')::int AS skipped_count
			FROM platform_publishing_restriction_email_recipients WHERE campaign_id=$1
		)
		UPDATE platform_publishing_restriction_email_campaigns campaign
		SET pending_count=counts.pending_count, sent_count=counts.sent_count,
		    failed_count=counts.failed_count, skipped_count=counts.skipped_count,
		    status=CASE WHEN counts.pending_count > 0 THEN 'running'
		                WHEN counts.failed_count > 0 THEN 'completed_with_failures'
		                ELSE 'completed' END,
		    completed_at=CASE WHEN counts.pending_count=0 THEN NOW() ELSE NULL END,
		    updated_at=NOW()
		FROM counts WHERE campaign.id=$1
	`, campaignID)
	return err
}
