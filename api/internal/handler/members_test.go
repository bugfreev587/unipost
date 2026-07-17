package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/audit"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestMembersInviteTeamIgnoresFinitePlanThresholdAndAudits(t *testing.T) {
	store := &membersTestDB{planID: "team", activeMembers: 50}
	queries := db.New(store)
	h := NewMembersHandler(queries, quota.NewChecker(queries), nil, "https://app.unipost.dev")
	req := httptest.NewRequest(http.MethodPost, "/v1/members/invite", strings.NewReader(`{
		"email": "operator@release-lab.dev",
		"role": "editor"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_owner"))
	rec := httptest.NewRecorder()

	h.Invite(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", rec.Code, rec.Body.String())
	}
	if store.countMemberCalls != 0 {
		t.Fatalf("Team invite unexpectedly counted finite cap: calls=%d", store.countMemberCalls)
	}
	if store.createInviteCalls != 1 {
		t.Fatalf("create invite calls=%d, want 1", store.createInviteCalls)
	}
	write := requireSingleMembersAuditWrite(t, store)
	assertMembersAuditIdentity(t, write, audit.ActionMemberInvited, "invite", "invite_1")
	if strings.Contains(auditJSONPayload(write), store.lastInviteToken) {
		t.Fatal("invite token leaked into audit payload")
	}
}

func TestMembersInviteGrowthEnforcesMemberCap(t *testing.T) {
	store := &membersTestDB{planID: "growth", activeMembers: 3, maxMembers: 3}
	queries := db.New(store)
	h := NewMembersHandler(queries, quota.NewChecker(queries), nil, "https://app.unipost.dev")
	req := httptest.NewRequest(http.MethodPost, "/v1/members/invite", strings.NewReader(`{
		"email": "operator@release-lab.dev",
		"role": "editor"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_owner"))
	rec := httptest.NewRecorder()

	h.Invite(rec, req)

	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), "MEMBER_LIMIT_REACHED") {
		t.Fatalf("status=%d body=%s, want 402 MEMBER_LIMIT_REACHED", rec.Code, rec.Body.String())
	}
	if store.createInviteCalls != 0 {
		t.Fatalf("create invite calls=%d, want 0", store.createInviteCalls)
	}
}

func TestMembersAcceptInviteRejectsInvalidLifecycleStates(t *testing.T) {
	tests := []struct {
		name       string
		expiresAt  time.Time
		revokedAt  pgtype.Timestamptz
		acceptedAt pgtype.Timestamptz
		wantCode   string
	}{
		{name: "expired", expiresAt: time.Now().Add(-time.Minute), wantCode: "INVITE_EXPIRED"},
		{name: "revoked", expiresAt: time.Now().Add(time.Hour), revokedAt: timestampNow(), wantCode: "INVITE_REVOKED"},
		{name: "already accepted", expiresAt: time.Now().Add(time.Hour), acceptedAt: timestampNow(), wantCode: "INVITE_ALREADY_ACCEPTED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &membersTestDB{
				inviteExpiresAt:  tt.expiresAt,
				inviteRevokedAt:  tt.revokedAt,
				inviteAcceptedAt: tt.acceptedAt,
			}
			h := NewMembersHandler(db.New(store), nil, nil, "")
			req := httptest.NewRequest(http.MethodPost, "/v1/invites/token_1/accept", nil)
			req = withChiParam(req, "token", "token_1")
			req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_editor"))
			rec := httptest.NewRecorder()

			h.AcceptInvite(rec, req)

			if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), tt.wantCode) {
				t.Fatalf("status=%d body=%s, want 410 %s", rec.Code, rec.Body.String(), tt.wantCode)
			}
		})
	}
}

func TestMembersChangeRoleProtectsOwner(t *testing.T) {
	store := &membersTestDB{membershipRole: auth.RoleOwner, membershipStatus: "active"}
	h := NewMembersHandler(db.New(store), nil, nil, "")
	req := httptest.NewRequest(http.MethodPatch, "/v1/members/user_owner/role", strings.NewReader(`{"role":"editor"}`))
	req = withChiParam(req, "userID", "user_owner")
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.ChangeRole(rec, req)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "OWNER_ROLE_PROTECTED") {
		t.Fatalf("status=%d body=%s, want 403 OWNER_ROLE_PROTECTED", rec.Code, rec.Body.String())
	}
	if store.updateRoleCalls != 0 {
		t.Fatalf("role update calls=%d, want 0", store.updateRoleCalls)
	}
}

func TestMembersChangeRoleRejectsSelfDemotion(t *testing.T) {
	store := &membersTestDB{membershipRole: auth.RoleAdmin, membershipStatus: "active"}
	h := NewMembersHandler(db.New(store), nil, nil, "")
	req := httptest.NewRequest(http.MethodPatch, "/v1/members/user_admin/role", strings.NewReader(`{"role":"editor"}`))
	req = withChiParam(req, "userID", "user_admin")
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_admin"))
	rec := httptest.NewRecorder()

	h.ChangeRole(rec, req)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "CANNOT_CHANGE_OWN_ROLE") {
		t.Fatalf("status=%d body=%s, want 403 CANNOT_CHANGE_OWN_ROLE", rec.Code, rec.Body.String())
	}
	if store.updateRoleCalls != 0 {
		t.Fatalf("role update calls=%d, want 0", store.updateRoleCalls)
	}
}

func TestMembersRemoveProtectsSelfAndOwner(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		store := &membersTestDB{}
		h := NewMembersHandler(db.New(store), nil, nil, "")
		req := httptest.NewRequest(http.MethodDelete, "/v1/members/user_admin", nil)
		req = withChiParam(req, "userID", "user_admin")
		req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
		req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_admin"))
		rec := httptest.NewRecorder()

		h.Remove(rec, req)

		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "CANNOT_REMOVE_SELF") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("owner", func(t *testing.T) {
		store := &membersTestDB{membershipRole: auth.RoleOwner, membershipStatus: "active"}
		h := NewMembersHandler(db.New(store), nil, nil, "")
		req := httptest.NewRequest(http.MethodDelete, "/v1/members/user_owner", nil)
		req = withChiParam(req, "userID", "user_owner")
		req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
		req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_admin"))
		rec := httptest.NewRecorder()

		h.Remove(rec, req)

		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "OWNER_PROTECTED") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if store.deleteMembershipCalls != 0 {
			t.Fatalf("delete membership calls=%d, want 0", store.deleteMembershipCalls)
		}
	})
}

func TestMembersTransferOwnershipUsesSingleAtomicMutation(t *testing.T) {
	store := &membersTestDB{membershipRole: auth.RoleAdmin, membershipStatus: "active"}
	h := NewMembersHandler(db.New(store), nil, nil, "")
	req := httptest.NewRequest(http.MethodPost, "/v1/members/user_admin/transfer-ownership", nil)
	req = withChiParam(req, "userID", "user_admin")
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_owner"))
	rec := httptest.NewRecorder()

	h.TransferOwnership(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", rec.Code, rec.Body.String())
	}
	if store.atomicTransferCalls != 1 || store.demoteOwnerCalls != 0 || store.promoteOwnerCalls != 0 {
		t.Fatalf("atomic=%d demote=%d promote=%d, want 1/0/0", store.atomicTransferCalls, store.demoteOwnerCalls, store.promoteOwnerCalls)
	}
}

func requireSingleMembersAuditWrite(t *testing.T, store *membersTestDB) []any {
	t.Helper()
	if len(store.auditWrites) != 1 {
		t.Fatalf("audit writes=%d, want 1", len(store.auditWrites))
	}
	return store.auditWrites[0]
}

func assertMembersAuditIdentity(t *testing.T, write []any, action, resourceType, resourceID string) {
	t.Helper()
	if len(write) != 12 {
		t.Fatalf("audit argument count=%d, want 12", len(write))
	}
	if write[0] != "ws_1" || write[3] != action || write[4] != resourceType || write[6] != audit.CategoryMembership {
		t.Fatalf("unexpected audit identity: %#v", write)
	}
	gotResourceID, ok := write[5].(pgtype.Text)
	if !ok || !gotResourceID.Valid || gotResourceID.String != resourceID {
		t.Fatalf("audit resource_id=%#v, want %q", write[5], resourceID)
	}
}

type membersTestDB struct {
	planID                string
	activeMembers         int32
	maxMembers            int32
	membershipRole        string
	membershipStatus      string
	membershipErr         error
	inviteExpiresAt       time.Time
	inviteRevokedAt       pgtype.Timestamptz
	inviteAcceptedAt      pgtype.Timestamptz
	countMemberCalls      int
	createInviteCalls     int
	updateRoleCalls       int
	deleteMembershipCalls int
	demoteOwnerCalls      int
	promoteOwnerCalls     int
	atomicTransferCalls   int
	lastInviteToken       string
	auditWrites           [][]any
}

func (f *membersTestDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: WriteAuditLog"):
		f.auditWrites = append(f.auditWrites, append([]any(nil), args...))
	case strings.Contains(query, "-- name: DeleteMembership"):
		f.deleteMembershipCalls++
	case strings.Contains(query, "-- name: DemoteCurrentOwner"):
		f.demoteOwnerCalls++
	case strings.Contains(query, "-- name: PromoteToOwner"):
		f.promoteOwnerCalls++
	}
	return pgconn.CommandTag{}, nil
}

func (f *membersTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "-- name: ListPendingInvitesByWorkspace") {
		return emptyScheduledIdempotencyRows{}, nil
	}
	return nil, errors.New("unexpected Query")
}

func (f *membersTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return subscriptionScanRow(f.planID)
	case strings.Contains(query, "-- name: GetPlan"):
		return planScanRow(&freePlanLimitsTestDB{planID: f.planID, freePlanMaxMembers: f.maxMembers})
	case strings.Contains(query, "-- name: CountActiveMembersByWorkspace"):
		f.countMemberCalls++
		return scanRow{values: []any{f.activeMembers}}
	case strings.Contains(query, "-- name: GetPendingInviteByWorkspaceAndEmail"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: CreateInvite"):
		f.createInviteCalls++
		f.lastInviteToken, _ = args[3].(string)
		expiresAt, _ := args[5].(pgtype.Timestamptz)
		return inviteScanRow("invite_1", "ws_1", args[1].(string), args[2].(string), f.lastInviteToken, args[4].(string), expiresAt, pgtype.Timestamptz{}, pgtype.Timestamptz{})
	case strings.Contains(query, "-- name: GetWorkspace"):
		return workspaceScanRow()
	case strings.Contains(query, "-- name: GetInviteByToken"):
		expiresAt := f.inviteExpiresAt
		if expiresAt.IsZero() {
			expiresAt = time.Now().Add(time.Hour)
		}
		return inviteScanRow("invite_1", "ws_1", "operator@release-lab.dev", auth.RoleEditor, "token_1", "user_owner", pgtype.Timestamptz{Time: expiresAt, Valid: true}, f.inviteAcceptedAt, f.inviteRevokedAt)
	case strings.Contains(query, "-- name: GetMembership"):
		if f.membershipErr != nil {
			return scanRow{err: f.membershipErr}
		}
		return membershipScanRow("ws_1", args[1].(string), f.membershipRole, f.membershipStatus)
	case strings.Contains(query, "-- name: UpdateMemberRole"):
		f.updateRoleCalls++
		return membershipScanRow("ws_1", args[1].(string), args[2].(string), "active")
	case strings.Contains(query, "-- name: TransferWorkspaceOwnership"):
		f.atomicTransferCalls++
		return membershipScanRow("ws_1", args[1].(string), auth.RoleOwner, "active")
	default:
		return scanRow{err: errors.New("unexpected QueryRow")}
	}
}

func inviteScanRow(id, workspaceID, email, role, token, invitedBy string, expiresAt, acceptedAt, revokedAt pgtype.Timestamptz) scanRow {
	return scanRow{values: []any{
		id,
		workspaceID,
		email,
		role,
		token,
		invitedBy,
		expiresAt,
		acceptedAt,
		revokedAt,
		timestampNow(),
	}}
}

func membershipScanRow(workspaceID, userID, role, status string) scanRow {
	now := timestampNow()
	return scanRow{values: []any{
		workspaceID,
		userID,
		role,
		status,
		pgtype.Text{},
		now,
		now,
		now,
		now,
	}}
}

func timestampNow() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now(), Valid: true}
}
