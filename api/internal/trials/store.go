package trials

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type PostgresStore struct {
	queries *db.Queries
}

func NewPostgresStore(queries *db.Queries) *PostgresStore {
	return &PostgresStore{queries: queries}
}

func (s *PostgresStore) GetBilling(ctx context.Context, workspaceID string) (BillingSnapshot, error) {
	workspace, err := s.queries.GetWorkspace(ctx, workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return BillingSnapshot{}, ErrWorkspaceNotFound
	}
	if err != nil {
		return BillingSnapshot{}, err
	}
	result := BillingSnapshot{WorkspaceID: workspace.ID, OwnerUserID: workspace.UserID}
	sub, err := s.queries.GetSubscriptionByWorkspace(ctx, workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		result.Subscription = SubscriptionRecord{PlanID: "free", Status: "active"}
		return result, nil
	}
	if err != nil {
		return BillingSnapshot{}, err
	}
	result.Subscription = SubscriptionRecord{
		PlanID: sub.PlanID, Status: sub.Status,
		StripeCustomerID:     sub.StripeCustomerID.String,
		StripeSubscriptionID: sub.StripeSubscriptionID.String,
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd.Bool,
	}
	return result, nil
}

func (s *PostgresStore) GetOpenGrant(ctx context.Context, workspaceID string) (Grant, error) {
	row, err := s.queries.GetOpenWorkspaceTrialGrant(ctx, workspaceID)
	return mapGrant(row, mapLookupError(err))
}

func (s *PostgresStore) GetGrant(ctx context.Context, id string) (Grant, error) {
	row, err := s.queries.GetWorkspaceTrialGrant(ctx, id)
	return mapGrant(row, mapLookupError(err))
}

func (s *PostgresStore) CreateGrant(ctx context.Context, input CreateGrantInput) (Grant, error) {
	row, err := s.queries.CreateWorkspaceTrialGrant(ctx, db.CreateWorkspaceTrialGrantParams{
		WorkspaceID: input.WorkspaceID, Kind: string(input.Kind), PlanID: input.PlanID,
		DurationDays: input.DurationDays, Status: string(input.Status), GrantedByUserID: input.ActorUserID,
		StripeMode: text(input.StripeMode), StripeCustomerID: text(input.StripeCustomerID),
		StripeSubscriptionID: text(input.StripeSubscriptionID),
		GrantedAt:            pgtype.Timestamptz{Time: input.GrantedAt, Valid: true},
	})
	return mapGrant(row, mapCreateError(err))
}

func (s *PostgresStore) MarkScheduled(ctx context.Context, update ScheduledUpdate) (Grant, error) {
	row, err := s.queries.MarkWorkspaceTrialGrantScheduled(ctx, db.MarkWorkspaceTrialGrantScheduledParams{
		StripeCustomerID: text(update.StripeCustomerID), StripeSubscriptionID: text(update.StripeSubscriptionID),
		StripeScheduleID: text(update.StripeScheduleID),
		ScheduledStartAt: pgtype.Timestamptz{Time: update.ScheduledStartAt, Valid: true},
		EndsAt:           pgtype.Timestamptz{Time: update.EndsAt, Valid: true},
		ID:               update.ID, ExpectedStatus: string(update.ExpectedStatus),
	})
	return mapGrant(row, mapTransitionError(err))
}

func (s *PostgresStore) MarkFailed(ctx context.Context, update FailureUpdate) (Grant, error) {
	row, err := s.queries.MarkWorkspaceTrialGrantFailed(ctx, db.MarkWorkspaceTrialGrantFailedParams{
		FailureCode: text(update.Code), FailureMessage: text(update.Message),
		ID: update.ID, ExpectedStatus: string(update.ExpectedStatus),
	})
	return mapGrant(row, mapTransitionError(err))
}

func (s *PostgresStore) MarkRevoked(ctx context.Context, id string, expected Status, at time.Time) (Grant, error) {
	row, err := s.queries.MarkWorkspaceTrialGrantRevoked(ctx, db.MarkWorkspaceTrialGrantRevokedParams{
		RevokedAt: pgtype.Timestamptz{Time: at, Valid: true}, ID: id, ExpectedStatus: string(expected),
	})
	return mapGrant(row, mapTransitionError(err))
}

type ManagerModeResolver struct {
	manager *billing.Manager
}

func NewManagerModeResolver(manager *billing.Manager) *ManagerModeResolver {
	return &ManagerModeResolver{manager: manager}
}

func (r *ManagerModeResolver) Resolve(ctx context.Context, ownerUserID, planID string) (BillingMode, error) {
	if r == nil || r.manager == nil {
		return BillingMode{}, ErrBillingModeUnavailable
	}
	mode := r.manager.For(ctx, ownerUserID)
	if mode == nil || mode.Name == "" || mode.PriceID(planID) == "" {
		return BillingMode{}, ErrBillingModeUnavailable
	}
	return BillingMode{Name: mode.Name, PriceID: mode.PriceID(planID)}, nil
}

func mapGrant(row db.WorkspaceTrialGrant, err error) (Grant, error) {
	if err != nil {
		return Grant{}, err
	}
	return Grant{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Kind: Kind(row.Kind), PlanID: row.PlanID,
		DurationDays: row.DurationDays, Status: Status(row.Status), ActorUserID: row.GrantedByUserID,
		StripeMode: row.StripeMode.String, StripeCustomerID: row.StripeCustomerID.String,
		StripeSubscriptionID: row.StripeSubscriptionID.String, StripeScheduleID: row.StripeScheduleID.String,
		StripeCheckoutSessionID: row.StripeCheckoutSessionID.String, GrantedAt: row.GrantedAt.Time.UTC(),
		ScheduledStartAt: optionalTime(row.ScheduledStartAt), StartedAt: optionalTime(row.StartedAt),
		EndsAt: optionalTime(row.EndsAt), ActivatedAt: optionalTime(row.ActivatedAt),
		CanceledAt: optionalTime(row.CanceledAt), RevokedAt: optionalTime(row.RevokedAt),
		SupersededAt: optionalTime(row.SupersededAt), CompletedAt: optionalTime(row.CompletedAt),
		SupersededByPlanID: row.SupersededByPlanID.String,
		FailureCode:        row.FailureCode.String, FailureMessage: row.FailureMessage.String,
	}, nil
}

func mapLookupError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrGrantNotFound
	}
	return err
}

func mapTransitionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConcurrentTransition
	}
	return err
}

func mapCreateError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "workspace_trial_grants_one_open_per_workspace" {
		return ErrOpenGrantExists
	}
	return err
}

func text(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
