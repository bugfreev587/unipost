package publishingrestrictions

import (
	"errors"
	"time"
)

const (
	APICode        = "PLAN_PLATFORM_PUBLISHING_RESTRICTED"
	NormalizedCode = "plan_platform_publishing_restricted"
	UserMessage    = "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted."
	NextAction     = "upgrade_or_wait_then_retry"
	FailureStage   = "publishing_policy"
	ResultTitle    = "Publishing restricted"
)

var ErrVersionConflict = errors.New("publishing restriction version conflict")

type Restriction struct {
	ID                 string     `json:"id"`
	Platform           string     `json:"platform"`
	Enabled            bool       `json:"enabled"`
	RestrictedPlanIDs  []string   `json:"restricted_plan_ids"`
	ReasonCode         string     `json:"reason_code"`
	UserMessage        string     `json:"user_message"`
	CycleID            string     `json:"cycle_id,omitempty"`
	Version            int64      `json:"version"`
	EnabledAt          *time.Time `json:"enabled_at,omitempty"`
	DisabledAt         *time.Time `json:"disabled_at,omitempty"`
	UpdatedByUserID    string     `json:"updated_by_user_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	AffectedWorkspaces int        `json:"affected_workspace_count,omitempty"`
	AffectedAccounts   int        `json:"affected_account_count,omitempty"`
}

type Decision struct {
	Restricted bool   `json:"restricted"`
	Platform   string `json:"platform"`
	PlanID     string `json:"plan_id"`
	ReasonCode string `json:"reason_code,omitempty"`
	CycleID    string `json:"-"`
	Version    int64  `json:"-"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
	NextAction string `json:"next_action,omitempty"`
}

type TransitionRequest struct {
	Platform        string
	Enabled         bool
	ExpectedVersion int64
	ActorUserID     string
	RequestID       string
	ActorIP         string
	ActorUserAgent  string
}

type TransitionResult struct {
	Restriction Restriction `json:"restriction"`
	Changed     bool        `json:"changed"`
}

type VersionConflictError struct {
	Current Restriction
}

func (e *VersionConflictError) Error() string { return ErrVersionConflict.Error() }
func (e *VersionConflictError) Unwrap() error { return ErrVersionConflict }
