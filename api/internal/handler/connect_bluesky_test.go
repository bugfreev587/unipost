package handler

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/connectownership"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// TestIPLimiter — basic burst + window behavior.
func TestIPLimiter(t *testing.T) {
	l := newIPLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.1.1.1") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if l.Allow("1.1.1.1") {
		t.Error("4th attempt should be denied")
	}
	// Different IP gets its own bucket.
	if !l.Allow("2.2.2.2") {
		t.Error("different IP should be allowed")
	}
}

// TestIPLimiter_Window — entries older than the window are dropped.
func TestIPLimiter_Window(t *testing.T) {
	l := newIPLimiter(2, 100*time.Millisecond)
	l.Allow("ip")
	l.Allow("ip")
	if l.Allow("ip") {
		t.Fatal("should be at limit")
	}
	time.Sleep(120 * time.Millisecond)
	if !l.Allow("ip") {
		t.Error("after window, attempt should succeed")
	}
}

// TestClientIP — XFF handling, then RemoteAddr fallback.
func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	if got := clientIP(r); got != "10.0.0.5:1234" {
		t.Errorf("no XFF: got %q", got)
	}

	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("single XFF: got %q", got)
	}

	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1, 10.0.0.2")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("XFF chain: got %q", got)
	}
}

// TestBlueskyTemplate_NoPasswordEcho — sanity check that the form
// template never includes the password field's value attribute, even
// if blueskyTplData were to have one. Locks the credential-handling
// invariant via a string check on the template source.
func TestBlueskyTemplate_NoPasswordEcho(t *testing.T) {
	for _, line := range strings.Split(blueskyResultTplSrc, "\n") {
		if strings.Contains(line, `name="app_password"`) {
			if strings.Contains(line, "value=") {
				t.Errorf("password input should never carry a value attribute: %q", line)
			}
		}
	}
}

func TestConnectBlueskyUsesVerifiedDIDThroughOwnershipStore(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	fdb := &connectSessionTestDB{platform: "bluesky", allowQuickstart: true}
	store := &fakeManagedOwnershipStore{
		checkDecision: connectownership.Decision{Kind: connectownership.Create},
		saveAccount:   db.SocialAccount{ID: "sa_bluesky_1"},
	}
	bus := &recordingConnectBus{}
	h := NewConnectBlueskyHandler(db.New(fdb), encryptor, bus, store)
	h.connectAccount = func(context.Context, map[string]string) (*platform.ConnectResult, error) {
		return &platform.ConnectResult{
			AccessToken:       "access-jwt",
			RefreshToken:      "refresh-jwt",
			ExternalAccountID: "  did:plc:verified  ",
			AccountName:       "robyn.bsky.social",
			Metadata:          map[string]any{"did": "did:plc:verified", "handle": "robyn.bsky.social"},
		}, nil
	}
	req := blueskySubmitRequest("cs_1", "state_1")
	rec := httptest.NewRecorder()

	h.SubmitForm(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.checkCalls != 1 || store.saveCalls != 1 {
		t.Fatalf("ownership check/save calls = %d/%d", store.checkCalls, store.saveCalls)
	}
	if store.checkKey != (connectownership.OwnershipKey{
		WorkspaceID: "ws_1", ProfileID: "pr_1", Platform: "bluesky",
		ProviderIdentity: "did:plc:verified", ExternalUserID: "user_123",
	}) {
		t.Fatalf("ownership key = %+v", store.checkKey)
	}
	if store.saveRequest.ProviderIdentity != "did:plc:verified" || store.saveRequest.Create.ExternalAccountID != "did:plc:verified" {
		t.Fatalf("ownership save request = %+v", store.saveRequest)
	}
	if fdb.refreshManagedCalls != 0 || fdb.createManagedCalls != 0 || fdb.upsertManagedCalls != 0 {
		t.Fatalf("legacy save calls = refresh %d/create %d/upsert %d", fdb.refreshManagedCalls, fdb.createManagedCalls, fdb.upsertManagedCalls)
	}
	if fdb.completedAcctID != "sa_bluesky_1" || bus.calls != 1 {
		t.Fatalf("completed account/bus calls = %q/%d", fdb.completedAcctID, bus.calls)
	}
}

func TestConnectBlueskyOwnershipConflictsHaveZeroDownstreamSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name          string
		checkDecision connectownership.Decision
		saveErr       error
		wantSaveCalls int
	}{
		{
			name: "early BYO conflict",
			checkDecision: connectownership.Decision{
				Kind: connectownership.Conflict, ConflictClass: connectownership.ConflictOwnerBYO, MatchCount: 1,
			},
		},
		{
			name: "early managed user mismatch",
			checkDecision: connectownership.Decision{
				Kind: connectownership.Conflict, ConflictClass: connectownership.ConflictManagedUserMismatch, MatchCount: 1,
			},
		},
		{
			name: "early cross profile conflict",
			checkDecision: connectownership.Decision{
				Kind: connectownership.Conflict, ConflictClass: connectownership.ConflictProfileMismatch, MatchCount: 1,
			},
		},
		{
			name: "early ambiguous matches",
			checkDecision: connectownership.Decision{
				Kind: connectownership.Conflict, ConflictClass: connectownership.ConflictAmbiguousMatches, MatchCount: 2,
			},
		},
		{
			name:          "late authoritative conflict",
			checkDecision: connectownership.Decision{Kind: connectownership.Create},
			saveErr:       connectownership.ErrOwnershipConflict,
			wantSaveCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
			if err != nil {
				t.Fatal(err)
			}
			fdb := &connectSessionTestDB{platform: "bluesky", allowQuickstart: true}
			store := &fakeManagedOwnershipStore{checkDecision: tc.checkDecision, saveErr: tc.saveErr}
			bus := &recordingConnectBus{}
			h := NewConnectBlueskyHandler(db.New(fdb), encryptor, bus, store)
			h.connectAccount = func(context.Context, map[string]string) (*platform.ConnectResult, error) {
				return &platform.ConnectResult{
					AccessToken: "access-secret", RefreshToken: "refresh-secret",
					ExternalAccountID: "did:plc:provider-secret", AccountName: "robyn.bsky.social",
				}, nil
			}
			rec := httptest.NewRecorder()

			h.SubmitForm(rec, blueskySubmitRequest("cs_1", "state_1"))

			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if store.saveCalls != tc.wantSaveCalls || bus.calls != 0 || fdb.completedAcctID != "" {
				t.Fatalf("save/publish/completed = %d/%d/%q", store.saveCalls, bus.calls, fdb.completedAcctID)
			}
			if fdb.refreshManagedCalls != 0 || fdb.createManagedCalls != 0 || fdb.upsertManagedCalls != 0 {
				t.Fatalf("legacy writes = refresh %d/create %d/upsert %d", fdb.refreshManagedCalls, fdb.createManagedCalls, fdb.upsertManagedCalls)
			}
			for _, secret := range []string{"provider-secret", "access-secret", "refresh-secret", "user_123"} {
				if strings.Contains(rec.Body.String(), secret) {
					t.Fatalf("response leaked %q: %s", secret, rec.Body.String())
				}
			}
		})
	}
}

func TestConnectBlueskyAllowsExactSameOwnerReconnect(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	fdb := &connectSessionTestDB{platform: "bluesky", allowQuickstart: true}
	store := &fakeManagedOwnershipStore{
		checkDecision: connectownership.Decision{Kind: connectownership.Reconnect, AccountID: "sa_existing"},
		saveAccount:   db.SocialAccount{ID: "sa_existing"},
	}
	bus := &recordingConnectBus{}
	h := NewConnectBlueskyHandler(db.New(fdb), encryptor, bus, store)
	h.connectAccount = func(context.Context, map[string]string) (*platform.ConnectResult, error) {
		return &platform.ConnectResult{
			AccessToken:       "access-secret",
			RefreshToken:      "refresh-secret",
			ExternalAccountID: "did:plc:verified",
			AccountName:       "robyn.bsky.social",
		}, nil
	}
	rec := httptest.NewRecorder()

	h.SubmitForm(rec, blueskySubmitRequest("cs_1", "state_1"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.checkCalls != 1 || store.saveCalls != 1 || store.saveRequest.ProviderIdentity != "did:plc:verified" {
		t.Fatalf("check/save/provider = %d/%d/%q", store.checkCalls, store.saveCalls, store.saveRequest.ProviderIdentity)
	}
	if fdb.refreshManagedCalls != 0 || fdb.createManagedCalls != 0 || fdb.upsertManagedCalls != 0 {
		t.Fatalf("legacy writes = refresh %d/create %d/upsert %d", fdb.refreshManagedCalls, fdb.createManagedCalls, fdb.upsertManagedCalls)
	}
	if fdb.completedAcctID != "sa_existing" || bus.calls != 1 || bus.workspaceID != "ws_1" {
		t.Fatalf("completed/event/workspace = %q/%d/%q", fdb.completedAcctID, bus.calls, bus.workspaceID)
	}
}

func TestConnectBlueskyManagedSharingUsesVerifiedDIDAndFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name           string
		sharingBlocked bool
		sharingErr     error
		wantStatus     int
	}{
		{name: "cross-workspace violation", sharingBlocked: true, wantStatus: http.StatusConflict},
		{name: "lookup outage", sharingErr: fmt.Errorf("database outage containing did:plc:provider-secret user_123 access-secret"), wantStatus: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })

			encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
			if err != nil {
				t.Fatal(err)
			}
			fdb := &connectSessionTestDB{
				platform:              "bluesky",
				allowQuickstart:       true,
				managedSharingBlocked: tc.sharingBlocked,
				managedSharingErr:     tc.sharingErr,
			}
			store := &fakeManagedOwnershipStore{
				checkDecision: connectownership.Decision{Kind: connectownership.Create},
				saveAccount:   db.SocialAccount{ID: "sa_must_not_save"},
			}
			bus := &recordingConnectBus{}
			h := NewConnectBlueskyHandler(db.New(fdb), encryptor, bus, store)
			h.connectAccount = func(context.Context, map[string]string) (*platform.ConnectResult, error) {
				return &platform.ConnectResult{
					AccessToken:       "access-secret",
					RefreshToken:      "refresh-secret",
					ExternalAccountID: "  did:plc:provider-secret  ",
					AccountName:       "robyn.bsky.social",
				}, nil
			}
			rec := httptest.NewRecorder()

			h.SubmitForm(rec, blueskySubmitRequest("cs_1", "state_1"))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.sharingBlocked && !strings.Contains(rec.Body.String(), accountNotAvailableOnFreePlanMessage) {
				t.Fatalf("sharing violation body = %q", rec.Body.String())
			}
			if fdb.managedSharingCalls != 1 || fdb.managedSharingWorkspaceID != "ws_1" ||
				fdb.managedSharingPlatform != "bluesky" || fdb.managedSharingProviderIdentity != "did:plc:provider-secret" {
				t.Fatalf("sharing calls/scope = %d/%q/%q/%q", fdb.managedSharingCalls, fdb.managedSharingWorkspaceID, fdb.managedSharingPlatform, fdb.managedSharingProviderIdentity)
			}
			if store.saveCalls != 0 || bus.calls != 0 || fdb.completedAcctID != "" {
				t.Fatalf("save/publish/completed = %d/%d/%q", store.saveCalls, bus.calls, fdb.completedAcctID)
			}
			output := logs.String() + rec.Body.String()
			for _, forbidden := range []string{"did:plc:provider-secret", "user_123", "access-secret", "refresh-secret"} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("sharing failure leaked %q: %s", forbidden, output)
				}
			}
		})
	}
}

func TestConnectBlueskyCompletionClaimFailureHasNoSuccessSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name       string
		claimErr   error
		wantStatus int
	}{
		{
			name:       "database error",
			claimErr:   fmt.Errorf("completion failed containing did:plc:provider-secret user_123 access-secret"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "concurrent loser",
			claimErr:   pgx.ErrNoRows,
			wantStatus: http.StatusConflict,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })

			encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
			if err != nil {
				t.Fatal(err)
			}
			fdb := &connectSessionTestDB{platform: "bluesky", allowQuickstart: true, completionClaimErr: tc.claimErr}
			store := &fakeManagedOwnershipStore{
				checkDecision: connectownership.Decision{Kind: connectownership.Create},
				saveAccount:   db.SocialAccount{ID: "sa_saved_before_claim"},
			}
			bus := &recordingConnectBus{}
			h := NewConnectBlueskyHandler(db.New(fdb), encryptor, bus, store)
			h.connectAccount = func(context.Context, map[string]string) (*platform.ConnectResult, error) {
				return &platform.ConnectResult{
					AccessToken:       "access-secret",
					RefreshToken:      "refresh-secret",
					ExternalAccountID: "did:plc:provider-secret",
					AccountName:       "robyn.bsky.social",
				}, nil
			}
			rec := httptest.NewRecorder()

			h.SubmitForm(rec, blueskySubmitRequest("cs_1", "state_1"))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if store.saveCalls != 1 || fdb.completionClaimCalls != 1 {
				t.Fatalf("save/completion calls = %d/%d, want 1/1", store.saveCalls, fdb.completionClaimCalls)
			}
			if bus.calls != 0 || fdb.completedAcctID != "" {
				t.Fatalf("publish/completed = %d/%q, want zero/empty", bus.calls, fdb.completedAcctID)
			}
			output := logs.String() + rec.Body.String()
			for _, forbidden := range []string{"did:plc:provider-secret", "user_123", "access-secret", "refresh-secret"} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("completion failure leaked %q: %s", forbidden, output)
				}
			}
		})
	}
}

func TestConnectBlueskyConcurrentCompletionPublishesOnlyWinner(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	fdb := &connectSessionTestDB{
		platform:                   "bluesky",
		allowQuickstart:            true,
		completionRaceParticipants: 2,
		completionRaceRelease:      make(chan struct{}),
	}
	bus := &recordingConnectBus{}
	recorders := []*httptest.ResponseRecorder{httptest.NewRecorder(), httptest.NewRecorder()}
	stores := []*fakeManagedOwnershipStore{
		{checkDecision: connectownership.Decision{Kind: connectownership.Create}, saveAccount: db.SocialAccount{ID: "sa_concurrent"}},
		{checkDecision: connectownership.Decision{Kind: connectownership.Reconnect, AccountID: "sa_concurrent"}, saveAccount: db.SocialAccount{ID: "sa_concurrent"}},
	}

	var wait sync.WaitGroup
	for index := range recorders {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			h := NewConnectBlueskyHandler(db.New(fdb), encryptor, bus, stores[index])
			h.connectAccount = func(context.Context, map[string]string) (*platform.ConnectResult, error) {
				return &platform.ConnectResult{
					AccessToken:       "access-secret",
					RefreshToken:      "refresh-secret",
					ExternalAccountID: "did:plc:provider-concurrent",
					AccountName:       "robyn.bsky.social",
				}, nil
			}
			h.SubmitForm(recorders[index], blueskySubmitRequest("cs_1", "state_1"))
		}()
	}
	wait.Wait()

	statusCounts := map[int]int{}
	for _, recorder := range recorders {
		statusCounts[recorder.Code]++
	}
	if statusCounts[http.StatusOK] != 1 || statusCounts[http.StatusConflict] != 1 {
		t.Fatalf("status counts = %#v, want one 200 and one 409", statusCounts)
	}
	if bus.calls != 1 || bus.workspaceID != "ws_1" {
		t.Fatalf("event calls/workspace = %d/%q, want 1/ws_1", bus.calls, bus.workspaceID)
	}
	if fdb.completionClaimCalls != 2 || fdb.completedAcctID != "sa_concurrent" {
		t.Fatalf("completion calls/account = %d/%q", fdb.completionClaimCalls, fdb.completedAcctID)
	}
	for index, store := range stores {
		if store.saveCalls != 1 {
			t.Fatalf("store %d save calls = %d, want 1", index, store.saveCalls)
		}
	}
}

func TestConnectBlueskyRejectsMissingVerifiedDIDBeforeOwnershipCheck(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	fdb := &connectSessionTestDB{platform: "bluesky", allowQuickstart: true}
	store := &fakeManagedOwnershipStore{}
	bus := &recordingConnectBus{}
	h := NewConnectBlueskyHandler(db.New(fdb), encryptor, bus, store)
	h.connectAccount = func(context.Context, map[string]string) (*platform.ConnectResult, error) {
		return &platform.ConnectResult{AccessToken: "access-secret", RefreshToken: "refresh-secret"}, nil
	}
	rec := httptest.NewRecorder()

	h.SubmitForm(rec, blueskySubmitRequest("cs_1", "state_1"))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.checkCalls != 0 || store.saveCalls != 0 || bus.calls != 0 || fdb.completedAcctID != "" {
		t.Fatalf("check/save/publish/completed = %d/%d/%d/%q", store.checkCalls, store.saveCalls, bus.calls, fdb.completedAcctID)
	}
}

func blueskySubmitRequest(sessionID, state string) *http.Request {
	body := strings.NewReader("handle=robyn.bsky.social&app_password=app-password")
	req := httptest.NewRequest(http.MethodPost, "/v1/public/connect/sessions/"+sessionID+"/bluesky?state="+state, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
