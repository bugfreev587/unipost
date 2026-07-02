package loops

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
)

type LifecycleClient interface {
	Enabled() bool
	UpsertContact(ctx context.Context, contact Contact) error
	SendEvent(ctx context.Context, event Event) error
	SendTransactional(ctx context.Context, email TransactionalEmail) error
}

type DashboardUser struct {
	ID            string
	Email         string
	Name          string
	FirstName     string
	LastName      string
	WorkspaceID   string
	WorkspaceName string
	PlanID        string
	Event         string
}

type LifecycleEvent struct {
	UserID         string
	Email          string
	Name           string
	FirstName      string
	LastName       string
	WorkspaceID    string
	WorkspaceName  string
	PlanID         string
	EventName      string
	IdempotencyKey string
	Properties     map[string]any
	SkipContact    bool
}

type Options struct {
	Enabled          func(context.Context, DashboardUser) bool
	TransactionalIDs TransactionalIDs
	EmailAuditStore  EmailAuditStore
	EmailPolicy      EmailPolicy
}

type EmailPolicy interface {
	Prepare(context.Context, emailpolicy.Request) (emailpolicy.Decision, error)
}

type TransactionalIDs struct {
	PlanChanged                 string
	BillingPaymentFailed        string
	BillingPaymentRecovered     string
	BillingSubscriptionCanceled string
	AccountDisconnected         string
	AccountCanceled             string
	PostFailed                  string
}

type Syncer struct {
	client           LifecycleClient
	enabled          func(context.Context, DashboardUser) bool
	transactionalIDs TransactionalIDs
	emailAuditStore  EmailAuditStore
	emailPolicy      EmailPolicy
}

func NewSyncer(client LifecycleClient, opts Options) *Syncer {
	if opts.EmailAuditStore != nil && client != nil {
		client = NewAuditedClient(client, opts.EmailAuditStore)
	}
	enabled := opts.Enabled
	if enabled == nil {
		enabled = func(context.Context, DashboardUser) bool {
			return true
		}
	}
	return &Syncer{
		client:           client,
		enabled:          enabled,
		transactionalIDs: opts.TransactionalIDs,
		emailAuditStore:  opts.EmailAuditStore,
		emailPolicy:      opts.EmailPolicy,
	}
}

func (s *Syncer) SyncDashboardUser(ctx context.Context, user DashboardUser) error {
	if s == nil || s.client == nil || !s.client.Enabled() {
		return nil
	}
	if strings.TrimSpace(user.Email) == "" {
		return nil
	}
	if s.enabled != nil && !s.enabled(ctx, user) {
		return nil
	}

	props := dashboardUserProperties(user)
	if err := s.client.UpsertContact(ctx, Contact{
		Email:      user.Email,
		UserID:     user.ID,
		FirstName:  firstNonEmpty(user.FirstName, firstNameFromFullName(user.Name)),
		LastName:   firstNonEmpty(user.LastName, lastNameFromFullName(user.Name)),
		Source:     "unipost_dashboard",
		UserGroup:  user.PlanID,
		Properties: props,
	}); err != nil {
		slog.Warn("loops: contact sync failed", "user_id", user.ID, "email", user.Email, "event", user.Event, "error", err)
		return nil
	}

	if user.Event == "user.created" {
		if err := s.client.SendEvent(ctx, Event{
			Email:          user.Email,
			UserID:         user.ID,
			Name:           "user_signed_up",
			IdempotencyKey: "clerk_user.created:" + user.ID,
			Properties:     props,
		}); err != nil {
			slog.Warn("loops: signup event failed", "user_id", user.ID, "email", user.Email, "error", err)
		}
	}
	return nil
}

func (s *Syncer) SendLifecycleEvent(ctx context.Context, event LifecycleEvent) error {
	if s == nil || s.client == nil || !s.client.Enabled() {
		return nil
	}
	if strings.TrimSpace(event.Email) == "" || strings.TrimSpace(event.EventName) == "" {
		return nil
	}

	user := DashboardUser{
		ID:            event.UserID,
		Email:         event.Email,
		Name:          event.Name,
		FirstName:     event.FirstName,
		LastName:      event.LastName,
		WorkspaceID:   event.WorkspaceID,
		WorkspaceName: event.WorkspaceName,
		PlanID:        event.PlanID,
		Event:         event.EventName,
	}
	if s.enabled != nil && !s.enabled(ctx, user) {
		return nil
	}

	props := lifecycleEventProperties(event)
	if !event.SkipContact {
		if err := s.client.UpsertContact(ctx, Contact{
			Email:      event.Email,
			UserID:     event.UserID,
			FirstName:  firstNonEmpty(event.FirstName, firstNameFromFullName(event.Name)),
			LastName:   firstNonEmpty(event.LastName, lastNameFromFullName(event.Name)),
			Source:     "unipost_dashboard",
			UserGroup:  event.PlanID,
			Properties: props,
		}); err != nil {
			slog.Warn("loops: lifecycle contact sync failed", "user_id", event.UserID, "email", event.Email, "event", event.EventName, "error", err)
			return nil
		}
	}

	if transactionalID := s.transactionalIDFor(event.EventName); transactionalID != "" {
		dataVariables := lifecycleTransactionalDataVariables(event, props)
		audit := lifecycleTransactionalAudit(event)
		if s.emailPolicy != nil && strings.TrimSpace(audit.EventKey) != "" {
			decision, err := s.emailPolicy.Prepare(ctx, emailpolicy.Request{
				EventKey:      audit.EventKey,
				UserID:        event.UserID,
				Email:         event.Email,
				DataVariables: dataVariables,
			})
			if err != nil {
				slog.Warn("loops: lifecycle transactional email policy failed", "user_id", event.UserID, "email", event.Email, "event", event.EventName, "error", err)
				return nil
			}
			dataVariables = decision.DataVariables
			if !decision.ShouldSend {
				s.createSkippedTransactionalAudit(ctx, TransactionalEmail{
					TransactionalID: transactionalID,
					Email:           event.Email,
					UserID:          event.UserID,
					IdempotencyKey:  event.IdempotencyKey,
					DataVariables:   dataVariables,
					Audit:           audit,
				}, decision.SkipReason)
				return nil
			}
		}
		if err := s.client.SendTransactional(ctx, TransactionalEmail{
			TransactionalID: transactionalID,
			Email:           event.Email,
			UserID:          event.UserID,
			IdempotencyKey:  event.IdempotencyKey,
			DataVariables:   dataVariables,
			Audit:           audit,
		}); err != nil {
			slog.Warn("loops: lifecycle transactional email failed", "user_id", event.UserID, "email", event.Email, "event", event.EventName, "error", err)
		}
		return nil
	}

	if err := s.client.SendEvent(ctx, Event{
		Email:          event.Email,
		UserID:         event.UserID,
		Name:           event.EventName,
		IdempotencyKey: event.IdempotencyKey,
		Properties:     props,
	}); err != nil {
		slog.Warn("loops: lifecycle event failed", "user_id", event.UserID, "email", event.Email, "event", event.EventName, "error", err)
	}
	return nil
}

func (s *Syncer) transactionalIDFor(eventName string) string {
	if s == nil {
		return ""
	}
	switch strings.TrimSpace(eventName) {
	case "plan_changed":
		return strings.TrimSpace(s.transactionalIDs.PlanChanged)
	case "billing_payment_failed":
		return strings.TrimSpace(s.transactionalIDs.BillingPaymentFailed)
	case "billing_payment_recovered":
		return strings.TrimSpace(s.transactionalIDs.BillingPaymentRecovered)
	case "billing_subscription_canceled":
		return strings.TrimSpace(s.transactionalIDs.BillingSubscriptionCanceled)
	case "account_disconnected":
		return strings.TrimSpace(s.transactionalIDs.AccountDisconnected)
	case "user_account_canceled":
		return strings.TrimSpace(s.transactionalIDs.AccountCanceled)
	case "post_failed":
		return strings.TrimSpace(s.transactionalIDs.PostFailed)
	default:
		return ""
	}
}

func (s *Syncer) createSkippedTransactionalAudit(ctx context.Context, email TransactionalEmail, reason string) {
	if s == nil || s.emailAuditStore == nil || strings.TrimSpace(email.Audit.EventKey) == "" {
		return
	}
	provider := strings.TrimSpace(email.Audit.Provider)
	if provider == "" {
		provider = "loops"
	}
	if strings.TrimSpace(reason) == "" {
		reason = "skipped"
	}
	if _, err := s.emailAuditStore.CreateSkippedEmailSendAttempt(ctx, EmailSendAttempt{
		EventKey:           email.Audit.EventKey,
		RecipientUserID:    email.UserID,
		RecipientEmail:     email.Email,
		WorkspaceID:        email.Audit.WorkspaceID,
		Provider:           provider,
		ProviderTemplateID: email.TransactionalID,
		IdempotencyKey:     email.IdempotencyKey,
		DeliveryClass:      email.Audit.DeliveryClass,
		SubjectSnapshot:    email.Audit.Subject,
		DataVariables:      email.DataVariables,
		TriggerSource:      email.Audit.TriggerSource,
		TriggerReferenceID: email.Audit.TriggerReferenceID,
	}, reason); err != nil {
		slog.Warn("loops: skipped email audit create failed", "event_key", email.Audit.EventKey, "email", email.Email, "reason", reason, "error", err)
	}
}

func dashboardUserProperties(user DashboardUser) map[string]any {
	props := map[string]any{
		"source": "unipost_dashboard",
	}
	addProp(props, "workspace_id", user.WorkspaceID)
	addProp(props, "workspace_name", user.WorkspaceName)
	addProp(props, "plan_id", user.PlanID)
	addProp(props, "clerk_user_id", user.ID)
	return props
}

func lifecycleEventProperties(event LifecycleEvent) map[string]any {
	props := map[string]any{
		"source": "unipost_dashboard",
	}
	for key, value := range event.Properties {
		props[key] = value
	}
	addProp(props, "workspace_id", event.WorkspaceID)
	addProp(props, "workspace_name", event.WorkspaceName)
	addProp(props, "plan_id", event.PlanID)
	addProp(props, "clerk_user_id", event.UserID)
	return props
}

func lifecycleTransactionalDataVariables(event LifecycleEvent, props map[string]any) map[string]any {
	vars := map[string]any{}
	switch strings.TrimSpace(event.EventName) {
	case "plan_changed":
		addTransactionalValue(vars, "workspace_name", props["workspace_name"])
		addTransactionalValue(vars, "old_plan_id", props["old_plan_id"])
		addTransactionalValue(vars, "new_plan_id", props["new_plan_id"])
		addTransactionalValue(vars, "change_type", props["change_type"])
		addTransactionalValue(vars, "billing_url", props["billing_url"])
	case "billing_payment_failed":
		addTransactionalValue(vars, "workspace_name", props["workspace_name"])
		addTransactionalValue(vars, "plan_id", props["plan_id"])
		addTransactionalValue(vars, "billing_url", props["billing_url"])
		addTransactionalValue(vars, "retry_message", props["retry_message"])
		addTransactionalValue(vars, "attempt_count", props["attempt_count"])
		addTransactionalValue(vars, "next_payment_attempt", props["next_payment_attempt"])
	case "billing_payment_recovered":
		addTransactionalValue(vars, "workspace_name", props["workspace_name"])
		addTransactionalValue(vars, "plan_id", props["plan_id"])
		addTransactionalValue(vars, "billing_url", props["billing_url"])
	case "billing_subscription_canceled":
		addTransactionalValue(vars, "workspace_name", props["workspace_name"])
		addTransactionalValue(vars, "plan_id", props["plan_id"])
		addTransactionalValue(vars, "effective_at", props["effective_at"])
		addTransactionalValue(vars, "billing_url", props["billing_url"])
	case "account_disconnected":
		addTransactionalValue(vars, "workspace_name", props["workspace_name"])
		addTransactionalValue(vars, "platform", props["platform"])
		addTransactionalValue(vars, "account_name", props["account_name"])
		addTransactionalValue(vars, "reconnect_url", props["reconnect_url"])
		addTransactionalValue(vars, "reason", props["reason"])
	case "post_failed":
		addTransactionalValue(vars, "workspace_name", props["workspace_name"])
		addTransactionalValue(vars, "post_id", props["post_id"])
		addTransactionalValue(vars, "platform", props["platform"])
		addTransactionalValue(vars, "error_code", props["error_code"])
		addTransactionalValue(vars, "dashboard_url", props["dashboard_url"])
		addTransactionalValue(vars, "retriable", props["retriable"])
	case "user_account_canceled":
		addTransactionalValue(vars, "canceled_at", props["canceled_at"])
	}
	return vars
}

func addTransactionalValue(vars map[string]any, key string, value any) {
	if strings.TrimSpace(key) == "" {
		return
	}
	if normalized, ok := transactionalValue(value); ok {
		if s, ok := normalized.(string); ok && strings.TrimSpace(s) == "" {
			return
		}
		vars[key] = normalized
	}
}

func transactionalValue(value any) (any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case string:
		return v, true
	case bool:
		return fmt.Sprintf("%t", v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return fmt.Sprint(v), true
	}
}

func addProp(props map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		props[key] = value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNameFromFullName(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func lastNameFromFullName(name string) string {
	parts := strings.Fields(name)
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], " ")
}
