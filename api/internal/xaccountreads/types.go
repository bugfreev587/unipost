package xaccountreads

import (
	"errors"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

const (
	ReceiptExecuting         = "executing"
	ReceiptOutcomeUnknown    = "outcome_unknown"
	ReceiptSettlementPending = "settlement_pending"
	ReceiptSucceeded         = "succeeded"
	ReceiptFailed            = "failed"

	CodeIdempotencyConflict   = "IDEMPOTENCY_CONFLICT"
	CodeReadInProgress        = "READ_IN_PROGRESS"
	CodeReadSettlementPending = "READ_SETTLEMENT_PENDING"
	CodeInsufficientCredits   = "INSUFFICIENT_X_CREDITS"
	CodeRateLimited           = "RATE_LIMITED"
	CodeReauthorization       = "ACCOUNT_REAUTHORIZATION_REQUIRED"
	CodeXUpstream             = "X_UPSTREAM_ERROR"
)

var (
	ErrReceiptNotFound        = errors.New("X account-read receipt not found")
	ErrAccountReadRateLimited = errors.New("X account-read operational rate limit exceeded")
)

type OperationError struct {
	Code               string
	RetryAfter         time.Duration
	Cause              error
	EstimatedCredits   int64
	AvailableCredits   int64
	MaxAffordableLimit int
}

func (e *OperationError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Cause.Error()
	}
	return e.Code
}

func (e *OperationError) Unwrap() error { return e.Cause }

type Receipt struct {
	OperationID        string
	WorkspaceID        string
	AccountID          string
	ExternalUserID     string
	ExposureID         string
	Endpoint           string
	IdempotencyKeyHash string
	RequestFingerprint string
	EncryptedRequest   string
	RequestedResources int
	OperationKey       string
	Status             string
	ResponseJSON       []byte
	FailureClass       string
	AttemptCount       int
	NextAttemptAt      time.Time
	CompletedAt        *time.Time
	ExpiresAt          time.Time
	AccountingEnabled  bool
	BypassReason       string
	ReservedUnits      int64
	ActualUnits        int64
	CatalogVersion     string
	AppMode            string
}

type NewReceipt struct {
	Receipt
}

func (r NewReceipt) WithExposure(exposureID string) Receipt {
	receipt := r.Receipt
	receipt.ExposureID = exposureID
	return receipt
}

type CreditsReceipt struct {
	OperationID       string `json:"operation_id"`
	Status            string `json:"status"`
	AccountingEnabled bool   `json:"accounting_enabled"`
	BillingMode       string `json:"billing_mode"`
	BypassReason      string `json:"bypass_reason,omitempty"`
	Operation         string `json:"operation"`
	Estimated         int64  `json:"estimated"`
	Reserved          int64  `json:"reserved"`
	Charged           int64  `json:"charged"`
	Released          int64  `json:"released"`
	CatalogVersion    string `json:"catalog_version"`
}

type ProfileRequest struct {
	WorkspaceID       string
	AccountID         string
	ExternalUserID    string
	ExternalAccountID string
	AppMode           string
	AccessToken       string
	IdempotencyKey    string
	Now               time.Time
}

type ProfileData struct {
	AccountID string `json:"account_id"`
	Platform  string `json:"platform"`
	platform.TwitterAccountProfile
	RetrievedAt time.Time `json:"retrieved_at"`
}

type ProfileMeta struct {
	Credits  CreditsReceipt `json:"credits"`
	Replayed bool           `json:"replayed,omitempty"`
}

type ProfileResult struct {
	Data ProfileData `json:"data"`
	Meta ProfileMeta `json:"meta"`
}

type PostsRequest struct {
	WorkspaceID            string
	AccountID              string
	ExternalUserID         string
	ExternalAccountID      string
	AppMode                string
	AccessToken            string
	IdempotencyKey         string
	Limit                  int
	Cursor                 string
	StartTime              *time.Time
	EndTime                *time.Time
	ExcludeReposts         bool
	ExcludeRepliesToOthers bool
	Now                    time.Time
}

type PostThread struct {
	ThreadID string `json:"thread_id"`
}

type PostData struct {
	platform.TwitterAuthoredPost
	AccountID string     `json:"account_id"`
	Thread    PostThread `json:"thread"`
}

type PostsMeta struct {
	Limit           int            `json:"limit"`
	ScannedCount    int            `json:"scanned_count"`
	ReturnedCount   int            `json:"returned_count"`
	HasMore         bool           `json:"has_more"`
	NextCursor      string         `json:"next_cursor,omitempty"`
	CursorExpiresAt *time.Time     `json:"cursor_expires_at,omitempty"`
	Credits         CreditsReceipt `json:"credits"`
	Replayed        bool           `json:"replayed,omitempty"`
}

type PostsResult struct {
	Data []PostData `json:"data"`
	Meta PostsMeta  `json:"meta"`
}
