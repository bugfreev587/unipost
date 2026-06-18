package changelog

import (
	"encoding/json"
	"errors"
	"time"
)

type Category string

const (
	CategoryAPI         Category = "api"
	CategorySDK         Category = "sdk"
	CategoryDashboard   Category = "dashboard"
	CategoryPlatform    Category = "platform"
	CategoryDX          Category = "dx"
	CategoryReliability Category = "reliability"
)

type Impact string

const (
	ImpactNew      Impact = "new"
	ImpactImproved Impact = "improved"
	ImpactChanged  Impact = "changed"
	ImpactFixed    Impact = "fixed"
)

type Ecosystem string

const (
	EcosystemNPM   Ecosystem = "npm"
	EcosystemPip   Ecosystem = "pip"
	EcosystemGo    Ecosystem = "go"
	EcosystemMaven Ecosystem = "maven"
)

type CandidateStatus string

const (
	StatusPending    CandidateStatus = "pending"
	StatusSaved      CandidateStatus = "saved"
	StatusDiscarded  CandidateStatus = "discarded"
	StatusPublishing CandidateStatus = "publishing"
	StatusPublished  CandidateStatus = "published"
	StatusFailed     CandidateStatus = "failed"
)

type Action string

const (
	ActionPublish Action = "publish"
	ActionSave    Action = "save"
	ActionDiscard Action = "discard"
)

var (
	ErrCandidateInvalid        = errors.New("candidate invalid")
	ErrCandidateNotFound       = errors.New("candidate not found")
	ErrCandidateAlreadyHandled = errors.New("candidate already handled")
	ErrInvalidSignature        = errors.New("invalid signature")
	ErrExpiredSignature        = errors.New("expired signature")
	ErrUnsupportedAction       = errors.New("unsupported action")
)

type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type SDKVersion struct {
	Ecosystem      Ecosystem `json:"ecosystem"`
	PackageName    string    `json:"packageName"`
	Version        string    `json:"version"`
	Href           string    `json:"href"`
	InstallCommand string    `json:"installCommand,omitempty"`
}

type Candidate struct {
	ID              string       `json:"id"`
	Date            string       `json:"date"`
	DisplayDate     string       `json:"displayDate,omitempty"`
	Title           string       `json:"title"`
	Summary         string       `json:"summary"`
	Category        Category     `json:"category"`
	Impact          Impact       `json:"impact"`
	IsBreaking      bool         `json:"isBreaking"`
	SDKVersions     []SDKVersion `json:"sdkVersions,omitempty"`
	Links           []Link       `json:"links"`
	SourceLinks     []Link       `json:"sourceLinks"`
	Confidence      string       `json:"confidence,omitempty"`
	WhyUserVisible  string       `json:"whyUserVisible"`
	ExcludedCommits []string     `json:"excludedCommits,omitempty"`
}

type CandidatePayload struct {
	HasCandidate    bool      `json:"hasCandidate"`
	Candidate       Candidate `json:"candidate,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	ExcludedCommits []string  `json:"excludedCommits,omitempty"`
}

type CandidateRecord struct {
	ID               string           `json:"id"`
	SourceHash       string           `json:"source_hash"`
	Status           CandidateStatus  `json:"status"`
	Payload          CandidatePayload `json:"payload"`
	PayloadJSON      json.RawMessage  `json:"-"`
	WindowStart      time.Time        `json:"window_start"`
	WindowEnd        time.Time        `json:"window_end"`
	DiscordMessageID string           `json:"discord_message_id,omitempty"`
	ActionRequestID  string           `json:"action_request_id,omitempty"`
	WorkflowRunURL   string           `json:"workflow_run_url,omitempty"`
	ActedByAdminID   string           `json:"acted_by_admin_id,omitempty"`
	ErrorMessage     string           `json:"error_message,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

type CreateCandidateInput struct {
	Payload          CandidatePayload `json:"payload"`
	SourceHash       string           `json:"source_hash"`
	WindowStart      time.Time        `json:"window_start"`
	WindowEnd        time.Time        `json:"window_end"`
	DiscordMessageID string           `json:"discord_message_id,omitempty"`
}

type ActionRequest struct {
	CandidateID  string
	Action       Action
	ExpiresUnix  int64
	Signature    string
	ActorAdminID string
}

type ActionResult struct {
	CandidateID    string          `json:"candidate_id"`
	Status         CandidateStatus `json:"status"`
	Action         Action          `json:"action"`
	Message        string          `json:"message"`
	WorkflowRunURL string          `json:"workflow_run_url,omitempty"`
}

type ActionLinks struct {
	Publish string `json:"publish"`
	Save    string `json:"save"`
	Discard string `json:"discard"`
}

func ValidAction(action Action) bool {
	switch action {
	case ActionPublish, ActionSave, ActionDiscard:
		return true
	default:
		return false
	}
}
