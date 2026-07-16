package xinbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

type fakeIngestStore struct {
	accounts     map[string]InboxAccount
	external     map[string][]InboxAccount
	inserted     []InboxItem
	insertResult InboxItem
	insertedNew  bool
	insertErr    error
}

func (s *fakeIngestStore) AccountForApp(_ context.Context, appClientID, accountID string) (InboxAccount, error) {
	account, ok := s.accounts[appClientID+":"+accountID]
	if !ok {
		return InboxAccount{}, ErrInboxAccountNotFound
	}
	return account, nil
}

func (s *fakeIngestStore) AccountsForExternalUser(_ context.Context, appClientID, externalUserID string) ([]InboxAccount, error) {
	return s.external[appClientID+":"+externalUserID], nil
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

func TestXIngestReplyUsesConversationIDAndNotifiesOnlyAfterInsert(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"client-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1",
				ExternalUserID: "owner-1", AppMode: AppModeUniPostManaged,
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
		Notify: func(_ context.Context, _ string, item InboxItem) {
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
				ExternalUserID: "owner-1", AppMode: AppModeUniPostManaged,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
	}
	service := NewIngestionService(IngestionConfig{
		Store: store,
		Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
			return InboundAdmission{Accepted: false, Suppressed: true}, nil
		},
	})
	event := ActivityEvent{
		AccountID:      "account-1",
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

func TestXIngestDMUsesCanonicalConversationIDAndDeduplicates(t *testing.T) {
	store := &fakeIngestStore{
		accounts: map[string]InboxAccount{
			"client-1:account-1": {
				ID: "account-1", WorkspaceID: "workspace-1",
				ExternalUserID: "owner-1", AppMode: AppModeWorkspace,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
		insertedNew: true,
	}
	admissions := 0
	service := NewIngestionService(IngestionConfig{
		Store: store,
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
				ID: "account-1", WorkspaceID: "workspace-1", AppMode: AppModeWorkspace,
				Scopes: []string{"dm.read", "users.read"}, PlanAllowsInbox: true,
			},
		},
		insertErr: errors.New("database unavailable"),
	}
	notified := false
	service := NewIngestionService(IngestionConfig{
		Store: store,
		Admit: func(context.Context, InboundAdmissionRequest) (InboundAdmission, error) {
			return InboundAdmission{Accepted: true}, nil
		},
		Notify: func(context.Context, string, InboxItem) { notified = true },
	})
	event := ActivityEvent{AccountID: "account-1", ExternalID: "dm-1", ConversationID: "c-1"}
	if err := service.IngestActivityEvent(context.Background(), "client-1", event); err == nil {
		t.Fatal("expected insert error")
	}
	if notified {
		t.Fatal("notified after failed insert")
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

type fakeSecretStore struct {
	values map[string][]string
}

func (f fakeSecretStore) EncryptedConsumerSecrets(_ context.Context, appClientID string) ([]string, error) {
	return f.values[appClientID], nil
}

func TestXWebhookSecretResolverKeepsAppsIsolatedAndFailsOnConflict(t *testing.T) {
	resolver := NewAppSecretResolver(AppSecretResolverConfig{
		ManagedAppClientID: "managed-client",
		ManagedSecret:      "managed-secret",
		Store: fakeSecretStore{values: map[string][]string{
			"workspace-client": {"encrypted-one", "encrypted-two"},
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
	if got, err := resolver.ConsumerSecret(context.Background(), "managed-client"); err != nil || got != "managed-secret" {
		t.Fatalf("managed secret = %q err=%v", got, err)
	}
	if got, err := resolver.ConsumerSecret(context.Background(), "workspace-client"); err != nil || got != "workspace-secret" {
		t.Fatalf("workspace secret = %q err=%v", got, err)
	}
	if _, err := resolver.ConsumerSecret(context.Background(), "other-client"); !errors.Is(err, ErrAppSecretNotFound) {
		t.Fatalf("missing app err = %v", err)
	}

	conflicting := NewAppSecretResolver(AppSecretResolverConfig{
		Store: fakeSecretStore{values: map[string][]string{
			"workspace-client": {"encrypted-one", "encrypted-conflict"},
		}},
		Decrypt: func(value string) (string, error) {
			if value == "encrypted-one" {
				return "first-secret", nil
			}
			return "different-secret", nil
		},
	})
	if _, err := conflicting.ConsumerSecret(context.Background(), "workspace-client"); err == nil {
		t.Fatal("expected conflicting workspace secrets to fail closed")
	}
}
