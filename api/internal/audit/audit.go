// Package audit centralizes writing audit_log rows. The pattern is:
//
//	audit.Log(ctx, queries, audit.Event{
//	    WorkspaceID:  ws,
//	    ActorUserID:  uid,
//	    Action:       "MEMBER.INVITED",
//	    ResourceType: "invite",
//	    ResourceID:   invite.ID,
//	    Category:     audit.CategoryMembership,
//	    After:        invite,
//	})
//
// All writes are best-effort: a logging failure does NOT cause the
// caller's mutation to fail. The point of audit is "after-the-fact
// transparency", not "block the action". Errors are logged via slog
// at warn level and otherwise swallowed.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// Categories — coarse-grained buckets the dashboard filter UI uses.
const (
	CategoryMembership = "membership"
	CategoryBilling    = "billing"
	CategoryConfig     = "config"
	CategoryPublishing = "publishing"
	CategoryAuth       = "auth"
)

// Action codes follow DOMAIN.VERB. Add constants here when wiring a
// new mutation so call sites don't drift on string literals.
const (
	ActionMemberInvited        = "MEMBER.INVITED"
	ActionMemberJoined         = "MEMBER.JOINED"
	ActionMemberRemoved        = "MEMBER.REMOVED"
	ActionMemberRoleChanged    = "MEMBER.ROLE_CHANGED"
	ActionMemberInviteRevoked  = "MEMBER.INVITE_REVOKED"
	ActionOwnershipTransferred = "WORKSPACE.OWNERSHIP_TRANSFERRED"

	ActionAPIKeyCreated = "API_KEY.CREATED"
	ActionAPIKeyRevoked = "API_KEY.REVOKED"

	ActionPlatformCredentialCreated = "PLATFORM_CREDENTIAL.CREATED"
	ActionPlatformCredentialDeleted = "PLATFORM_CREDENTIAL.DELETED"

	ActionPlanChanged = "PLAN.CHANGED"
)

// Event is the input shape for Log. Pointer-y fields default to NULL
// when zero-valued, so callers can omit irrelevant fields.
type Event struct {
	WorkspaceID   string
	ActorUserID   string // exactly one of ActorUserID / ActorAPIKeyID should be set
	ActorAPIKeyID string
	Action        string
	ResourceType  string
	ResourceID    string
	Category      string
	IPAddress     string
	UserAgent     string
	Before        any // pre-mutation snapshot (omitted for create)
	After         any // post-mutation snapshot (omitted for delete)
	Metadata      any // arbitrary extra context
}

// Log writes a single audit row. Best-effort: errors are logged but
// never returned. Callers should NEVER condition on the return value.
func Log(ctx context.Context, queries *db.Queries, e Event) {
	if queries == nil || e.WorkspaceID == "" || e.Action == "" {
		return
	}
	beforeJSON := jsonBytes(e.Before)
	afterJSON := jsonBytes(e.After)
	metaJSON := jsonBytes(e.Metadata)

	err := queries.WriteAuditLog(ctx, db.WriteAuditLogParams{
		WorkspaceID:   e.WorkspaceID,
		ActorUserID:   pgText(e.ActorUserID),
		ActorApiKeyID: pgText(e.ActorAPIKeyID),
		Action:        e.Action,
		ResourceType:  e.ResourceType,
		ResourceID:    pgText(e.ResourceID),
		Category:      e.Category,
		IpAddress:     pgText(e.IPAddress),
		UserAgent:     pgText(e.UserAgent),
		BeforeJson:    beforeJSON,
		AfterJson:     afterJSON,
		Metadata:      metaJSON,
	})
	if err != nil {
		slog.Warn("audit_log_write_failed",
			"workspace_id", e.WorkspaceID,
			"action", e.Action,
			"resource_type", e.ResourceType,
			"resource_id", e.ResourceID,
			"error", err)
	}
}

func pgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func jsonBytes(v any) []byte {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
