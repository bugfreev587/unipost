package xinbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInboxAccountNotFound   = errors.New("X inbox account not found")
	ErrInboxAccountAmbiguous  = errors.New("X inbox account routing is ambiguous")
	ErrAppSecretNotFound      = errors.New("X app consumer secret not found")
	ErrMalformedEvent         = errors.New("malformed recognized X inbox event")
	ErrDMFeatureNotConfigured = errors.New("X DM feature evaluator is not configured")
)

type InboxAccount struct {
	ID                string
	WorkspaceID       string
	ExternalUserID    string
	ExternalAccountID string
	AccountName       string
	AppMode           AppMode
	Scopes            []string
	ConnectionType    string
	PlanID            string
	PlanAllowsInbox   bool
}

type InboxItem struct {
	ID               string         `json:"id"`
	SocialAccountID  string         `json:"social_account_id"`
	WorkspaceID      string         `json:"workspace_id"`
	Source           string         `json:"source"`
	ExternalID       string         `json:"external_id"`
	ParentExternalID string         `json:"parent_external_id,omitempty"`
	AuthorName       string         `json:"author_name,omitempty"`
	AuthorID         string         `json:"author_id,omitempty"`
	AuthorAvatarURL  string         `json:"author_avatar_url,omitempty"`
	Body             string         `json:"body,omitempty"`
	IsRead           bool           `json:"is_read"`
	IsOwn            bool           `json:"is_own"`
	ReceivedAt       time.Time      `json:"received_at"`
	CreatedAt        time.Time      `json:"created_at"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	ThreadKey        string         `json:"thread_key"`
	ThreadStatus     string         `json:"thread_status"`
	AssignedTo       string         `json:"assigned_to,omitempty"`
	LinkedPostID     string         `json:"linked_post_id,omitempty"`
}

type ActivityEvent struct {
	AccountID       string
	ExternalUserID  string
	ExternalID      string
	ConversationID  string
	SenderID        string
	RecipientID     string
	SenderName      string
	SenderAvatarURL string
	Text            string
	CreatedAt       time.Time
}

func (e ActivityEvent) ThreadKey() string {
	if value := strings.TrimSpace(e.ConversationID); value != "" {
		return value
	}
	participants := make([]string, 0, 2)
	if value := strings.TrimSpace(e.SenderID); value != "" {
		participants = append(participants, value)
	}
	if value := strings.TrimSpace(e.RecipientID); value != "" {
		participants = append(participants, value)
	}
	sort.Strings(participants)
	participants = compactStrings(participants)
	if len(participants) == 0 {
		return strings.TrimSpace(e.ExternalID)
	}
	return "x-dm:" + strings.Join(participants, ":")
}

type InboundAdmissionRequest struct {
	WorkspaceID          string
	SocialAccountID      string
	AppMode              string
	OperationKey         string
	Source               string
	UpstreamResourceType string
	UpstreamResourceID   string
	Now                  time.Time
}

type InboundAdmission struct {
	Accepted    bool
	Suppressed  bool
	Duplicate   bool
	Decision    string
	PauseReason string
}

type IngestionResult struct {
	Admission InboundAdmission
	Item      InboxItem
	Inserted  bool
}

type IngestionStore interface {
	AccountForApp(context.Context, string, string) (InboxAccount, error)
	InsertInboxItem(context.Context, InboxItem) (InboxItem, bool, error)
}

type providerUserAccountStore interface {
	AccountsForProviderUser(context.Context, string, string) ([]InboxAccount, error)
}

type providerUserIngestionStore interface {
	IngestionStore
	providerUserAccountStore
}

type IngestionConfig struct {
	Store         IngestionStore
	Admit         func(context.Context, InboundAdmissionRequest) (InboundAdmission, error)
	AtomicProcess func(context.Context, InboundAdmissionRequest, InboxItem) (InboundAdmission, InboxItem, bool, error)
	DMsAvailable  func(context.Context, string) (bool, error)
	Notify        func(context.Context, string, string, InboxItem)
	Now           func() time.Time
}

type IngestionService struct {
	store        IngestionStore
	admit        func(context.Context, InboundAdmissionRequest) (InboundAdmission, error)
	atomic       func(context.Context, InboundAdmissionRequest, InboxItem) (InboundAdmission, InboxItem, bool, error)
	dmsAvailable func(context.Context, string) (bool, error)
	notify       func(context.Context, string, string, InboxItem)
	now          func() time.Time
}

func NewIngestionService(config IngestionConfig) *IngestionService {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &IngestionService{
		store:        config.Store,
		admit:        config.Admit,
		atomic:       config.AtomicProcess,
		dmsAvailable: config.DMsAvailable,
		notify:       config.Notify,
		now:          now,
	}
}

func (s *IngestionService) IngestStreamEvent(ctx context.Context, appClientID string, event StreamEvent) error {
	if s == nil || s.store == nil {
		return errors.New("X inbox ingestion store is not configured")
	}
	accountIDs := streamAccountIDs(event.MatchingRules)
	if len(accountIDs) == 0 ||
		strings.TrimSpace(event.Data.ID) == "" ||
		strings.TrimSpace(event.Data.AuthorID) == "" ||
		strings.TrimSpace(event.Data.CreatedAt) == "" ||
		parseRFC3339(event.Data.CreatedAt).IsZero() ||
		strings.TrimSpace(event.Data.ConversationID) == "" {
		return fmt.Errorf("%w: filtered stream event is missing id, account route, author, conversation, or timestamp", ErrMalformedEvent)
	}
	authorName, authorAvatar := streamAuthor(event)
	receivedAt := parseRFC3339(event.Data.CreatedAt)
	if receivedAt.IsZero() {
		receivedAt = s.now().UTC()
	}
	parentID := repliedToID(event.Data.ReferencedTweets)
	var errs []error
	for _, accountID := range accountIDs {
		account, err := s.store.AccountForApp(ctx, appClientID, accountID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !account.PlanAllowsInbox || !hasRequiredScopes(account.Scopes, "tweet.read", "users.read") {
			continue
		}
		item := InboxItem{
			SocialAccountID:  account.ID,
			WorkspaceID:      account.WorkspaceID,
			Source:           "x_reply",
			ExternalID:       event.Data.ID,
			ParentExternalID: parentID,
			AuthorName:       authorName,
			AuthorID:         event.Data.AuthorID,
			AuthorAvatarURL:  authorAvatar,
			Body:             event.Data.Text,
			IsOwn:            event.Data.AuthorID != "" && event.Data.AuthorID == account.ExternalUserID,
			ReceivedAt:       receivedAt,
			ThreadKey:        firstNonEmptyString(event.Data.ConversationID, event.Data.ID),
			ThreadStatus:     "open",
			Metadata: map[string]any{
				"conversation_id": event.Data.ConversationID,
				"permalink":       xPostPermalink(event.Data.ID),
				"reply_eligible":  true,
			},
		}
		if err := s.admitAndInsert(ctx, account, item, "post.mention.received", "filtered_stream"); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *IngestionService) IngestActivityEvent(ctx context.Context, appClientID string, event ActivityEvent) error {
	if s == nil || s.store == nil {
		return errors.New("X inbox ingestion store is not configured")
	}
	var account InboxAccount
	if strings.TrimSpace(event.AccountID) != "" {
		var err error
		account, err = s.store.AccountForApp(ctx, appClientID, event.AccountID)
		if err != nil {
			return err
		}
		providerUserID := strings.TrimSpace(event.ExternalUserID)
		if providerUserID == "" || providerUserID != strings.TrimSpace(account.ExternalAccountID) {
			return fmt.Errorf("%w: tagged provider route mismatch", ErrInboxAccountNotFound)
		}
	} else {
		providerStore, ok := s.store.(providerUserAccountStore)
		if !ok {
			return errors.New("X inbox provider user lookup is not configured")
		}
		providerUserID := firstNonEmptyString(event.ExternalUserID, event.RecipientID)
		accounts, err := providerStore.AccountsForProviderUser(ctx, appClientID, providerUserID)
		if err != nil {
			return err
		}
		switch len(accounts) {
		case 0:
			return ErrInboxAccountNotFound
		case 1:
			account = accounts[0]
		default:
			return fmt.Errorf("%w: provider user %q matched %d accounts", ErrInboxAccountAmbiguous, providerUserID, len(accounts))
		}
	}
	if err := s.requireDMFeatureEvaluator("x_dm"); err != nil {
		return err
	}
	if !account.PlanAllowsInbox || !hasRequiredScopes(account.Scopes, "dm.read", "users.read") {
		return nil
	}
	isOwn := event.SenderID != "" &&
		(event.SenderID == account.ExternalUserID || event.SenderID == account.ExternalAccountID)
	item := InboxItem{
		SocialAccountID:  account.ID,
		WorkspaceID:      account.WorkspaceID,
		Source:           "x_dm",
		ExternalID:       event.ExternalID,
		ParentExternalID: event.ThreadKey(),
		AuthorName:       event.SenderName,
		AuthorID:         event.SenderID,
		AuthorAvatarURL:  event.SenderAvatarURL,
		Body:             event.Text,
		IsOwn:            isOwn,
		ReceivedAt:       event.CreatedAt,
		ThreadKey:        event.ThreadKey(),
		ThreadStatus:     "open",
		Metadata: map[string]any{
			"conversation_id": event.ConversationID,
		},
	}
	if item.ReceivedAt.IsZero() {
		item.ReceivedAt = s.now().UTC()
	}
	return s.admitAndInsert(ctx, account, item, "dm.received", "activity")
}

func (s *IngestionService) admitAndInsert(
	ctx context.Context,
	account InboxAccount,
	item InboxItem,
	operationKey string,
	source string,
) error {
	_, err := s.admitAndInsertResult(ctx, account, item, operationKey, source, item.ReceivedAt)
	return err
}

func (s *IngestionService) IngestRecovery(
	ctx context.Context,
	account InboxAccount,
	item InboxItem,
	operationKey string,
	source string,
) (IngestionResult, error) {
	if s == nil || s.store == nil {
		return IngestionResult{}, errors.New("X inbox ingestion store is not configured")
	}
	if account.ID == "" || account.WorkspaceID == "" ||
		item.SocialAccountID != "" && item.SocialAccountID != account.ID ||
		item.WorkspaceID != "" && item.WorkspaceID != account.WorkspaceID {
		return IngestionResult{}, errors.New("X recovery item does not match the account workspace")
	}
	if err := s.requireDMFeatureEvaluator(item.Source); err != nil {
		return IngestionResult{}, err
	}
	item.SocialAccountID = account.ID
	item.WorkspaceID = account.WorkspaceID
	return s.admitAndInsertResult(ctx, account, item, operationKey, source, s.now().UTC())
}

func (s *IngestionService) admitAndInsertResult(
	ctx context.Context,
	account InboxAccount,
	item InboxItem,
	operationKey string,
	source string,
	admissionAt time.Time,
) (IngestionResult, error) {
	if err := s.requireDMFeatureEvaluator(item.Source); err != nil {
		return IngestionResult{}, err
	}
	if item.ExternalID == "" {
		return IngestionResult{}, nil
	}
	if item.Source == "x_dm" {
		available, err := s.dmsAvailable(ctx, account.WorkspaceID)
		if err != nil {
			return IngestionResult{}, err
		}
		if !available {
			return IngestionResult{}, nil
		}
	}
	request := InboundAdmissionRequest{
		WorkspaceID:          account.WorkspaceID,
		SocialAccountID:      account.ID,
		AppMode:              string(account.AppMode),
		OperationKey:         operationKey,
		Source:               source,
		UpstreamResourceType: item.Source,
		UpstreamResourceID:   item.ExternalID,
		Now:                  admissionAt,
	}
	if s.atomic != nil {
		admission, insertedItem, inserted, err := s.atomic(ctx, request, item)
		if err != nil {
			return IngestionResult{}, err
		}
		if (!admission.Accepted || admission.Suppressed) && inserted {
			return IngestionResult{}, errors.New("X atomic inbound processor inserted a suppressed event")
		}
		if inserted && s.notify != nil {
			s.notify(ctx, account.WorkspaceID, account.ExternalUserID, insertedItem)
		}
		return IngestionResult{Admission: admission, Item: insertedItem, Inserted: inserted}, nil
	}
	admission := InboundAdmission{Accepted: true}
	if s.admit != nil {
		var err error
		admission, err = s.admit(ctx, request)
		if err != nil {
			return IngestionResult{}, err
		}
		if !admission.Accepted || admission.Suppressed {
			return IngestionResult{Admission: admission}, nil
		}
	}
	insertedItem, inserted, err := s.store.InsertInboxItem(ctx, item)
	if err != nil {
		return IngestionResult{}, err
	}
	if inserted && s.notify != nil {
		s.notify(ctx, account.WorkspaceID, account.ExternalUserID, insertedItem)
	}
	return IngestionResult{Admission: admission, Item: insertedItem, Inserted: inserted}, nil
}

func (s *IngestionService) requireDMFeatureEvaluator(source string) error {
	if source == "x_dm" && (s == nil || s.dmsAvailable == nil) {
		return ErrDMFeatureNotConfigured
	}
	return nil
}

func ParseActivityEvents(body []byte) ([]ActivityEvent, error) {
	type currentActivityData struct {
		EventType string `json:"event_type"`
		Filter    struct {
			UserID string `json:"user_id"`
		} `json:"filter"`
		Tag       string          `json:"tag"`
		CreatedAt string          `json:"created_at"`
		Payload   json.RawMessage `json:"payload"`
	}
	var envelope struct {
		Data                json.RawMessage `json:"data"`
		CurrentEventTypeRaw json.RawMessage `json:"event_type"`
		CurrentPayloadRaw   json.RawMessage `json:"payload"`
		ForUserID           string          `json:"for_user_id"`
		DirectMessageEvents []struct {
			Type             string `json:"type"`
			ID               string `json:"id"`
			CreatedTimestamp string `json:"created_timestamp"`
			MessageCreate    struct {
				Target struct {
					RecipientID string `json:"recipient_id"`
				} `json:"target"`
				SenderID    string `json:"sender_id"`
				MessageData struct {
					Text string `json:"text"`
				} `json:"message_data"`
			} `json:"message_create"`
		} `json:"direct_message_events"`
		Users map[string]struct {
			Name                string `json:"name"`
			ScreenName          string `json:"screen_name"`
			ProfileImageURLHTTP string `json:"profile_image_url_https"`
		} `json:"users"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode X activity envelope: %w", err)
	}
	currentShape := len(envelope.Data) > 0 || len(envelope.CurrentEventTypeRaw) > 0 || len(envelope.CurrentPayloadRaw) > 0
	if currentShape {
		var data currentActivityData
		if len(envelope.Data) > 0 {
			if err := json.Unmarshal(envelope.Data, &data); err != nil {
				return nil, fmt.Errorf("%w: decode current X activity data: %v", ErrMalformedEvent, err)
			}
		}
		eventType := strings.TrimSpace(data.EventType)
		if eventType == "" {
			return nil, fmt.Errorf("%w: current X activity envelope is missing event_type", ErrMalformedEvent)
		}
		if eventType != "dm.received" {
			return []ActivityEvent{}, nil
		}
		if len(data.Payload) == 0 {
			return nil, fmt.Errorf("%w: dm.received payload is missing", ErrMalformedEvent)
		}
		var payload struct {
			ID               string   `json:"id"`
			DMEventID        string   `json:"dm_event_id"`
			DMConversationID string   `json:"dm_conversation_id"`
			ConversationID   string   `json:"conversation_id"`
			CreatedAt        string   `json:"created_at"`
			SenderID         string   `json:"sender_id"`
			RecipientID      string   `json:"recipient_id"`
			ParticipantIDs   []string `json:"participant_ids"`
			Text             string   `json:"text"`
		}
		if err := json.Unmarshal(data.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode X dm.received payload: %w", err)
		}
		recipientID := firstNonEmptyString(payload.RecipientID, data.Filter.UserID)
		if recipientID == "" {
			for _, participantID := range payload.ParticipantIDs {
				if participantID != payload.SenderID {
					recipientID = participantID
					break
				}
			}
		}
		event := ActivityEvent{
			AccountID:      activityAccountID(data.Tag),
			ExternalUserID: data.Filter.UserID,
			ExternalID:     firstNonEmptyString(payload.ID, payload.DMEventID),
			ConversationID: firstNonEmptyString(payload.DMConversationID, payload.ConversationID),
			SenderID:       payload.SenderID,
			RecipientID:    recipientID,
			Text:           payload.Text,
			CreatedAt:      parseRFC3339(firstNonEmptyString(payload.CreatedAt, data.CreatedAt)),
		}
		if event.AccountID == "" || event.ExternalID == "" || event.ExternalUserID == "" || event.SenderID == "" ||
			event.CreatedAt.IsZero() || (event.ConversationID == "" && event.RecipientID == "") {
			return nil, fmt.Errorf("%w: dm.received is missing account tag, event id, routing user, participants, conversation, or timestamp", ErrMalformedEvent)
		}
		return []ActivityEvent{event}, nil
	}

	events := make([]ActivityEvent, 0, len(envelope.DirectMessageEvents))
	for _, dm := range envelope.DirectMessageEvents {
		if dm.Type != "message_create" {
			continue
		}
		sender := envelope.Users[dm.MessageCreate.SenderID]
		event := ActivityEvent{
			ExternalUserID:  envelope.ForUserID,
			ExternalID:      dm.ID,
			SenderID:        dm.MessageCreate.SenderID,
			RecipientID:     dm.MessageCreate.Target.RecipientID,
			SenderName:      sender.Name,
			SenderAvatarURL: sender.ProfileImageURLHTTP,
			Text:            dm.MessageCreate.MessageData.Text,
			CreatedAt:       parseUnixMilliseconds(dm.CreatedTimestamp),
		}
		if event.ExternalUserID == "" || event.ExternalID == "" || event.SenderID == "" ||
			event.RecipientID == "" || event.CreatedAt.IsZero() {
			return nil, fmt.Errorf("%w: direct_message_events message_create is missing event id, routing user, participants, or timestamp", ErrMalformedEvent)
		}
		events = append(events, event)
	}
	return events, nil
}

func streamAccountIDs(rules []StreamRule) []string {
	var accountIDs []string
	for _, rule := range rules {
		if accountID := strings.TrimPrefix(rule.Tag, "unipost:x:account:"); accountID != rule.Tag && accountID != "" {
			accountIDs = append(accountIDs, accountID)
		}
	}
	sort.Strings(accountIDs)
	return compactStrings(accountIDs)
}

func activityAccountID(tag string) string {
	const prefix = "unipost:x:dm:"
	if !strings.HasPrefix(tag, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(tag, prefix))
}

func streamAuthor(event StreamEvent) (string, string) {
	var includes struct {
		Users []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			ProfileImageURL string `json:"profile_image_url"`
		} `json:"users"`
	}
	if json.Unmarshal(event.Includes, &includes) != nil {
		return "", ""
	}
	for _, user := range includes.Users {
		if user.ID == event.Data.AuthorID {
			return user.Name, user.ProfileImageURL
		}
	}
	return "", ""
}

func repliedToID(references []ReferencedTweet) string {
	for _, reference := range references {
		if reference.Type == "replied_to" {
			return reference.ID
		}
	}
	return ""
}

func parseRFC3339(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	return parsed
}

func parseUnixMilliseconds(value string) time.Time {
	milliseconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || milliseconds <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(milliseconds).UTC()
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func xPostPermalink(postID string) string {
	if postID == "" {
		return ""
	}
	return "https://x.com/i/web/status/" + postID
}

func hasRequiredScopes(scopes []string, required ...string) bool {
	have := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		have[strings.ToLower(strings.TrimSpace(scope))] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := have[scope]; !ok {
			return false
		}
	}
	return true
}
