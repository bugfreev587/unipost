package platformcontrols

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const APICode = "PLATFORM_OPERATION_DISABLED"

var (
	ErrUnsupportedPlatform  = errors.New("unsupported platform")
	ErrUnsupportedOperation = errors.New("unsupported platform operation")
	ErrUnsupportedState     = errors.New("unsupported platform operational state")
	ErrVersionConflict      = errors.New("platform operational control version conflict")
)

type State string

const (
	StateActive             State = "active"
	StatePublishingDisabled State = "publishing_disabled"
	StateFullyDisabled      State = "fully_disabled"
)

func (s State) Valid() bool {
	switch s {
	case StateActive, StatePublishingDisabled, StateFullyDisabled:
		return true
	default:
		return false
	}
}

type Operation string

const (
	OperationConnect Operation = "connect"
	OperationPublish Operation = "publish"
	OperationRead    Operation = "read"
)

func (o Operation) Valid() bool {
	switch o {
	case OperationConnect, OperationPublish, OperationRead:
		return true
	default:
		return false
	}
}

var SupportedPlatforms = []string{
	"twitter",
	"linkedin",
	"bluesky",
	"youtube",
	"tiktok",
	"instagram",
	"threads",
	"facebook",
	"pinterest",
}

var supportedPlatformSet = func() map[string]struct{} {
	result := make(map[string]struct{}, len(SupportedPlatforms))
	for _, platform := range SupportedPlatforms {
		result[platform] = struct{}{}
	}
	return result
}()

func NormalizePlatform(platform string) (string, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if _, ok := supportedPlatformSet[platform]; !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPlatform, platform)
	}
	return platform, nil
}

type Control struct {
	ID              string    `json:"id"`
	Platform        string    `json:"platform"`
	State           State     `json:"state"`
	ReasonCode      string    `json:"reason_code"`
	UserMessage     string    `json:"user_message"`
	Version         int64     `json:"version"`
	StateChangedAt  time.Time `json:"state_changed_at"`
	UpdatedByUserID string    `json:"updated_by_user_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Decision struct {
	Platform   string    `json:"platform"`
	State      State     `json:"state"`
	Operation  Operation `json:"operation"`
	Blocked    bool      `json:"blocked"`
	ReasonCode string    `json:"reason_code,omitempty"`
	Message    string    `json:"message,omitempty"`
	Version    int64     `json:"-"`
}

type TransitionRequest struct {
	Platform        string
	State           State
	ExpectedVersion int64
	ReasonCode      string
	UserMessage     string
	ActorUserID     string
	RequestID       string
	ActorIP         string
	ActorUserAgent  string
}

type TransitionResult struct {
	Control Control `json:"control"`
	Changed bool    `json:"changed"`
}

type VersionConflictError struct {
	Current Control
}

func (e *VersionConflictError) Error() string { return ErrVersionConflict.Error() }
func (e *VersionConflictError) Unwrap() error { return ErrVersionConflict }

type Evaluator interface {
	Evaluate(context.Context, string, Operation) (Decision, error)
}

type Store interface {
	ControlForPlatform(context.Context, string) (Control, error)
	List(context.Context) ([]Control, error)
	SetState(context.Context, TransitionRequest) (TransitionResult, error)
}
