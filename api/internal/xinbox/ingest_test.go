package xinbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeIngestStore struct {
	accounts     map[string]InboxAccount
	accountCalls int
	provider     map[string][]InboxAccount
	providerErr  error
	providerKeys []string
	inserted     []InboxItem
	insertResult InboxItem
	insertedNew  bool
	insertErr    error
}

func (s *fakeIngestStore) AccountForApp(_ context.Context, appClientID, accountID string) (InboxAccount, error) {
	s.accountCalls++
	account, ok := s.accounts[appClientID+":"+accountID]
	if !ok {
		return InboxAccount{}, ErrInboxAccountNotFound
	}
	return account, nil
}

func TestXIngestTaggedDMRequiresExactProviderUserBeforeSideEffects(t *testing.T) {
	for _, test := range []struct {
		name                  string
		eventProviderUserID   string
		accountProviderUserID string
	}{
		{name: "missing event provider user", accountProviderUserID: "provider-owner"},
		{name: "mismatched event provider user", eventProviderUserID: "provider-other", accountProviderUserID: "provider-owner"},
		{name: "missing account provider user", eventProviderUserID: "provider-owner"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeIngestStore{accounts: map[string]InboxAccount{
				"route-1:account-1": {
					ID: "account-1", WorkspaceID: "workspace-1",
					ExternalUserID: "managed-owner", ExternalAccountID: test.accountProviderUserID,
					AppMode: AppModeUniPostManaged, Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
				},
			}}
			featureCalls := 0
			admissionCalls := 0
			atomicCalls := 0
			notifyCalls := 0
			service := NewIngestionService(IngestionConfig{
				Store: store,
				DMsAvailable: func(context.Context, string) (bool, error) {
					featureCalls++
					return true, nil
				},
				Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
					admissionCalls++
					return InboundAdmission{Accepted: true}, nil
				},
				AtomicProcess: func(context.Context, InboundAdmissionRequest, InboxItem) (InboundAdmission, InboxItem, bool, error) {
					atomicCalls++
					return InboundAdmission{Accepted: true}, InboxItem{}, true, nil
				},
				Notify: func(context.Context, string, string, InboxItem) { notifyCalls++ },
			})

			err := service.IngestActivityEvent(context.Background(), "route-1", ActivityEvent{
				AccountID: "account-1", ExternalUserID: test.eventProviderUserID, ExternalID: "dm-secret",
				ConversationID: "private-thread", SenderID: "sender", RecipientID: "provider-owner",
			})
			if !errors.Is(err, ErrInboxAccountNotFound) {
				t.Fatalf("error = %v, want route mismatch as account not found", err)
			}
			if err != nil && (strings.Contains(err.Error(), "provider-owner") || strings.Contains(err.Error(), "provider-other")) {
				t.Fatalf("route error leaked provider identifier: %v", err)
			}
			if store.accountCalls != 1 || featureCalls != 0 || admissionCalls != 0 || atomicCalls != 0 || len(store.inserted) != 0 || notifyCalls != 0 {
				t.Fatalf("side effects after route mismatch: account=%d feature=%d admission=%d atomic=%d insert=%d notify=%d",
					store.accountCalls, featureCalls, admissionCalls, atomicCalls, len(store.inserted), notifyCalls)
			}
		})
	}
}

func TestXIngestDMWithoutAvailabilityEvaluatorFailsBeforeSideEffects(t *testing.T) {
	store := &fakeIngestStore{accounts: map[string]InboxAccount{
		"route-1:account-1": {
			ID: "account-1", WorkspaceID: "workspace-1",
			ExternalUserID: "managed-owner", ExternalAccountID: "provider-owner",
			AppMode: AppModeUniPostManaged, Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
		},
	}}
	admissionCalls := 0
	atomicCalls := 0
	notifyCalls := 0
	service := NewIngestionService(IngestionConfig{
		Store: store,
		Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
			admissionCalls++
			return InboundAdmission{Accepted: true}, nil
		},
		AtomicProcess: func(context.Context, InboundAdmissionRequest, InboxItem) (InboundAdmission, InboxItem, bool, error) {
			atomicCalls++
			return InboundAdmission{Accepted: true}, InboxItem{}, true, nil
		},
		Notify: func(context.Context, string, string, InboxItem) { notifyCalls++ },
	})

	err := service.IngestActivityEvent(context.Background(), "route-1", ActivityEvent{
		AccountID: "account-1", ExternalUserID: "provider-owner", ExternalID: "dm-secret",
		ConversationID: "private-thread", SenderID: "sender", RecipientID: "provider-owner",
	})
	if err == nil || err.Error() != "X DM feature evaluator is not configured" {
		t.Fatalf("error = %v, want fixed missing evaluator error", err)
	}
	if admissionCalls != 0 || atomicCalls != 0 || len(store.inserted) != 0 || notifyCalls != 0 {
		t.Fatalf("side effects without evaluator: admission=%d atomic=%d insert=%d notify=%d",
			admissionCalls, atomicCalls, len(store.inserted), notifyCalls)
	}
}

func TestXIngestDMAvailabilityMatrixBeforeAllSideEffects(t *testing.T) {
	sentinel := errors.New("feature evaluator unavailable")
	for _, path := range []string{"activity", "recovery"} {
		for _, test := range []struct {
			name         string
			available    func(context.Context, string) (bool, error)
			wantErr      error
			wantFeature  int
			wantAtomic   int
			wantNotified int
		}{
			{name: "nil", wantErr: ErrDMFeatureNotConfigured},
			{name: "false", available: func(context.Context, string) (bool, error) { return false, nil }, wantFeature: 1},
			{name: "error", available: func(context.Context, string) (bool, error) { return false, sentinel }, wantErr: sentinel, wantFeature: 1},
			{name: "true", available: func(context.Context, string) (bool, error) { return true, nil }, wantFeature: 1, wantAtomic: 1, wantNotified: 1},
		} {
			t.Run(path+"/"+test.name, func(t *testing.T) {
				store := &fakeIngestStore{accounts: map[string]InboxAccount{
					"route-1:account-1": {
						ID: "account-1", WorkspaceID: "workspace-1",
						ExternalUserID: "managed-owner", ExternalAccountID: "provider-owner",
						AppMode: AppModeUniPostManaged, Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
					},
				}}
				featureCalls := 0
				available := test.available
				if available != nil {
					available = func(ctx context.Context, workspaceID string) (bool, error) {
						featureCalls++
						return test.available(ctx, workspaceID)
					}
				}
				admissionCalls := 0
				atomicCalls := 0
				notifyCalls := 0
				service := NewIngestionService(IngestionConfig{
					Store: store, DMsAvailable: available,
					Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
						admissionCalls++
						return InboundAdmission{Accepted: true}, nil
					},
					AtomicProcess: func(_ context.Context, _ InboundAdmissionRequest, item InboxItem) (InboundAdmission, InboxItem, bool, error) {
						atomicCalls++
						return InboundAdmission{Accepted: true}, item, true, nil
					},
					Notify: func(context.Context, string, string, InboxItem) { notifyCalls++ },
				})

				var err error
				if path == "activity" {
					err = service.IngestActivityEvent(context.Background(), "route-1", ActivityEvent{
						AccountID: "account-1", ExternalUserID: "provider-owner", ExternalID: "dm-1",
						ConversationID: "thread-1", SenderID: "sender", RecipientID: "provider-owner",
					})
				} else {
					_, err = service.IngestRecovery(context.Background(), store.accounts["route-1:account-1"], InboxItem{
						Source: "x_dm", ExternalID: "dm-1",
					}, "dm.received", "recovery")
				}
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("error = %v, want errors.Is(%v)", err, test.wantErr)
				}
				if test.name == "error" && err != sentinel {
					t.Fatalf("evaluator error = %v, want exact sentinel", err)
				}
				if featureCalls != test.wantFeature || atomicCalls != test.wantAtomic || notifyCalls != test.wantNotified || admissionCalls != 0 || len(store.inserted) != 0 {
					t.Fatalf("side effects: feature=%d atomic=%d notify=%d admission=%d insert=%d",
						featureCalls, atomicCalls, notifyCalls, admissionCalls, len(store.inserted))
				}
			})
		}
	}
}

func (s *fakeIngestStore) AccountsForProviderUser(_ context.Context, appClientID, providerUserID string) ([]InboxAccount, error) {
	key := appClientID + ":" + providerUserID
	s.providerKeys = append(s.providerKeys, key)
	return s.provider[key], s.providerErr
}

func (s *fakeIngestStore) InsertInboxItem(_ context.Context, item InboxItem) (InboxItem, bool, error) {
	s.inserted = append(s.inserted, item)
	if s.insertErr != nil {
		return InboxItem{}, false, s.insertErr
	}
	if s.insertResult.ExternalID == "" {
		s.insertResult = item
	}
	return s.insertResult, s.insertedNew, nil
}

func allowXDMs(context.Context, string) (bool, error) { return true, nil }

func TestXIngestReplyUsesConversationIDAndNotifiesOnlyAfterInsert(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"client-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1",
				ExternalUserID: "owner-1", ExternalAccountID: "provider-owner", AppMode: AppModeUniPostManaged,
				Scopes: []string{"tweet.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
		insertedNew: true,
	}
	var admitted []InboundAdmissionRequest
	var notified []InboxItem
	service := NewIngestionService(IngestionConfig{
		Store: store,
		Admit: func(_ context.Context, req InboundAdmissionRequest) (InboundAdmission, error) {
			admitted = append(admitted, req)
			return InboundAdmission{Accepted: true}, nil
		},
		Notify: func(_ context.Context, _, _ string, item InboxItem) {
			notified = append(notified, item)
		},
	})

	event := StreamEvent{}
	event.Data.ID = "tweet-2"
	event.Data.Text = "A public reply"
	event.Data.AuthorID = "author-2"
	event.Data.CreatedAt = "2026-07-16T12:00:00Z"
	event.Data.ConversationID = "conversation-1"
	event.Data.ReferencedTweets = []ReferencedTweet{{Type: "replied_to", ID: "tweet-1"}}
	event.MatchingRules = []StreamRule{{Tag: FilteredStreamRuleTag("account-1")}}

	if err := service.IngestStreamEvent(context.Background(), "client-1", event); err != nil {
		t.Fatalf("IngestStreamEvent: %v", err)
	}
	if len(admitted) != 1 || admitted[0].OperationKey != "post.mention.received" ||
		admitted[0].UpstreamResourceID != "tweet-2" {
		t.Fatalf("admission = %#v", admitted)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("insert count = %d, want 1", len(store.inserted))
	}
	got := store.inserted[0]
	if got.Source != "x_reply" || got.ThreadKey != "conversation-1" ||
		got.ParentExternalID != "tweet-1" || got.Body != "A public reply" {
		t.Fatalf("inserted item = %#v", got)
	}
	if eligible, _ := got.Metadata["reply_eligible"].(bool); !eligible {
		t.Fatalf("reply_eligible metadata = %#v", got.Metadata)
	}
	if len(notified) != 1 || notified[0].ExternalID != "tweet-2" {
		t.Fatalf("notifications = %#v", notified)
	}

	store.insertedNew = false
	if err := service.IngestStreamEvent(context.Background(), "client-1", event); err != nil {
		t.Fatalf("duplicate IngestStreamEvent: %v", err)
	}
	if len(notified) != 1 {
		t.Fatalf("duplicate sent notification; count = %d", len(notified))
	}
}

func TestXIngestDMSuppressionNeverPersistsPrivateBody(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"client-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1",
				ExternalUserID: "owner-1", ExternalAccountID: "provider-owner", AppMode: AppModeUniPostManaged,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
	}
	service := NewIngestionService(IngestionConfig{
		Store: store, DMsAvailable: allowXDMs,
		Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
			return InboundAdmission{Accepted: false, Suppressed: true}, nil
		},
	})
	event := ActivityEvent{
		AccountID:      "account-1",
		ExternalUserID: "provider-owner",
		ExternalID:     "dm-1",
		ConversationID: "dm-conversation-1",
		SenderID:       "sender-1",
		RecipientID:    "owner-1",
		Text:           "private secret body",
		CreatedAt:      time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	}
	if err := service.IngestActivityEvent(context.Background(), "client-1", event); err != nil {
		t.Fatalf("IngestActivityEvent: %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("suppressed DM was persisted: %#v", store.inserted)
	}
}

func TestXIngestDMFeatureOffNeverAdmitsOrPersistsPrivateBody(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"client-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1",
				ExternalUserID: "owner-1", ExternalAccountID: "provider-owner", AppMode: AppModeUniPostManaged,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
		insertedNew: true,
	}
	admitted := false
	service := NewIngestionService(IngestionConfig{
		Store: store,
		DMsAvailable: func(context.Context, string) (bool, error) {
			return false, nil
		},
		Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
			admitted = true
			return InboundAdmission{Accepted: true}, nil
		},
	})
	err := service.IngestActivityEvent(context.Background(), "client-1", ActivityEvent{
		AccountID: "account-1", ExternalUserID: "provider-owner", ExternalID: "dm-flag-off", ConversationID: "private-thread",
		SenderID: "sender-1", RecipientID: "owner-1", Text: "must not persist",
	})
	if err != nil {
		t.Fatal(err)
	}
	if admitted || len(store.inserted) != 0 {
		t.Fatalf("admitted=%v inserted=%#v, want DM dropped before admission", admitted, store.inserted)
	}
}

func TestXIngestDMUsesCanonicalConversationIDAndDeduplicates(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"client-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1",
				ExternalUserID: "owner-1", ExternalAccountID: "provider-owner", AppMode: AppModeWorkspace,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
		insertedNew: true,
	}
	admissions := 0
	service := NewIngestionService(IngestionConfig{
		Store: store, DMsAvailable: allowXDMs,
		Admit: func(_ context.Context, req InboundAdmissionRequest) (InboundAdmission, error) {
			admissions++
			if req.OperationKey != "dm.received" || req.UpstreamResourceType != "x_dm" {
				t.Fatalf("admission request = %#v", req)
			}
			return InboundAdmission{Accepted: true, Duplicate: admissions > 1}, nil
		},
	})
	event := ActivityEvent{
		AccountID:      "account-1",
		ExternalUserID: "provider-owner",
		ExternalID:     "dm-1",
		ConversationID: "dm-conversation-1",
		SenderID:       "sender-1",
		RecipientID:    "owner-1",
		Text:           "hello",
		CreatedAt:      time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	}
	if err := service.IngestActivityEvent(context.Background(), "client-1", event); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	store.insertedNew = false
	if err := service.IngestActivityEvent(context.Background(), "client-1", event); err != nil {
		t.Fatalf("duplicate ingest: %v", err)
	}
	if len(store.inserted) != 2 {
		t.Fatalf("insert attempts = %d, want 2 so a charged-but-failed insert can recover", len(store.inserted))
	}
	if got := store.inserted[0]; got.Source != "x_dm" || got.ThreadKey != "dm-conversation-1" ||
		got.ParentExternalID != "dm-conversation-1" || got.Body != "hello" {
		t.Fatalf("DM item = %#v", got)
	}
}

func TestXIngestDoesNotNotifyWhenInsertFails(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"client-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1", ExternalAccountID: "provider-owner", AppMode: AppModeWorkspace,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
		insertErr: errors.New("database unavailable"),
	}
	notified := false
	service := NewIngestionService(IngestionConfig{
		Store: store, DMsAvailable: allowXDMs,
		AtomicProcess: func(context.Context, InboundAdmissionRequest, InboxItem) (InboundAdmission, InboxItem, bool, error) {
			return InboundAdmission{}, InboxItem{}, false, errors.New("database unavailable")
		},
		Notify: func(context.Context, string, string, InboxItem) { notified = true },
	})
	event := ActivityEvent{AccountID: "account-1", ExternalUserID: "provider-owner", ExternalID: "dm-1", ConversationID: "c-1"}
	if err := service.IngestActivityEvent(context.Background(), "client-1", event); err == nil {
		t.Fatal("expected insert error")
	}
	if notified {
		t.Fatal("notified after failed insert")
	}
}

func TestXIngestUsesAtomicAdmissionInsertPath(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"route-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1", ExternalUserID: "owner-1",
				ExternalAccountID: "provider-owner",
				AppMode:           AppModeUniPostManaged, Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
	}
	atomicCalls := 0
	notified := 0
	service := NewIngestionService(IngestionConfig{
		Store: store, DMsAvailable: allowXDMs,
		AtomicProcess: func(_ context.Context, req InboundAdmissionRequest, item InboxItem) (InboundAdmission, InboxItem, bool, error) {
			atomicCalls++
			if req.UpstreamResourceID != item.ExternalID {
				t.Fatalf("request/item mismatch: req=%+v item=%+v", req, item)
			}
			item.ID = "inbox-1"
			return InboundAdmission{Accepted: true}, item, true, nil
		},
		Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
			t.Fatal("split admission path must not run when atomic processor is configured")
			return InboundAdmission{}, nil
		},
		Notify: func(context.Context, string, string, InboxItem) { notified++ },
	})
	err := service.IngestActivityEvent(context.Background(), "route-1", ActivityEvent{
		AccountID: "account-1", ExternalUserID: "provider-owner", ExternalID: "dm-1", ConversationID: "conversation-1",
		SenderID: "sender-1", RecipientID: "owner-1", Text: "private",
		CreatedAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if atomicCalls != 1 || notified != 1 || len(store.inserted) != 0 {
		t.Fatalf("atomicCalls=%d notified=%d split inserts=%d", atomicCalls, notified, len(store.inserted))
	}
}

func TestXRealtimeOwnerUsesDatabaseAccountInsteadOfPayload(t *testing.T) {
	for _, test := range []struct {
		name         string
		atomic       bool
		accountOwner string
		wantOwner    string
	}{
		{name: "non-atomic managed", accountOwner: "managed-a", wantOwner: "managed-a"},
		{name: "atomic managed", atomic: true, accountOwner: "managed-a", wantOwner: "managed-a"},
		{name: "non-atomic BYO", wantOwner: ""},
		{name: "atomic BYO", atomic: true, wantOwner: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeIngestStore{
				accounts: map[string]InboxAccount{
					"client-1:account-1": {
						ID: "account-1", WorkspaceID: "workspace-1", ExternalUserID: test.accountOwner,
						ExternalAccountID: "provider-owner",
						AppMode:           AppModeWorkspace, Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
					},
				},
				insertedNew: true,
			}
			var notifiedWorkspace, notifiedOwner string
			notified := 0
			config := IngestionConfig{
				Store: store, DMsAvailable: allowXDMs,
				Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
					return InboundAdmission{Accepted: true}, nil
				},
				Notify: func(_ context.Context, workspaceID, externalUserID string, _ InboxItem) {
					notified++
					notifiedWorkspace = workspaceID
					notifiedOwner = externalUserID
				},
			}
			if test.atomic {
				config.AtomicProcess = func(_ context.Context, _ InboundAdmissionRequest, item InboxItem) (InboundAdmission, InboxItem, bool, error) {
					return InboundAdmission{Accepted: true}, item, true, nil
				}
			}
			service := NewIngestionService(config)
			event := ActivityEvent{
				AccountID: "account-1", ExternalUserID: "provider-owner", ExternalID: "dm-1",
				ConversationID: "conversation-1", SenderID: "sender-1", RecipientID: "provider-owner",
			}

			if err := service.IngestActivityEvent(context.Background(), "client-1", event); err != nil {
				t.Fatalf("IngestActivityEvent: %v", err)
			}
			if notified != 1 || notifiedWorkspace != "workspace-1" || notifiedOwner != test.wantOwner {
				t.Fatalf("notification = count %d workspace %q owner %q, want 1/workspace-1/%q", notified, notifiedWorkspace, notifiedOwner, test.wantOwner)
			}
		})
	}
}

func TestXRecoveryReusesAtomicIngestionAndReturnsAdmissionResult(t *testing.T) {
	admissionNow := time.Date(2026, 7, 16, 15, 0, 0, 0, time.UTC)
	service := NewIngestionService(IngestionConfig{
		Store: &fakeIngestStore{},
		AtomicProcess: func(_ context.Context, req InboundAdmissionRequest, item InboxItem) (InboundAdmission, InboxItem, bool, error) {
			if req.OperationKey != "post.read" || req.Source != "backfill" ||
				req.UpstreamResourceID != item.ExternalID {
				t.Fatalf("request = %+v item = %+v", req, item)
			}
			if !req.Now.Equal(admissionNow) {
				t.Fatalf("admission time = %s, want paid-read time %s", req.Now, admissionNow)
			}
			item.ID = "inbox-1"
			return InboundAdmission{Accepted: true}, item, true, nil
		},
		Now: func() time.Time { return admissionNow },
	})
	result, err := service.IngestRecovery(
		context.Background(),
		InboxAccount{
			ID: "account-1", WorkspaceID: "workspace-1",
			AppMode: AppModeUniPostManaged, PlanAllowsInbox: true,
		},
		InboxItem{
			Source: "x_reply", ExternalID: "tweet-1",
			ReceivedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		},
		"post.read",
		"backfill",
	)
	if err != nil {
		t.Fatalf("IngestRecovery: %v", err)
	}
	if !result.Admission.Accepted || !result.Inserted || result.Item.ID != "inbox-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestParseXActivityFixtures(t *testing.T) {
	t.Run("current X Activity dm.received envelope", func(t *testing.T) {
		body, err := os.ReadFile("testdata/x_activity_dm_received.json")
		if err != nil {
			t.Fatal(err)
		}
		events, err := ParseActivityEvents(body)
		if err != nil {
			t.Fatalf("ParseActivityEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("events = %#v", events)
		}
		got := events[0]
		if got.AccountID != "account-1" || got.ExternalID != "dm-1" ||
			got.ConversationID != "conversation-1" || got.RecipientID != "owner-1" ||
			got.Text != "hello" {
			t.Fatalf("event = %#v", got)
		}
	})

	t.Run("filtered stream reply envelope", func(t *testing.T) {
		body, err := os.ReadFile("testdata/filtered_stream_reply.json")
		if err != nil {
			t.Fatal(err)
		}
		var event StreamEvent
		if err := json.Unmarshal(body, &event); err != nil {
			t.Fatal(err)
		}
		if event.Data.ConversationID != "conversation-1" ||
			repliedToID(event.Data.ReferencedTweets) != "tweet-1" ||
			len(streamAccountIDs(event.MatchingRules)) != 1 {
			t.Fatalf("event = %#v", event)
		}
	})

	t.Run("legacy Account Activity direct_message_events envelope", func(t *testing.T) {
		body := []byte(`{
		  "for_user_id": "owner-1",
		  "direct_message_events": [{
		    "type": "message_create",
		    "id": "dm-2",
		    "created_timestamp": "1784203200000",
		    "message_create": {
		      "target": {"recipient_id": "owner-1"},
		      "sender_id": "sender-1",
		      "message_data": {"text": "legacy hello"}
		    }
		  }]
		}`)
		events, err := ParseActivityEvents(body)
		if err != nil {
			t.Fatalf("ParseActivityEvents: %v", err)
		}
		if len(events) != 1 || events[0].ThreadKey() != "x-dm:owner-1:sender-1" {
			t.Fatalf("events = %#v", events)
		}
	})
}

func TestXIngestLegacyProviderUserRequiresExactlyOneAccountBeforeSideEffects(t *testing.T) {
	validAccount := func(id, workspaceID string) InboxAccount {
		return InboxAccount{
			ID: id, WorkspaceID: workspaceID, ExternalUserID: "database-owner",
			ExternalAccountID: "provider-owner", AppMode: AppModeUniPostManaged,
			Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
		}
	}
	lookupFailure := errors.New("row conversion failed")
	for _, test := range []struct {
		name      string
		accounts  []InboxAccount
		lookupErr error
		wantErr   error
	}{
		{name: "zero", wantErr: ErrInboxAccountNotFound},
		{name: "multiple same workspace", accounts: []InboxAccount{validAccount("account-1", "workspace-1"), validAccount("account-2", "workspace-1")}, wantErr: ErrInboxAccountAmbiguous},
		{name: "multiple workspaces", accounts: []InboxAccount{validAccount("account-1", "workspace-1"), validAccount("account-2", "workspace-2")}, wantErr: ErrInboxAccountAmbiguous},
		{name: "row conversion lookup error", lookupErr: lookupFailure, wantErr: lookupFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeIngestStore{
				provider:    map[string][]InboxAccount{"route-1:provider-owner": test.accounts},
				providerErr: test.lookupErr,
			}
			admissions := 0
			atomicCalls := 0
			notifications := 0
			service := NewIngestionService(IngestionConfig{
				Store: store,
				DMsAvailable: func(context.Context, string) (bool, error) {
					t.Fatal("feature eligibility must not run before exact-one routing")
					return true, nil
				},
				Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
					admissions++
					return InboundAdmission{Accepted: true}, nil
				},
				AtomicProcess: func(context.Context, InboundAdmissionRequest, InboxItem) (InboundAdmission, InboxItem, bool, error) {
					atomicCalls++
					return InboundAdmission{Accepted: true}, InboxItem{}, true, nil
				},
				Notify: func(context.Context, string, string, InboxItem) { notifications++ },
			})

			err := service.IngestActivityEvent(context.Background(), "route-1", ActivityEvent{
				ExternalUserID: "provider-owner", ExternalID: "dm-1",
				SenderID: "sender", RecipientID: "provider-owner", CreatedAt: time.Now().UTC(),
			})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want errors.Is(%v)", err, test.wantErr)
			}
			if admissions != 0 || atomicCalls != 0 || len(store.inserted) != 0 || notifications != 0 {
				t.Fatalf("side effects before exact-one routing: admissions=%d atomic=%d inserts=%d notifications=%d", admissions, atomicCalls, len(store.inserted), notifications)
			}
		})
	}
}

func TestXIngestCurrentTagAndLegacyProviderUserUseExactDatabaseRoutes(t *testing.T) {
	currentAccount := InboxAccount{
		ID: "current-account", WorkspaceID: "current-workspace", ExternalUserID: "current-db-owner",
		ExternalAccountID: "provider-owner",
		AppMode:           AppModeUniPostManaged, Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
	}
	legacyAccount := InboxAccount{
		ID: "legacy-account", WorkspaceID: "legacy-workspace", ExternalUserID: "legacy-db-owner",
		ExternalAccountID: "provider-owner", AppMode: AppModeUniPostManaged,
		Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
	}
	store := &fakeIngestStore{
		accounts:    map[string]InboxAccount{"route-1:current-account": currentAccount},
		provider:    map[string][]InboxAccount{"route-1:provider-owner": {legacyAccount}},
		insertedNew: true,
	}
	var admissions []InboundAdmissionRequest
	var notifications [][2]string
	service := NewIngestionService(IngestionConfig{
		Store: store, DMsAvailable: allowXDMs,
		Admit: func(_ context.Context, req InboundAdmissionRequest) (InboundAdmission, error) {
			admissions = append(admissions, req)
			return InboundAdmission{Accepted: true}, nil
		},
		Notify: func(_ context.Context, workspaceID, ownerID string, _ InboxItem) {
			notifications = append(notifications, [2]string{workspaceID, ownerID})
		},
	})

	currentBody := []byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"provider-owner"},"tag":"unipost:x:dm:current-account","payload":{"id":"current-dm","dm_conversation_id":"current-thread","sender_id":"sender","recipient_id":"provider-owner","workspace_id":"forged-workspace","owner_id":"forged-owner","created_at":"2026-07-16T12:00:00Z"}}}`)
	legacyBody := []byte(`{"for_user_id":"provider-owner","workspace_id":"forged-workspace","owner_id":"forged-owner","direct_message_events":[{"type":"message_create","id":"legacy-dm","created_timestamp":"1784203200000","message_create":{"target":{"recipient_id":"provider-owner"},"sender_id":"sender","message_data":{"text":"hello"}}}]}`)
	for _, body := range [][]byte{currentBody, legacyBody} {
		events, err := ParseActivityEvents(body)
		if err != nil || len(events) != 1 {
			t.Fatalf("ParseActivityEvents(%s) = %#v, %v", body, events, err)
		}
		if err := service.IngestActivityEvent(context.Background(), "route-1", events[0]); err != nil {
			t.Fatalf("IngestActivityEvent: %v", err)
		}
	}
	if len(store.providerKeys) != 1 || store.providerKeys[0] != "route-1:provider-owner" {
		t.Fatalf("provider lookups = %#v, want only exact legacy provider id", store.providerKeys)
	}
	if len(admissions) != 2 ||
		admissions[0].WorkspaceID != "current-workspace" || admissions[0].SocialAccountID != "current-account" ||
		admissions[1].WorkspaceID != "legacy-workspace" || admissions[1].SocialAccountID != "legacy-account" {
		t.Fatalf("admissions = %#v", admissions)
	}
	if len(notifications) != 2 || notifications[0] != [2]string{"current-workspace", "current-db-owner"} || notifications[1] != [2]string{"legacy-workspace", "legacy-db-owner"} {
		t.Fatalf("notifications = %#v", notifications)
	}
	if len(store.inserted) != 2 || store.inserted[0].WorkspaceID != "current-workspace" || store.inserted[1].WorkspaceID != "legacy-workspace" {
		t.Fatalf("inserted = %#v", store.inserted)
	}
}

func TestXChatAndNonReceivedDMEventsNeverProduceActivityEvents(t *testing.T) {
	for _, eventType := range []string{"chat.received", "chat.sent", "chat.conversation_join", "dm.sent", "dm.read", "typing", "unknown"} {
		t.Run(eventType, func(t *testing.T) {
			admissions := 0
			service := NewIngestionService(IngestionConfig{
				Store: &fakeIngestStore{}, DMsAvailable: allowXDMs,
				Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
					admissions++
					return InboundAdmission{Accepted: true}, nil
				},
			})
			body := []byte(`{"data":{"event_type":"` + eventType + `","payload":{"id":"current-event"}},"for_user_id":"provider-owner","direct_message_events":[{"type":"message_create","id":"legacy-dm","created_timestamp":"1784203200000","message_create":{"target":{"recipient_id":"provider-owner"},"sender_id":"sender","message_data":{"text":"must be ignored"}}}]}`)
			events, err := ParseActivityEvents(body)
			if err != nil {
				t.Fatalf("ParseActivityEvents: %v", err)
			}
			if len(events) != 0 {
				t.Fatalf("%s produced x_dm candidates: %#v", eventType, events)
			}
			for _, event := range events {
				if err := service.IngestActivityEvent(context.Background(), "route-1", event); err != nil {
					t.Fatalf("unexpected ingestion error: %v", err)
				}
			}
			if admissions != 0 {
				t.Fatalf("%s reached admission %d times, want zero", eventType, admissions)
			}
		})
	}
}

func TestParseXActivityCurrentEnvelopeNeverFallsBackToLegacyEvents(t *testing.T) {
	currentPayload := `"filter":{"user_id":"owner"},"tag":"unipost:x:dm:account-1","payload":{"id":"current-dm","dm_conversation_id":"current-thread","sender_id":"sender","recipient_id":"owner","created_at":"2026-07-16T12:00:00Z"}`
	for _, test := range []struct {
		name          string
		eventType     string
		legacyEvent   string
		wantCurrentID string
	}{
		{
			name:          "valid attached legacy",
			eventType:     "dm.received",
			legacyEvent:   `{"type":"message_create","id":"legacy-dm","created_timestamp":"1784203200000","message_create":{"target":{"recipient_id":"owner"},"sender_id":"sender","message_data":{"text":"legacy"}}}`,
			wantCurrentID: "current-dm",
		},
		{
			name:          "malformed attached legacy",
			eventType:     "dm.received",
			legacyEvent:   `{"type":"message_create"}`,
			wantCurrentID: "current-dm",
		},
		{
			name:          "trimmed current event type",
			eventType:     " dm.received ",
			legacyEvent:   `{"type":"message_create","id":"legacy-dm","created_timestamp":"1784203200000","message_create":{"target":{"recipient_id":"owner"},"sender_id":"sender"}}`,
			wantCurrentID: "current-dm",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(`{"data":{"event_type":"` + test.eventType + `",` + currentPayload + `},"for_user_id":"owner","direct_message_events":[` + test.legacyEvent + `]}`)
			events, err := ParseActivityEvents(body)
			if err != nil {
				t.Fatalf("ParseActivityEvents: %v", err)
			}
			if len(events) != 1 || events[0].ExternalID != test.wantCurrentID || events[0].AccountID != "account-1" {
				t.Fatalf("events = %#v, want only current event", events)
			}
		})
	}
}

func TestParseXActivityCurrentShapeRequiresNonBlankEventType(t *testing.T) {
	for _, data := range []string{
		`"payload":{"id":"current-dm"}`,
		`"event_type":"   ","payload":{"id":"current-dm"}`,
	} {
		body := []byte(`{"data":{` + data + `},"for_user_id":"owner","direct_message_events":[{"type":"message_create","id":"legacy-dm","created_timestamp":"1784203200000","message_create":{"target":{"recipient_id":"owner"},"sender_id":"sender"}}]}`)
		events, err := ParseActivityEvents(body)
		if !errors.Is(err, ErrMalformedEvent) {
			t.Fatalf("ParseActivityEvents(%s) = %#v, %v; want malformed current envelope", body, events, err)
		}
		if len(events) != 0 {
			t.Fatalf("current-shaped envelope fell back to legacy: %#v", events)
		}
	}
}

type fakeSecretStore struct {
	values map[string][]string
}

func (f fakeSecretStore) EncryptedConsumerSecrets(_ context.Context, routeKey string) ([]string, error) {
	return f.values[routeKey], nil
}

func TestXWebhookSecretResolverRotationLifecycle(t *testing.T) {
	store := &fakeSecretStore{values: map[string][]string{
		"stable-route": {"encrypted-old"},
	}}
	resolver := NewAppSecretResolver(AppSecretResolverConfig{
		Store: store,
		Decrypt: func(value string) (string, error) {
			switch value {
			case "encrypted-old":
				return "old-consumer-secret", nil
			case "encrypted-new":
				return "new-consumer-secret", nil
			default:
				return "", errors.New("unknown ciphertext")
			}
		},
	})
	if got, err := resolver.ConsumerSecret(context.Background(), "stable-route"); err != nil || got != "old-consumer-secret" {
		t.Fatalf("old route secret = %q err=%v", got, err)
	}

	// Same-app secret rotation updates signature validation immediately while
	// retaining the already registered webhook URL.
	store.values["stable-route"] = []string{"encrypted-new"}
	if got, err := resolver.ConsumerSecret(context.Background(), "stable-route"); err != nil || got != "new-consumer-secret" {
		t.Fatalf("rotated route secret = %q err=%v", got, err)
	}

	// An old app-generation route is present only while its cleanup intent
	// contributes encrypted signature material to the resolver query.
	store.values["old-generation-route"] = []string{"encrypted-old"}
	if _, err := resolver.ConsumerSecret(context.Background(), "old-generation-route"); err != nil {
		t.Fatalf("pending cleanup route was not valid: %v", err)
	}
	delete(store.values, "old-generation-route")
	if _, err := resolver.ConsumerSecret(context.Background(), "old-generation-route"); !errors.Is(err, ErrAppSecretNotFound) {
		t.Fatalf("completed cleanup route error = %v, want not found", err)
	}
}

func TestXWebhookSecretResolverKeepsAppsIsolatedAndFailsOnConflict(t *testing.T) {
	managedRoute := WebhookRouteKey("stable-route-secret", "managed-client")
	workspaceRoute := "random-workspace-route"
	resolver := NewAppSecretResolver(AppSecretResolverConfig{
		ManagedRouteKey: managedRoute,
		ManagedSecret:   "managed-secret",
		Store: fakeSecretStore{values: map[string][]string{
			workspaceRoute: {"encrypted-one", "encrypted-two"},
		}},
		Decrypt: func(value string) (string, error) {
			switch value {
			case "encrypted-one", "encrypted-two":
				return "workspace-secret", nil
			default:
				return "", errors.New("unknown ciphertext")
			}
		},
	})
	if got, err := resolver.ConsumerSecret(context.Background(), managedRoute); err != nil || got != "managed-secret" {
		t.Fatalf("managed secret = %q err=%v", got, err)
	}
	if got, err := resolver.ConsumerSecret(context.Background(), workspaceRoute); err != nil || got != "workspace-secret" {
		t.Fatalf("workspace secret = %q err=%v", got, err)
	}
	if _, err := resolver.ConsumerSecret(context.Background(), "other-route"); !errors.Is(err, ErrAppSecretNotFound) {
		t.Fatalf("missing app err = %v", err)
	}

	attackerRoute := "different-random-workspace-route"
	isolated := NewAppSecretResolver(AppSecretResolverConfig{
		Store: fakeSecretStore{values: map[string][]string{
			workspaceRoute: {"victim-ciphertext"},
			attackerRoute:  {"attacker-ciphertext"},
		}},
		Decrypt: func(value string) (string, error) {
			if value == "victim-ciphertext" {
				return "workspace-secret", nil
			}
			return "attacker-secret", nil
		},
	})
	if got, err := isolated.ConsumerSecret(context.Background(), workspaceRoute); err != nil || got != "workspace-secret" {
		t.Fatalf("victim route affected by attacker: secret=%q err=%v", got, err)
	}

	conflicting := NewAppSecretResolver(AppSecretResolverConfig{
		Store: fakeSecretStore{values: map[string][]string{
			workspaceRoute: {"encrypted-one", "encrypted-conflict"},
		}},
		Decrypt: func(value string) (string, error) {
			if value == "encrypted-one" {
				return "first-secret", nil
			}
			return "different-secret", nil
		},
	})
	if _, err := conflicting.ConsumerSecret(context.Background(), workspaceRoute); err == nil {
		t.Fatal("expected conflicting workspace secrets to fail closed")
	}
}

func TestManagedWebhookRouteKeyIsStableAcrossConsumerSecretRotation(t *testing.T) {
	first := WebhookRouteKey("stable-route-secret", "client-id")
	if first == "" || first != WebhookRouteKey("stable-route-secret", "client-id") {
		t.Fatalf("route key is not deterministic: %q", first)
	}
	for _, consumerSecret := range []string{"old-consumer-secret", "rotated-consumer-secret"} {
		if got := WebhookRouteKey("stable-route-secret", "client-id"); got != first {
			t.Fatalf("consumer secret %q changed managed route to %q", consumerSecret, got)
		}
	}
	if first == WebhookRouteKey("other-route-secret", "client-id") {
		t.Fatal("route key was not bound to dedicated route secret")
	}
	if first == WebhookRouteKey("stable-route-secret", "other-client") {
		t.Fatal("route key was not bound to client id")
	}
}

func TestRandomWebhookRouteKeysAreOpaqueAndUnique(t *testing.T) {
	first, err := RandomWebhookRouteKey()
	if err != nil {
		t.Fatal(err)
	}
	second, err := RandomWebhookRouteKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 32 || len(second) < 32 || first == second {
		t.Fatalf("random routes = %q and %q", first, second)
	}
}

func TestXIngestRejectsRecognizedMalformedEvents(t *testing.T) {
	service := NewIngestionService(IngestionConfig{Store: &fakeIngestStore{}})
	stream := StreamEvent{}
	stream.Data.ID = "tweet-1"
	stream.MatchingRules = []StreamRule{{Tag: FilteredStreamRuleTag("account-1")}}
	if err := service.IngestStreamEvent(context.Background(), "route", stream); !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("malformed stream error = %v", err)
	}

	for _, body := range [][]byte{
		[]byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"owner"},"tag":"unipost:x:dm:account","payload":{"sender_id":"sender","created_at":"2026-07-16T12:00:00Z"}}}`),
		[]byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"owner"},"payload":{"id":"dm","dm_conversation_id":"conversation","sender_id":"sender","recipient_id":"owner","created_at":"2026-07-16T12:00:00Z"}}}`),
		[]byte(`{"data":{"event_type":"dm.received","tag":"unipost:x:dm:account","payload":{"id":"dm","sender_id":"sender","created_at":"2026-07-16T12:00:00Z"}}}`),
		[]byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"owner"},"tag":"unipost:x:dm:account","payload":{"id":"dm","sender_id":"sender"}}}`),
		[]byte(`{"for_user_id":"owner","direct_message_events":[{"type":"message_create","created_timestamp":"1784212800000","message_create":{"target":{"recipient_id":"owner"},"sender_id":"sender"}}]}`),
	} {
		if _, err := ParseActivityEvents(body); !errors.Is(err, ErrMalformedEvent) {
			t.Fatalf("malformed activity error = %v body=%s", err, body)
		}
	}
}

func TestMatchingXInboxOutboundWebhookCandidateRequiresExactlyOnePayloadMatch(t *testing.T) {
	sentAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	deadline := sentAt.Add(30 * time.Minute)
	candidates := []xInboxOutboundWebhookCandidate{
		{
			ID:                     "request-1",
			InboxItemID:            "target-1",
			PayloadHash:            xInboxOutboundWebhookPayloadHash("target-1", "x_reply", "thanks"),
			SendStartedAt:          sentAt,
			ReconciliationDeadline: deadline,
		},
		{
			ID:                     "request-2",
			InboxItemID:            "target-2",
			PayloadHash:            xInboxOutboundWebhookPayloadHash("target-2", "x_reply", "different"),
			SendStartedAt:          sentAt,
			ReconciliationDeadline: deadline,
		},
	}
	if got, ok := matchingXInboxOutboundWebhookCandidate(
		candidates, "x_reply", "thanks", sentAt.Add(time.Minute),
	); !ok || got != "request-1" {
		t.Fatalf("match = %q, %v", got, ok)
	}
	candidates[1].PayloadHash = xInboxOutboundWebhookPayloadHash("target-2", "x_reply", "thanks")
	if got, ok := matchingXInboxOutboundWebhookCandidate(
		candidates, "x_reply", "thanks", sentAt.Add(time.Minute),
	); ok || got != "" {
		t.Fatalf("ambiguous match = %q, %v", got, ok)
	}
}

func TestMatchingXInboxOutboundWebhookCandidateRejectsLateIdenticalManualSend(t *testing.T) {
	sentAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	candidate := xInboxOutboundWebhookCandidate{
		ID:                     "request-1",
		InboxItemID:            "target-1",
		PayloadHash:            xInboxOutboundWebhookPayloadHash("target-1", "x_reply", "same text"),
		SendStartedAt:          sentAt,
		ReconciliationDeadline: sentAt.Add(30 * time.Minute),
	}
	for _, eventAt := range []time.Time{time.Time{}, sentAt.Add(31 * time.Minute)} {
		if got, ok := matchingXInboxOutboundWebhookCandidate(
			[]xInboxOutboundWebhookCandidate{candidate}, "x_reply", "same text", eventAt,
		); ok || got != "" {
			t.Fatalf("late/unprovable event at %s matched = %q, %v", eventAt, got, ok)
		}
	}
}

func TestXInboxWebhookConflictHealingMustHashPersistedItem(t *testing.T) {
	sentAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	candidate := xInboxOutboundWebhookCandidate{
		ID:                     "request-1",
		InboxItemID:            "target-1",
		PayloadHash:            xInboxOutboundWebhookPayloadHash("target-1", "x_reply", "persisted body"),
		SendStartedAt:          sentAt,
		ReconciliationDeadline: sentAt.Add(30 * time.Minute),
	}
	if got, ok := matchingXInboxOutboundWebhookCandidate(
		[]xInboxOutboundWebhookCandidate{candidate}, "x_reply", "incoming conflicting body", sentAt,
	); ok || got != "" {
		t.Fatalf("incoming conflicting payload matched = %q, %v", got, ok)
	}
	if got, ok := matchingXInboxOutboundWebhookCandidate(
		[]xInboxOutboundWebhookCandidate{candidate}, "x_reply", "persisted body", sentAt,
	); !ok || got != "request-1" {
		t.Fatalf("persisted payload match = %q, %v", got, ok)
	}
}
