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
	ID                                       string             `json:"id"`
	Platform                                 string             `json:"platform"`
	Enabled                                  bool               `json:"enabled"`
	RestrictedPlanIDs                        []string           `json:"restricted_plan_ids"`
	ReasonCode                               string             `json:"reason_code"`
	UserMessage                              string             `json:"user_message"`
	CycleID                                  string             `json:"cycle_id,omitempty"`
	Version                                  int64              `json:"version"`
	EnabledAt                                *time.Time         `json:"enabled_at,omitempty"`
	DisabledAt                               *time.Time         `json:"disabled_at,omitempty"`
	UpdatedByUserID                          string             `json:"updated_by_user_id,omitempty"`
	CreatedAt                                time.Time          `json:"created_at"`
	UpdatedAt                                time.Time          `json:"updated_at"`
	AffectedWorkspaces                       int                `json:"affected_workspace_count,omitempty"`
	AffectedAccounts                         int                `json:"affected_account_count,omitempty"`
	CurrentCycleRetainedObjectCount          int64              `json:"current_cycle_retained_object_count,omitempty"`
	CurrentCycleRetainedBytes                int64              `json:"current_cycle_retained_bytes,omitempty"`
	CurrentCycleProjected60DayStorageCostUSD float64            `json:"current_cycle_projected_60_day_storage_cost_usd,omitempty"`
	RecentEvents                             []RestrictionEvent `json:"recent_events,omitempty"`
}

type RestrictionEvent struct {
	ID               string    `json:"id"`
	EventType        string    `json:"event_type"`
	ActorUserID      string    `json:"actor_user_id,omitempty"`
	ExpectedVersion  int64     `json:"expected_version"`
	ResultingVersion int64     `json:"resulting_version"`
	CreatedAt        time.Time `json:"created_at"`
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

type CampaignType string

const (
	RestrictionNotice CampaignType = "restriction_notice"
	RecoveryNotice    CampaignType = "recovery_notice"
)

type CampaignCopy struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func CampaignCopyFor(campaignType CampaignType) CampaignCopy {
	switch campaignType {
	case RestrictionNotice:
		return CampaignCopy{
			Subject: "Temporary TikTok publishing restriction for Free plans",
			Body:    "Hi {{first_name}},\n\nWe’re sorry for the disruption.\n\nTikTok applies a daily capacity limit to publishing through UniPost. Because our current capacity has been reached, TikTok publishing is temporarily unavailable for Free-plan users.\n\nYour connected TikTok accounts, saved posts, and other publishing platforms are not affected. Paid plans will continue to have access to TikTok publishing while this restriction is active.\n\nWe have asked TikTok to increase our publishing capacity and will let you know as soon as TikTok publishing becomes available again on the Free plan.\n\nThank you for your patience and understanding.\n\nThe UniPost Team",
		}
	case RecoveryNotice:
		return CampaignCopy{
			Subject: "TikTok publishing is available again on the Free plan",
			Body:    "Hi {{first_name}},\n\nTikTok publishing is now available again on the UniPost Free plan.\n\nYou can create and publish new TikTok posts from your connected accounts. Posts that previously failed because of the temporary capacity restriction will not publish automatically; please review and publish them again when you’re ready.\n\nThank you for your patience while we worked to increase publishing capacity.\n\nThe UniPost Team",
		}
	default:
		return CampaignCopy{}
	}
}

type CampaignPreview struct {
	Platform           string       `json:"platform"`
	CycleID            string       `json:"cycle_id"`
	CampaignType       CampaignType `json:"campaign_type"`
	RestrictionVersion int64        `json:"restriction_version"`
	RecipientCount     int          `json:"recipient_count"`
	Subject            string       `json:"subject"`
	Body               string       `json:"body"`
	PreviewToken       string       `json:"preview_token"`
	ExpiresAt          time.Time    `json:"expires_at"`
}

type Campaign struct {
	ID                 string       `json:"id"`
	RestrictionID      string       `json:"restriction_id"`
	CycleID            string       `json:"cycle_id"`
	CampaignType       CampaignType `json:"campaign_type"`
	Status             string       `json:"status"`
	SubjectSnapshot    string       `json:"subject_snapshot"`
	BodySnapshot       string       `json:"body_snapshot"`
	RestrictionVersion int64        `json:"restriction_version"`
	PreviewedCount     int          `json:"previewed_count"`
	SnapshottedCount   int          `json:"snapshotted_count"`
	PendingCount       int          `json:"pending_count"`
	SentCount          int          `json:"sent_count"`
	FailedCount        int          `json:"failed_count"`
	SkippedCount       int          `json:"skipped_count"`
	CreatedByUserID    string       `json:"created_by_user_id,omitempty"`
	ConfirmedByUserID  string       `json:"confirmed_by_user_id,omitempty"`
	SnapshottedAt      time.Time    `json:"snapshotted_at"`
	StartedAt          *time.Time   `json:"started_at,omitempty"`
	CompletedAt        *time.Time   `json:"completed_at,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

type RecipientSnapshot struct {
	CanonicalUserID         string
	RecipientEmail          string
	NormalizedEmail         string
	FirstName               string
	RepresentedWorkspaceIDs []string
}

type ConfirmCampaignRequest struct {
	Platform     string
	CampaignType CampaignType
	PreviewToken string
	Confirmation string
	ActorUserID  string
}
