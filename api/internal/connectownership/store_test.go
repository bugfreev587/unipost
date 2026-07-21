package connectownership

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestDecideOwnership(t *testing.T) {
	tests := []struct {
		name           string
		matches        []db.SocialAccount
		profileID      string
		externalUserID string
		want           Decision
	}{
		{
			name:           "no match creates",
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Create},
		},
		{
			name: "same profile and same nonempty managed user reconnects",
			matches: []db.SocialAccount{{
				ID:             "account-a",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Reconnect, AccountID: "account-a"},
		},
		{
			name: "different managed user conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-b",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "managed-b", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "owner BYO null ownership conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-owner",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "empty stored managed ownership conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-empty",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "same managed user in a different profile conflicts",
			matches: []db.SocialAccount{{
				ID:             "account-a",
				ProfileID:      "profile-other",
				ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
			}},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
		{
			name: "multiple active matches conflict even when one is an exact reconnect",
			matches: []db.SocialAccount{
				{
					ID:             "account-a",
					ProfileID:      "profile-a",
					ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
				},
				{
					ID:             "account-b",
					ProfileID:      "profile-b",
					ExternalUserID: pgtype.Text{String: "managed-b", Valid: true},
				},
			},
			profileID:      "profile-a",
			externalUserID: "managed-a",
			want:           Decision{Kind: Conflict},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := decide(test.matches, test.profileID, test.externalUserID)
			if got != test.want {
				t.Fatalf("decide() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestConnectOwnershipQuery(t *testing.T) {
	source, err := os.ReadFile("../db/queries/social_accounts.sql")
	if err != nil {
		t.Fatalf("read social account queries: %v", err)
	}

	checkQuery := extractSQLQuery(t, string(source), "CheckActiveAccountsByWorkspaceProviderIdentity")
	checkCompact := strings.Join(strings.Fields(strings.ToLower(checkQuery)), " ")
	if strings.Contains(checkCompact, "for update") {
		t.Fatalf("read-only Check ownership query must not lock rows: %s", checkCompact)
	}
	checkContract := strings.ReplaceAll(checkCompact, "::text", "")
	for _, want := range []string{
		"join profiles p on p.id = sa.profile_id",
		"p.workspace_id = @workspace_id",
		"sa.platform = @platform",
		"sa.status = 'active'",
		"sa.disconnected_at is null",
		"@platform = 'instagram' and sa.metadata->>'instagram_webhook_user_id' = @provider_identity",
		"@platform <> 'instagram' and sa.external_account_id = @provider_identity",
	} {
		if !strings.Contains(checkContract, want) {
			t.Errorf("read-only Check ownership query missing %q: %s", want, checkCompact)
		}
	}

	saveQuery := extractSQLQuery(t, string(source), "ListActiveAccountsByWorkspaceProviderIdentity")
	compact := strings.Join(strings.Fields(strings.ToLower(saveQuery)), " ")
	contract := strings.ReplaceAll(compact, "::text", "")

	for _, want := range []string{
		"select sa.id, sa.profile_id, sa.platform, sa.access_token",
		"sa.connection_type, sa.connect_session_id, sa.external_user_id, sa.external_user_email",
		"sa.last_refreshed_at, sa.x_app_mode from social_accounts sa",
		"join profiles p on p.id = sa.profile_id",
		"p.workspace_id = @workspace_id",
		"sa.platform = @platform",
		"sa.status = 'active'",
		"sa.disconnected_at is null",
		"@platform = 'instagram' and sa.metadata->>'instagram_webhook_user_id' = @provider_identity",
		"@platform <> 'instagram' and sa.external_account_id = @provider_identity",
		"order by sa.connected_at desc, sa.id",
		"for update of sa",
	} {
		if !strings.Contains(contract, want) {
			t.Errorf("ownership query missing %q: %s", want, compact)
		}
	}

	for _, forbidden := range []string{
		"select sa.*",
		"sa.profile_id =",
		"sa.connection_type =",
		"limit ",
	} {
		if strings.Contains(compact, forbidden) {
			t.Errorf("ownership query must include every workspace profile and both managed/BYO rows; found %q in %s", forbidden, compact)
		}
	}

	generated, err := os.ReadFile("../db/social_accounts.sql.go")
	if err != nil {
		t.Fatalf("read generated social account queries: %v", err)
	}
	paramsPattern := regexp.MustCompile(`(?s)type ListActiveAccountsByWorkspaceProviderIdentityParams struct \{.*?ProviderIdentity\s+string\s+`)
	if !paramsPattern.Match(generated) {
		t.Fatal("generated ownership query must accept provider identity as text")
	}
	checkParamsPattern := regexp.MustCompile(`(?s)type CheckActiveAccountsByWorkspaceProviderIdentityParams struct \{.*?ProviderIdentity\s+string\s+`)
	if !checkParamsPattern.Match(generated) || !strings.Contains(string(generated), "func (q *Queries) CheckActiveAccountsByWorkspaceProviderIdentity") {
		t.Fatal("generated read-only Check ownership query is missing or has non-text provider identity")
	}
}

func TestStoreCheckIsReadOnlyClassification(t *testing.T) {
	queries := &fakeOwnershipQueries{
		matches: []db.SocialAccount{{
			ID:             "account-a",
			ProfileID:      "profile-a",
			ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
		}},
	}
	store := &Store{queries: queries}

	decision, err := store.Check(context.Background(), OwnershipKey{
		WorkspaceID:      "workspace-a",
		ProfileID:        "profile-a",
		Platform:         "instagram",
		ProviderIdentity: "provider-a",
		ExternalUserID:   "managed-a",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if decision != (Decision{Kind: Reconnect, AccountID: "account-a"}) {
		t.Fatalf("Check() decision = %+v", decision)
	}
	if queries.checkLookupCalls != 1 || queries.lockingLookupCalls != 0 || queries.mutationCalls != 0 {
		t.Fatalf("check/locking/mutation calls = %d/%d/%d", queries.checkLookupCalls, queries.lockingLookupCalls, queries.mutationCalls)
	}
	if got := queries.lookupParams; got.WorkspaceID != "workspace-a" || got.Platform != "instagram" || got.ProviderIdentity != "provider-a" {
		t.Fatalf("lookup params = %+v", got)
	}
}

func TestStoreSaveRepeatsLookupUnderAdvisoryLock(t *testing.T) {
	var events []string
	checkQueries := &fakeOwnershipQueries{events: &events}
	authoritativeQueries := &fakeOwnershipQueries{
		events: &events,
		matches: []db.SocialAccount{{
			ID:             "account-b",
			ProfileID:      "profile-a",
			ExternalUserID: pgtype.Text{String: "managed-b", Valid: true},
		}},
	}
	tx := &fakeOwnershipTx{events: &events}
	store := &Store{
		queries: checkQueries,
		beginTx: func(context.Context) (ownershipTx, error) {
			events = append(events, "begin")
			return tx, nil
		},
		queriesFor: func(db.DBTX) ownershipSaveQueries { return authoritativeQueries },
	}

	decision, err := store.Check(context.Background(), OwnershipKey{
		WorkspaceID:      "workspace-a",
		ProfileID:        "profile-a",
		Platform:         "facebook",
		ProviderIdentity: "provider-a",
		ExternalUserID:   "managed-a",
	})
	if err != nil || decision.Kind != Create {
		t.Fatalf("Check() = %+v, %v", decision, err)
	}

	_, err = store.Save(context.Background(), SaveRequest{
		WorkspaceID:      "workspace-a",
		ProfileID:        "profile-a",
		Platform:         "facebook",
		ProviderIdentity: "provider-a",
		ExternalUserID:   "managed-a",
	})
	if !errors.Is(err, ErrOwnershipConflict) {
		t.Fatalf("Save() error = %v, want ownership conflict", err)
	}
	if err.Error() != "ACCOUNT_OWNERSHIP_CONFLICT" {
		t.Fatalf("conflict error leaked details: %q", err.Error())
	}
	var conflict *OwnershipConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Save() error type = %T, want *OwnershipConflictError", err)
	}
	if authoritativeQueries.mutationCalls != 0 || tx.commitCalls != 0 {
		t.Fatalf("mutation calls = %d, commit calls = %d", authoritativeQueries.mutationCalls, tx.commitCalls)
	}
	wantEvents := []string{"check_lookup", "begin", "lock", "lookup", "rollback"}
	if strings.Join(events, ",") != strings.Join(wantEvents, ",") {
		t.Fatalf("events = %v, want %v", events, wantEvents)
	}
	if tx.lockSQL != "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))" {
		t.Fatalf("lock SQL = %q", tx.lockSQL)
	}
	if len(tx.lockArgs) != 1 || tx.lockArgs[0] != "11:776f726b73706163652d61;8:66616365626f6f6b;10:70726f76696465722d61;" {
		t.Fatalf("lock args = %#v", tx.lockArgs)
	}
	if strings.Contains(tx.lockArgs[0].(string), "\x00") {
		t.Fatalf("PostgreSQL text lock argument contains NUL: %#v", tx.lockArgs[0])
	}
	if got := authoritativeQueries.lookupParams; got.WorkspaceID != "workspace-a" || got.Platform != "facebook" || got.ProviderIdentity != "provider-a" {
		t.Fatalf("authoritative lookup params = %+v", got)
	}
}

func TestConnectOwnershipLockKeyIsNULFreeAndUnambiguous(t *testing.T) {
	keys := []string{
		connectOwnershipLockKey("a", "b\x00c", "d"),
		connectOwnershipLockKey("a\x00b", "c", "d"),
		connectOwnershipLockKey("a:1", "b;2", "c"),
		connectOwnershipLockKey("a", "1:b", "2;c"),
	}

	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if strings.Contains(key, "\x00") {
			t.Fatalf("lock key contains NUL: %q", key)
		}
		if _, exists := seen[key]; exists {
			t.Fatalf("distinct ownership tuples produced duplicate lock key %q", key)
		}
		seen[key] = struct{}{}
	}
}

func TestStoreSaveAppliesOnlyCreateOrReconnect(t *testing.T) {
	tests := []struct {
		name             string
		platform         string
		matches          []db.SocialAccount
		wantAccountID    string
		wantRefreshCalls int
		wantUpsertCalls  int
		wantCreateCalls  int
	}{
		{
			name:     "reconnect refreshes the exact account",
			platform: "instagram",
			matches: []db.SocialAccount{{
				ID:             "existing-a",
				ProfileID:      "profile-a",
				ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
			}},
			wantAccountID:    "existing-a",
			wantRefreshCalls: 1,
		},
		{
			name:            "non-Bluesky create uses managed upsert",
			platform:        "threads",
			wantAccountID:   "upserted-a",
			wantUpsertCalls: 1,
		},
		{
			name:            "Bluesky create uses insert",
			platform:        "bluesky",
			wantAccountID:   "created-a",
			wantCreateCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &fakeOwnershipTx{}
			queries := &fakeOwnershipQueries{
				matches:       test.matches,
				refreshResult: db.SocialAccount{ID: "existing-a"},
				upsertResult:  db.SocialAccount{ID: "upserted-a"},
				createResult:  db.SocialAccount{ID: "created-a"},
			}
			store := &Store{
				beginTx:    func(context.Context) (ownershipTx, error) { return tx, nil },
				queriesFor: func(db.DBTX) ownershipSaveQueries { return queries },
			}

			account, err := store.Save(context.Background(), SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         test.platform,
				ProviderIdentity: "provider-a",
				ExternalUserID:   "managed-a",
				Refresh:          db.RefreshConnectedSocialAccountParams{ID: "caller-controlled-id"},
			})
			if err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			if account.ID != test.wantAccountID {
				t.Fatalf("Save() account ID = %q, want %q", account.ID, test.wantAccountID)
			}
			if queries.refreshCalls != test.wantRefreshCalls || queries.upsertCalls != test.wantUpsertCalls || queries.createCalls != test.wantCreateCalls {
				t.Fatalf("refresh/upsert/create calls = %d/%d/%d", queries.refreshCalls, queries.upsertCalls, queries.createCalls)
			}
			if test.wantRefreshCalls == 1 && queries.refreshParams.ID != "existing-a" {
				t.Fatalf("refresh account ID = %q, want DB-derived existing-a", queries.refreshParams.ID)
			}
			if test.wantRefreshCalls == 1 {
				if queries.refreshParams.ConnectionType != "managed" || queries.refreshParams.ExternalUserID != (pgtype.Text{String: "managed-a", Valid: true}) {
					t.Fatalf("normalized refresh identity = %+v", queries.refreshParams)
				}
				assertInstagramWebhookIdentity(t, queries.refreshParams.Metadata, "provider-a")
			}
			if test.wantUpsertCalls == 1 {
				assertManagedCreateIdentity(t, queries.upsertParams.ProfileID, queries.upsertParams.Platform, queries.upsertParams.ExternalAccountID, queries.upsertParams.ExternalUserID, "threads")
			}
			if test.wantCreateCalls == 1 {
				assertManagedCreateIdentity(t, queries.createParams.ProfileID, queries.createParams.Platform, queries.createParams.ExternalAccountID, queries.createParams.ExternalUserID, "bluesky")
			}
			if tx.commitCalls != 1 {
				t.Fatalf("commit calls = %d, want 1", tx.commitCalls)
			}
		})
	}
}

func TestStoreSaveRejectsNestedIdentityMismatchBeforeTransaction(t *testing.T) {
	tests := []struct {
		name    string
		request SaveRequest
	}{
		{
			name: "Refresh cannot target a different Instagram webhook identity",
			request: SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         "instagram",
				ProviderIdentity: "provider-a",
				ExternalUserID:   "managed-a",
				Refresh: db.RefreshConnectedSocialAccountParams{
					Metadata:       []byte(`{"instagram_webhook_user_id":"provider-b"}`),
					ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
				},
			},
		},
		{
			name: "Upsert cannot target a different external account",
			request: SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         "threads",
				ProviderIdentity: "provider-a",
				ExternalUserID:   "managed-a",
				Upsert: db.UpsertManagedSocialAccountParams{
					ProfileID:         "profile-a",
					Platform:          "threads",
					ExternalAccountID: "provider-b",
					ExternalUserID:    pgtype.Text{String: "managed-a", Valid: true},
				},
			},
		},
		{
			name: "Create cannot target a different managed user",
			request: SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         "bluesky",
				ProviderIdentity: "provider-a",
				ExternalUserID:   "managed-a",
				Create: db.CreateManagedSocialAccountParams{
					ProfileID:         "profile-a",
					Platform:          "bluesky",
					ExternalAccountID: "provider-a",
					ExternalUserID:    pgtype.Text{String: "managed-b", Valid: true},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beginCalls := 0
			queries := &fakeOwnershipQueries{}
			store := &Store{
				beginTx: func(context.Context) (ownershipTx, error) {
					beginCalls++
					return &fakeOwnershipTx{}, nil
				},
				queriesFor: func(db.DBTX) ownershipSaveQueries { return queries },
			}

			_, err := store.Save(context.Background(), test.request)
			if !errors.Is(err, ErrInvalidOwnershipRequest) {
				t.Fatalf("Save() error = %v, want invalid ownership request", err)
			}
			if err.Error() != "INVALID_ACCOUNT_OWNERSHIP_REQUEST" {
				t.Fatalf("invalid request error leaked details: %q", err.Error())
			}
			if beginCalls != 0 || queries.mutationCalls != 0 {
				t.Fatalf("begin calls = %d, mutation calls = %d; mismatch must be rejected before transaction", beginCalls, queries.mutationCalls)
			}
		})
	}
}

func TestStoreSaveRejectsEmptyCanonicalIdentityBeforeTransaction(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SaveRequest)
	}{
		{name: "workspace", mutate: func(request *SaveRequest) { request.WorkspaceID = "" }},
		{name: "profile", mutate: func(request *SaveRequest) { request.ProfileID = "" }},
		{name: "platform", mutate: func(request *SaveRequest) { request.Platform = "" }},
		{name: "provider identity", mutate: func(request *SaveRequest) { request.ProviderIdentity = "" }},
		{name: "external user", mutate: func(request *SaveRequest) { request.ExternalUserID = "" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         "threads",
				ProviderIdentity: "provider-a",
				ExternalUserID:   "managed-a",
			}
			test.mutate(&request)
			beginCalls := 0
			store := &Store{beginTx: func(context.Context) (ownershipTx, error) {
				beginCalls++
				return &fakeOwnershipTx{}, nil
			}}

			_, err := store.Save(context.Background(), request)
			if !errors.Is(err, ErrInvalidOwnershipRequest) {
				t.Fatalf("Save() error = %v, want invalid ownership request", err)
			}
			if beginCalls != 0 {
				t.Fatalf("begin calls = %d, want 0", beginCalls)
			}
		})
	}
}

func TestStoreSavePreservesUnrelatedInstagramProviderPayload(t *testing.T) {
	queries := &fakeOwnershipQueries{
		matches: []db.SocialAccount{{
			ID:             "account-a",
			ProfileID:      "profile-a",
			ExternalUserID: pgtype.Text{String: "managed-a", Valid: true},
		}},
		refreshResult: db.SocialAccount{ID: "account-a"},
	}
	store := &Store{
		beginTx:    func(context.Context) (ownershipTx, error) { return &fakeOwnershipTx{}, nil },
		queriesFor: func(db.DBTX) ownershipSaveQueries { return queries },
	}

	_, err := store.Save(context.Background(), SaveRequest{
		WorkspaceID:      "workspace-a",
		ProfileID:        "profile-a",
		Platform:         "instagram",
		ProviderIdentity: "webhook-a",
		ExternalUserID:   "managed-a",
		Refresh: db.RefreshConnectedSocialAccountParams{
			ExternalAccountID: "business-account-a",
			Metadata:          []byte(`{"username":"alice","provider_payload":{"page_id":"page-a"}}`),
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if queries.refreshParams.ExternalAccountID != "business-account-a" {
		t.Fatalf("Instagram external account ID = %q, want preserved business-account-a", queries.refreshParams.ExternalAccountID)
	}
	var metadata map[string]any
	if err := json.Unmarshal(queries.refreshParams.Metadata, &metadata); err != nil {
		t.Fatalf("decode normalized metadata: %v", err)
	}
	if metadata["username"] != "alice" || metadata["instagram_webhook_user_id"] != "webhook-a" {
		t.Fatalf("normalized Instagram metadata = %#v", metadata)
	}
	payload, ok := metadata["provider_payload"].(map[string]any)
	if !ok || payload["page_id"] != "page-a" {
		t.Fatalf("unrelated provider payload was overwritten: %#v", metadata["provider_payload"])
	}
}

func assertManagedCreateIdentity(t *testing.T, profileID, platform, externalAccountID string, externalUserID pgtype.Text, wantPlatform string) {
	t.Helper()
	if profileID != "profile-a" || platform != wantPlatform || externalAccountID != "provider-a" || externalUserID != (pgtype.Text{String: "managed-a", Valid: true}) {
		t.Fatalf("normalized managed identity = profile=%q platform=%q account=%q user=%+v", profileID, platform, externalAccountID, externalUserID)
	}
}

func assertInstagramWebhookIdentity(t *testing.T, metadata []byte, want string) {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(metadata, &object); err != nil {
		t.Fatalf("decode Instagram metadata: %v", err)
	}
	if object["instagram_webhook_user_id"] != want {
		t.Fatalf("Instagram webhook identity = %#v, want %q", object["instagram_webhook_user_id"], want)
	}
}

func TestStoreSaveSerializesConcurrentOwnershipDecisions(t *testing.T) {
	database := &serializedOwnershipDB{}
	store := &Store{
		beginTx: database.begin,
		queriesFor: func(tx db.DBTX) ownershipSaveQueries {
			return &serializedOwnershipQueries{tx: tx.(*serializedOwnershipTx)}
		},
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, externalUserID := range []string{"managed-a", "managed-b"} {
		externalUserID := externalUserID
		go func() {
			<-start
			_, err := store.Save(context.Background(), SaveRequest{
				WorkspaceID:      "workspace-a",
				ProfileID:        "profile-a",
				Platform:         "threads",
				ProviderIdentity: "provider-a",
				ExternalUserID:   externalUserID,
				Upsert: db.UpsertManagedSocialAccountParams{
					ProfileID:      "profile-a",
					Platform:       "threads",
					ExternalUserID: pgtype.Text{String: externalUserID, Valid: true},
				},
			})
			results <- err
		}()
	}
	close(start)

	var successes, conflicts int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrOwnershipConflict):
			conflicts++
		default:
			t.Fatalf("unexpected Save() error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes/conflicts = %d/%d, want 1/1", successes, conflicts)
	}
	database.mu.Lock()
	defer database.mu.Unlock()
	if database.upsertCalls != 1 || len(database.accounts) != 1 {
		t.Fatalf("upsert calls = %d, accounts = %d", database.upsertCalls, len(database.accounts))
	}
}

type fakeOwnershipQueries struct {
	events             *[]string
	matches            []db.SocialAccount
	lookupParams       db.ListActiveAccountsByWorkspaceProviderIdentityParams
	checkLookupCalls   int
	lockingLookupCalls int
	mutationCalls      int
	refreshCalls       int
	upsertCalls        int
	createCalls        int
	refreshParams      db.RefreshConnectedSocialAccountParams
	upsertParams       db.UpsertManagedSocialAccountParams
	createParams       db.CreateManagedSocialAccountParams
	refreshResult      db.SocialAccount
	upsertResult       db.SocialAccount
	createResult       db.SocialAccount
}

func (f *fakeOwnershipQueries) CheckActiveAccountsByWorkspaceProviderIdentity(_ context.Context, params db.CheckActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error) {
	f.checkLookupCalls++
	f.lookupParams = db.ListActiveAccountsByWorkspaceProviderIdentityParams{
		WorkspaceID:      params.WorkspaceID,
		Platform:         params.Platform,
		ProviderIdentity: params.ProviderIdentity,
	}
	f.record("check_lookup")
	return f.matches, nil
}

func (f *fakeOwnershipQueries) ListActiveAccountsByWorkspaceProviderIdentity(_ context.Context, params db.ListActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error) {
	f.lockingLookupCalls++
	f.lookupParams = params
	f.record("lookup")
	return f.matches, nil
}

func (f *fakeOwnershipQueries) RefreshConnectedSocialAccount(_ context.Context, params db.RefreshConnectedSocialAccountParams) (db.SocialAccount, error) {
	f.mutationCalls++
	f.refreshCalls++
	f.refreshParams = params
	f.record("refresh")
	return f.refreshResult, nil
}

func (f *fakeOwnershipQueries) UpsertManagedSocialAccount(_ context.Context, params db.UpsertManagedSocialAccountParams) (db.SocialAccount, error) {
	f.mutationCalls++
	f.upsertCalls++
	f.upsertParams = params
	f.record("upsert")
	return f.upsertResult, nil
}

func (f *fakeOwnershipQueries) CreateManagedSocialAccount(_ context.Context, params db.CreateManagedSocialAccountParams) (db.SocialAccount, error) {
	f.mutationCalls++
	f.createCalls++
	f.createParams = params
	f.record("create")
	return f.createResult, nil
}

func (f *fakeOwnershipQueries) record(event string) {
	if f.events != nil {
		*f.events = append(*f.events, event)
	}
}

type fakeOwnershipTx struct {
	events      *[]string
	lockSQL     string
	lockArgs    []any
	commitCalls int
}

func (f *fakeOwnershipTx) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	f.lockSQL = sql
	f.lockArgs = args
	f.record("lock")
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (*fakeOwnershipTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (*fakeOwnershipTx) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("unexpected QueryRow call")
}

func (f *fakeOwnershipTx) Commit(context.Context) error {
	f.commitCalls++
	f.record("commit")
	return nil
}

func (f *fakeOwnershipTx) Rollback(context.Context) error {
	f.record("rollback")
	return nil
}

func (f *fakeOwnershipTx) record(event string) {
	if f.events != nil {
		*f.events = append(*f.events, event)
	}
}

type serializedOwnershipDB struct {
	mu          sync.Mutex
	accounts    []db.SocialAccount
	upsertCalls int
}

func (d *serializedOwnershipDB) begin(context.Context) (ownershipTx, error) {
	return &serializedOwnershipTx{database: d}, nil
}

type serializedOwnershipTx struct {
	database *serializedOwnershipDB
	locked   bool
}

func (tx *serializedOwnershipTx) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	tx.database.mu.Lock()
	tx.locked = true
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (*serializedOwnershipTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (*serializedOwnershipTx) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("unexpected QueryRow call")
}

func (tx *serializedOwnershipTx) Commit(context.Context) error {
	tx.unlock()
	return nil
}

func (tx *serializedOwnershipTx) Rollback(context.Context) error {
	tx.unlock()
	return nil
}

func (tx *serializedOwnershipTx) unlock() {
	if tx.locked {
		tx.locked = false
		tx.database.mu.Unlock()
	}
}

type serializedOwnershipQueries struct {
	tx *serializedOwnershipTx
}

func (q *serializedOwnershipQueries) ListActiveAccountsByWorkspaceProviderIdentity(context.Context, db.ListActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error) {
	return append([]db.SocialAccount(nil), q.tx.database.accounts...), nil
}

func (*serializedOwnershipQueries) RefreshConnectedSocialAccount(context.Context, db.RefreshConnectedSocialAccountParams) (db.SocialAccount, error) {
	panic("unexpected refresh")
}

func (q *serializedOwnershipQueries) UpsertManagedSocialAccount(_ context.Context, params db.UpsertManagedSocialAccountParams) (db.SocialAccount, error) {
	account := db.SocialAccount{
		ID:             "created-account",
		ProfileID:      params.ProfileID,
		Platform:       params.Platform,
		ExternalUserID: params.ExternalUserID,
	}
	q.tx.database.accounts = append(q.tx.database.accounts, account)
	q.tx.database.upsertCalls++
	return account, nil
}

func (*serializedOwnershipQueries) CreateManagedSocialAccount(context.Context, db.CreateManagedSocialAccountParams) (db.SocialAccount, error) {
	panic("unexpected create")
}

func extractSQLQuery(t *testing.T, source, name string) string {
	t.Helper()

	pattern := regexp.MustCompile(`(?ms)^-- name: ` + regexp.QuoteMeta(name) + ` [^\n]*\n(.*?)(?:^-- name: |\z)`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("query %s is missing", name)
	}
	return match[1]
}
