package trials

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type Kind string

const (
	KindFreeToPaid   Kind = "free_to_paid"
	KindPaidSamePlan Kind = "paid_same_plan"
)

type Status string

const (
	StatusProvisioning      Status = "provisioning"
	StatusPendingActivation Status = "pending_activation"
	StatusCheckoutPending   Status = "checkout_pending"
	StatusScheduled         Status = "scheduled"
	StatusActive            Status = "active"
	StatusCompleted         Status = "completed"
	StatusCanceled          Status = "canceled"
	StatusRevoked           Status = "revoked"
	StatusSuperseded        Status = "superseded"
	StatusFailed            Status = "failed"
)

var (
	ErrInvalidPlan       = errors.New("invalid trial plan")
	ErrInvalidDuration   = errors.New("invalid trial duration")
	ErrInvalidTransition = errors.New("invalid trial status transition")
)

func ValidateGrantInput(planID string, durationDays int32) error {
	if durationDays < 1 || durationDays > 730 {
		return ErrInvalidDuration
	}

	switch planID {
	case "api", "basic", "growth", "team":
		return nil
	default:
		return ErrInvalidPlan
	}
}

var allowedTransitions = map[Status]map[Status]struct{}{
	StatusProvisioning: {
		StatusScheduled: {},
		StatusFailed:    {},
	},
	StatusPendingActivation: {
		StatusCheckoutPending: {},
		StatusRevoked:         {},
		StatusSuperseded:      {},
	},
	StatusCheckoutPending: {
		StatusPendingActivation: {},
		StatusActive:            {},
		StatusRevoked:           {},
		StatusSuperseded:        {},
		StatusFailed:            {},
	},
	StatusScheduled: {
		StatusActive:     {},
		StatusCanceled:   {},
		StatusSuperseded: {},
		StatusFailed:     {},
	},
	StatusActive: {
		StatusCompleted:  {},
		StatusCanceled:   {},
		StatusSuperseded: {},
		StatusFailed:     {},
	},
}

func CanTransition(from, to Status) bool {
	destinations, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = destinations[to]
	return ok
}

type TransitionResult string

const (
	TransitionApplied    TransitionResult = "applied"
	TransitionIdempotent TransitionResult = "idempotent"
	TransitionInvalid    TransitionResult = "invalid"
)

func ValidateTransition(from, to Status) (TransitionResult, error) {
	if from == to && isKnownStatus(from) {
		return TransitionIdempotent, nil
	}
	if CanTransition(from, to) {
		return TransitionApplied, nil
	}
	return TransitionInvalid, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}

func IsOpen(status Status) bool {
	return status.IsOpen()
}

func (status Status) IsOpen() bool {
	switch status {
	case StatusProvisioning, StatusPendingActivation, StatusCheckoutPending, StatusScheduled, StatusActive:
		return true
	default:
		return false
	}
}

func IsTerminal(status Status) bool {
	return status.IsTerminal()
}

func (status Status) IsTerminal() bool {
	switch status {
	case StatusCompleted, StatusCanceled, StatusRevoked, StatusSuperseded, StatusFailed:
		return true
	default:
		return false
	}
}

func isKnownStatus(status Status) bool {
	return status.IsOpen() || status.IsTerminal()
}

type TerminalReason string

const (
	TerminalReasonNone            TerminalReason = ""
	TerminalReasonCompleted       TerminalReason = "trial_completed"
	TerminalReasonRenewalCanceled TerminalReason = "renewal_canceled"
	TerminalReasonOfferRevoked    TerminalReason = "offer_revoked"
	TerminalReasonPlanChanged     TerminalReason = "plan_changed"
	TerminalReasonUnavailable     TerminalReason = "trial_unavailable"
)

type ProjectionOptions struct {
	PostTrialPriceCents *int64
	CancelAtPeriodEnd   bool
}

type TrialProjection struct {
	ID                        string         `json:"id"`
	Kind                      Kind           `json:"kind"`
	PlanID                    string         `json:"plan_id"`
	DurationDays              int32          `json:"duration_days"`
	Status                    Status         `json:"status"`
	GrantedAt                 *time.Time     `json:"granted_at,omitempty"`
	ScheduledStartAt          *time.Time     `json:"scheduled_start_at,omitempty"`
	StartedAt                 *time.Time     `json:"started_at,omitempty"`
	EndsAt                    *time.Time     `json:"ends_at,omitempty"`
	ActivatedAt               *time.Time     `json:"activated_at,omitempty"`
	CanceledAt                *time.Time     `json:"canceled_at,omitempty"`
	RevokedAt                 *time.Time     `json:"revoked_at,omitempty"`
	SupersededAt              *time.Time     `json:"superseded_at,omitempty"`
	CompletedAt               *time.Time     `json:"completed_at,omitempty"`
	PostTrialPriceCents       *int64         `json:"post_trial_price_cents,omitempty"`
	CancelAtPeriodEnd         bool           `json:"cancel_at_period_end"`
	ChangingPlanForfeitsTrial bool           `json:"changing_plan_forfeits_trial"`
	TerminalReason            TerminalReason `json:"terminal_reason,omitempty"`
}

type HistoryProjection TrialProjection

func NewTrialProjection(grant db.WorkspaceTrialGrant, options ProjectionOptions) TrialProjection {
	status := Status(grant.Status)
	return TrialProjection{
		ID:                        grant.ID,
		Kind:                      Kind(grant.Kind),
		PlanID:                    grant.PlanID,
		DurationDays:              grant.DurationDays,
		Status:                    status,
		GrantedAt:                 projectionTime(grant.GrantedAt),
		ScheduledStartAt:          projectionTime(grant.ScheduledStartAt),
		StartedAt:                 projectionTime(grant.StartedAt),
		EndsAt:                    projectionTime(grant.EndsAt),
		ActivatedAt:               projectionTime(grant.ActivatedAt),
		CanceledAt:                projectionTime(grant.CanceledAt),
		RevokedAt:                 projectionTime(grant.RevokedAt),
		SupersededAt:              projectionTime(grant.SupersededAt),
		CompletedAt:               projectionTime(grant.CompletedAt),
		PostTrialPriceCents:       options.PostTrialPriceCents,
		CancelAtPeriodEnd:         options.CancelAtPeriodEnd,
		ChangingPlanForfeitsTrial: status == StatusScheduled || status == StatusActive,
		TerminalReason:            NormalizeTerminalReason(status),
	}
}

func NewHistoryProjection(grant db.WorkspaceTrialGrant, options ProjectionOptions) HistoryProjection {
	return HistoryProjection(NewTrialProjection(grant, options))
}

func NormalizeTerminalReason(status Status) TerminalReason {
	switch status {
	case StatusCompleted:
		return TerminalReasonCompleted
	case StatusCanceled:
		return TerminalReasonRenewalCanceled
	case StatusRevoked:
		return TerminalReasonOfferRevoked
	case StatusSuperseded:
		return TerminalReasonPlanChanged
	case StatusFailed:
		return TerminalReasonUnavailable
	default:
		return TerminalReasonNone
	}
}

func projectionTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
