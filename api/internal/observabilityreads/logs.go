package observabilityreads

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

const logCursorVersion = 1

type LogSourceKind string

const (
	LogSourceRequest     LogSourceKind = "request"
	LogSourceIntegration LogSourceKind = "integration"
)

// LogProjection is the metadata-only representation shared by canonical
// request events and integration logs. SourceID is retained only for stable
// ordering; clients receive the source-qualified ID.
type LogProjection struct {
	ID               string          `json:"id"`
	SourceKind       LogSourceKind   `json:"source_kind,omitempty"`
	SourceID         string          `json:"-"`
	WorkspaceID      string          `json:"workspace_id"`
	WorkspaceName    string          `json:"workspace_name,omitempty"`
	OwnerEmail       string          `json:"owner_email,omitempty"`
	PlanID           string          `json:"plan_id,omitempty"`
	Timestamp        time.Time       `json:"ts"`
	Level            string          `json:"level"`
	Status           string          `json:"status"`
	Category         string          `json:"category"`
	Action           string          `json:"action"`
	Source           string          `json:"source"`
	Message          string          `json:"message"`
	RequestID        string          `json:"request_id,omitempty"`
	TraceID          string          `json:"trace_id,omitempty"`
	ActorUserID      string          `json:"actor_user_id,omitempty"`
	ActorAPIKeyID    string          `json:"actor_api_key_id,omitempty"`
	ProfileID        string          `json:"profile_id,omitempty"`
	SocialAccountID  string          `json:"social_account_id,omitempty"`
	PostID           string          `json:"post_id,omitempty"`
	PlatformPostID   string          `json:"platform_post_id,omitempty"`
	Platform         string          `json:"platform,omitempty"`
	Endpoint         string          `json:"endpoint,omitempty"`
	Method           string          `json:"method,omitempty"`
	HTTPStatusCode   *int32          `json:"http_status_code,omitempty"`
	RemoteStatusCode *int32          `json:"remote_status_code,omitempty"`
	DurationMs       *int32          `json:"duration_ms,omitempty"`
	ErrorCode        string          `json:"error_code,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	HasErrorDetail   bool            `json:"has_error_detail,omitempty"`
	DetailStatus     string          `json:"detail_status,omitempty"`
}

type LogCursor struct {
	Timestamp  time.Time
	SourceKind LogSourceKind
	SourceID   string
}

type LogPage struct {
	Logs       []LogProjection
	NextCursor *LogCursor
}

// LogFilters is shared by customer and admin log lists. WorkspaceID and
// OwnerEmail are honored only by the global admin reader; customer callers
// pass their authenticated workspace separately.
type LogFilters struct {
	WorkspaceID     string
	OwnerEmail      string
	Category        string
	Action          string
	Source          string
	Level           string
	Status          string
	Platform        string
	ProfileID       string
	SocialAccountID string
	PostID          string
	RequestID       string
	ErrorCode       string
	Query           string
	Method          string
	Endpoint        string
	From            time.Time
	To              time.Time
}

// LogDetail embeds the metadata projection and adds only the source-specific,
// bounded detail loaded after the base record has been authorized.
type LogDetail struct {
	LogProjection
	RequestPayload         json.RawMessage `json:"request_payload,omitempty"`
	ResponsePayload        json.RawMessage `json:"response_payload,omitempty"`
	RequestHeaders         json.RawMessage `json:"request_headers,omitempty"`
	ResponseHeaders        json.RawMessage `json:"response_headers,omitempty"`
	QueryKeys              []string        `json:"query_keys,omitempty"`
	RequestOriginalBytes   int64           `json:"request_original_bytes,omitempty"`
	RequestStoredBytes     int32           `json:"request_stored_bytes,omitempty"`
	ResponseOriginalBytes  int64           `json:"response_original_bytes,omitempty"`
	ResponseStoredBytes    int32           `json:"response_stored_bytes,omitempty"`
	RequestContentType     string          `json:"request_content_type,omitempty"`
	ResponseContentType    string          `json:"response_content_type,omitempty"`
	RequestSHA256          string          `json:"request_sha256,omitempty"`
	ResponseSHA256         string          `json:"response_sha256,omitempty"`
	RequestTruncated       bool            `json:"request_truncated,omitempty"`
	ResponseTruncated      bool            `json:"response_truncated,omitempty"`
	RequestOmissionReason  string          `json:"request_omission_reason,omitempty"`
	ResponseOmissionReason string          `json:"response_omission_reason,omitempty"`
	RedactionPolicyVersion int32           `json:"redaction_policy_version,omitempty"`
}

type encodedLogCursor struct {
	Version    int           `json:"v"`
	Timestamp  time.Time     `json:"timestamp"`
	SourceKind LogSourceKind `json:"source_kind"`
	SourceID   string        `json:"source_id"`
}

func EncodeLogID(sourceKind LogSourceKind, sourceID string) (string, error) {
	if err := validateSourceIdentity(sourceKind, sourceID); err != nil {
		return "", err
	}
	return string(sourceKind) + ":" + sourceID, nil
}

func DecodeLogID(id string) (LogSourceKind, string, error) {
	if id != strings.TrimSpace(id) {
		return "", "", errors.New("log ID contains surrounding whitespace")
	}
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return "", "", errors.New("log ID is not source-qualified")
	}
	sourceKind := LogSourceKind(parts[0])
	if err := validateSourceIdentity(sourceKind, parts[1]); err != nil {
		return "", "", err
	}
	return sourceKind, parts[1], nil
}

// DecodeLogLookupID accepts the source-qualified v2 identity and the exact
// positive decimal integration-log identity returned by the legacy API. The
// compatibility form is intentionally lookup-only: newly emitted v2 IDs stay
// source-qualified and opaque.
func DecodeLogLookupID(id string) (LogSourceKind, string, error) {
	if strings.Contains(id, ":") {
		return DecodeLogID(id)
	}
	if err := validateSourceIdentity(LogSourceIntegration, id); err != nil {
		return "", "", err
	}
	return LogSourceIntegration, id, nil
}

func EncodeLogCursor(cursor LogCursor) (string, error) {
	if cursor.Timestamp.IsZero() {
		return "", errors.New("cursor timestamp is required")
	}
	if err := validateSourceIdentity(cursor.SourceKind, cursor.SourceID); err != nil {
		return "", fmt.Errorf("invalid cursor identity: %w", err)
	}
	payload, err := json.Marshal(encodedLogCursor{
		Version:    logCursorVersion,
		Timestamp:  cursor.Timestamp.UTC(),
		SourceKind: cursor.SourceKind,
		SourceID:   cursor.SourceID,
	})
	if err != nil {
		return "", err
	}
	return "v1." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecodeLogCursor(value string) (LogCursor, error) {
	if value != strings.TrimSpace(value) || !strings.HasPrefix(value, "v1.") {
		return LogCursor{}, errors.New("invalid cursor version")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "v1."))
	if err != nil {
		return LogCursor{}, errors.New("invalid cursor encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var encoded encodedLogCursor
	if err := decoder.Decode(&encoded); err != nil {
		return LogCursor{}, errors.New("invalid cursor payload")
	}
	if err := requireJSONEnd(decoder); err != nil {
		return LogCursor{}, errors.New("invalid cursor payload")
	}
	if encoded.Version != logCursorVersion || encoded.Timestamp.IsZero() {
		return LogCursor{}, errors.New("invalid cursor payload")
	}
	if err := validateSourceIdentity(encoded.SourceKind, encoded.SourceID); err != nil {
		return LogCursor{}, errors.New("invalid cursor payload")
	}
	return LogCursor{
		Timestamp:  encoded.Timestamp.UTC(),
		SourceKind: encoded.SourceKind,
		SourceID:   encoded.SourceID,
	}, nil
}

// CompareLogProjections returns -1 when left sorts before right, 1 when it
// sorts after right, and 0 when they represent the same page position.
func CompareLogProjections(left, right LogProjection) int {
	return compareLogPositions(
		LogCursor{Timestamp: left.Timestamp, SourceKind: left.SourceKind, SourceID: left.SourceID},
		LogCursor{Timestamp: right.Timestamp, SourceKind: right.SourceKind, SourceID: right.SourceID},
	)
}

// MergeLogPage globally orders two source candidate sets, applies a
// source-qualified cursor, and reads at most limit+1 positions to determine
// whether another page exists. It never uses or derives an offset.
func MergeLogPage(requestLogs, integrationLogs []LogProjection, after *LogCursor, limit int) (LogPage, error) {
	if limit <= 0 {
		return LogPage{}, errors.New("page limit must be positive")
	}
	if after != nil {
		if after.Timestamp.IsZero() {
			return LogPage{}, errors.New("cursor timestamp is required")
		}
		if err := validateSourceIdentity(after.SourceKind, after.SourceID); err != nil {
			return LogPage{}, fmt.Errorf("invalid cursor identity: %w", err)
		}
	}

	merged := make([]LogProjection, 0, len(requestLogs)+len(integrationLogs))
	for _, log := range requestLogs {
		if err := normalizeLogProjection(&log, LogSourceRequest); err != nil {
			return LogPage{}, err
		}
		merged = append(merged, log)
	}
	for _, log := range integrationLogs {
		if err := normalizeLogProjection(&log, LogSourceIntegration); err != nil {
			return LogPage{}, err
		}
		merged = append(merged, log)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return CompareLogProjections(merged[i], merged[j]) < 0
	})

	candidates := make([]LogProjection, 0, min(limit+1, len(merged)))
	for _, log := range merged {
		position := LogCursor{Timestamp: log.Timestamp, SourceKind: log.SourceKind, SourceID: log.SourceID}
		if after != nil && compareLogPositions(position, *after) <= 0 {
			continue
		}
		candidates = append(candidates, log)
		if len(candidates) == limit+1 {
			break
		}
	}

	page := LogPage{Logs: candidates}
	if len(candidates) > limit {
		page.Logs = candidates[:limit]
		last := page.Logs[len(page.Logs)-1]
		page.NextCursor = &LogCursor{
			Timestamp:  last.Timestamp.UTC(),
			SourceKind: last.SourceKind,
			SourceID:   last.SourceID,
		}
	}
	return page, nil
}

func normalizeLogProjection(log *LogProjection, expected LogSourceKind) error {
	if log.Timestamp.IsZero() {
		return errors.New("log timestamp is required")
	}
	if log.SourceKind == "" {
		log.SourceKind = expected
	}
	if log.SourceKind != expected {
		return fmt.Errorf("log source kind %q does not match candidate set %q", log.SourceKind, expected)
	}
	encodedID, err := EncodeLogID(log.SourceKind, log.SourceID)
	if err != nil {
		return err
	}
	if log.ID == "" {
		log.ID = encodedID
	} else if log.ID != encodedID {
		return fmt.Errorf("log ID %q does not match source identity %q", log.ID, encodedID)
	}
	log.Timestamp = log.Timestamp.UTC()
	return nil
}

func compareLogPositions(left, right LogCursor) int {
	if !left.Timestamp.Equal(right.Timestamp) {
		if left.Timestamp.After(right.Timestamp) {
			return -1
		}
		return 1
	}
	if left.SourceKind != right.SourceKind {
		if sourceKindRank(left.SourceKind) < sourceKindRank(right.SourceKind) {
			return -1
		}
		return 1
	}
	return compareSourceIDsDescending(left.SourceKind, left.SourceID, right.SourceID)
}

func compareSourceIDsDescending(sourceKind LogSourceKind, left, right string) int {
	if left == right {
		return 0
	}
	if sourceKind == LogSourceIntegration {
		leftNumber, _ := strconv.ParseInt(left, 10, 64)
		rightNumber, _ := strconv.ParseInt(right, 10, 64)
		if leftNumber > rightNumber {
			return -1
		}
		return 1
	}
	if left > right {
		return -1
	}
	return 1
}

func sourceKindRank(sourceKind LogSourceKind) int {
	if sourceKind == LogSourceRequest {
		return 0
	}
	return 1
}

func validateSourceIdentity(sourceKind LogSourceKind, sourceID string) error {
	if sourceID == "" || sourceID != strings.TrimSpace(sourceID) {
		return errors.New("source-local log ID is required")
	}
	switch sourceKind {
	case LogSourceRequest:
		return nil
	case LogSourceIntegration:
		id, err := strconv.ParseInt(sourceID, 10, 64)
		if err != nil || id <= 0 || sourceID != strconv.FormatInt(id, 10) {
			return errors.New("integration log ID must be a positive integer")
		}
		return nil
	default:
		return fmt.Errorf("unknown log source kind %q", sourceKind)
	}
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("cursor has trailing data")
	}
	return nil
}
