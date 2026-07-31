package xcredits

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Event struct {
	OperationID    string     `json:"operation_id"`
	AccountID      string     `json:"account_id,omitempty"`
	ExternalUserID string     `json:"external_user_id,omitempty"`
	Operation      string     `json:"operation"`
	CatalogVersion string     `json:"catalog_version"`
	Estimated      int64      `json:"estimated"`
	Reserved       int64      `json:"reserved"`
	Charged        int64      `json:"charged"`
	Released       int64      `json:"released"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	FinalizedAt    *time.Time `json:"finalized_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type ListEventsRequest struct {
	WorkspaceID    string
	AccountID      string
	ExternalUserID string
	Operation      string
	Status         string
	StartTime      *time.Time
	EndTime        *time.Time
	Cursor         string
	Limit          int
}

type EventPage struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type eventCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func encodeEventCursor(cursor eventCursor) string {
	body, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodeEventCursor(value string) (eventCursor, error) {
	if strings.TrimSpace(value) == "" {
		return eventCursor{}, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return eventCursor{}, errors.New("invalid X Credits event cursor")
	}
	var cursor eventCursor
	if err := json.Unmarshal(body, &cursor); err != nil || cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return eventCursor{}, errors.New("invalid X Credits event cursor")
	}
	return cursor, nil
}

type EventStore interface {
	ListEvents(context.Context, ListEventsRequest) (EventPage, error)
}

func (s *Service) ListEvents(ctx context.Context, req ListEventsRequest) (EventPage, error) {
	store, ok := s.store.(EventStore)
	if !ok {
		return EventPage{}, errors.New("X Credits event store is not configured")
	}
	if req.WorkspaceID == "" {
		return EventPage{}, errors.New("workspace_id is required")
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	return store.ListEvents(ctx, req)
}
