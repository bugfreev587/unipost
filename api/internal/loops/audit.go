package loops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/emailregistry"
)

const emailAuditFinalizationTimeout = 5 * time.Second

type EmailAudit struct {
	EventKey           string
	WorkspaceID        string
	Provider           string
	DeliveryClass      string
	TriggerSource      string
	TriggerReferenceID string
	Subject            string
	// AttemptIdempotencyKey identifies one local network attempt in the
	// audit ledger. It is intentionally separate from TransactionalEmail's
	// stable provider idempotency key.
	AttemptIdempotencyKey string
}

type EmailSendAttempt struct {
	EventKey           string
	RecipientUserID    string
	RecipientEmail     string
	WorkspaceID        string
	Provider           string
	ProviderTemplateID string
	IdempotencyKey     string
	DeliveryClass      string
	SubjectSnapshot    string
	DataVariables      map[string]any
	TriggerSource      string
	TriggerReferenceID string
}

type EmailSendAttemptRecord struct {
	ID string
}

// EmailAuditFinalizationError means the provider request completed with
// ProviderError, but its audit row could not be moved out of pending. Callers
// must leave linked local work recoverable until reconciliation finalizes both
// records; this is deliberately not a SendOutcomeUnknownError.
type EmailAuditFinalizationError struct {
	ProviderError error
	AuditError    error
}

func (e *EmailAuditFinalizationError) Error() string {
	return fmt.Sprintf("loops: provider result %v; finalize failed email audit: %v", e.ProviderError, e.AuditError)
}

func (e *EmailAuditFinalizationError) Unwrap() error {
	return e.AuditError
}

func IsEmailAuditFinalizationError(err error) bool {
	var finalizationErr *EmailAuditFinalizationError
	return errors.As(err, &finalizationErr)
}

type EmailAuditStore interface {
	CreateEmailSendAttempt(ctx context.Context, attempt EmailSendAttempt) (EmailSendAttemptRecord, error)
	CreateSkippedEmailSendAttempt(ctx context.Context, attempt EmailSendAttempt, reason string) (EmailSendAttemptRecord, error)
	MarkEmailSendAttemptSent(ctx context.Context, id string) error
	MarkEmailSendAttemptFailed(ctx context.Context, id, reason string) error
}

type AuditedClient struct {
	client LifecycleClient
	store  EmailAuditStore
}

func NewAuditedClient(client LifecycleClient, store EmailAuditStore) *AuditedClient {
	return &AuditedClient{client: client, store: store}
}

func (c *AuditedClient) Enabled() bool {
	return c != nil && c.client != nil && c.client.Enabled()
}

func (c *AuditedClient) UpsertContact(ctx context.Context, contact Contact) error {
	if c == nil || c.client == nil {
		return errors.New("loops: audited client is not configured")
	}
	return c.client.UpsertContact(ctx, contact)
}

func (c *AuditedClient) SendEvent(ctx context.Context, event Event) error {
	if c == nil || c.client == nil {
		return errors.New("loops: audited client is not configured")
	}
	return c.client.SendEvent(ctx, event)
}

func (c *AuditedClient) SendTransactional(ctx context.Context, email TransactionalEmail) error {
	if c == nil || c.client == nil {
		return errors.New("loops: audited client is not configured")
	}
	if c.store == nil || strings.TrimSpace(email.Audit.EventKey) == "" {
		return c.client.SendTransactional(ctx, email)
	}

	_, err := c.SendTransactionalWithAttempt(ctx, email, nil)
	return err
}

// SendTransactionalWithAttempt is the strict campaign send path. It creates
// a durable audit row, lets the caller link that row to its atomic local send
// claim, and only then permits the provider request. A failure in either
// local gate prevents the network call.
func (c *AuditedClient) SendTransactionalWithAttempt(
	ctx context.Context,
	email TransactionalEmail,
	beforeSend func(context.Context, string) error,
) (EmailSendAttemptRecord, error) {
	if c == nil || c.client == nil || c.store == nil || strings.TrimSpace(email.Audit.EventKey) == "" {
		return EmailSendAttemptRecord{}, errors.New("loops: strict audited client is not configured")
	}
	record, err := c.createAttempt(ctx, email)
	if err != nil {
		return EmailSendAttemptRecord{}, err
	}
	if beforeSend != nil {
		if linkErr := beforeSend(ctx, record.ID); linkErr != nil {
			if auditErr := c.store.MarkEmailSendAttemptFailed(ctx, record.ID, linkErr.Error()); auditErr != nil {
				slog.Warn("loops: email audit linkage failure update failed", "attempt_id", record.ID, "error", auditErr)
			}
			return record, linkErr
		}
	}
	err = c.client.SendTransactional(ctx, email)
	if err != nil {
		finalizeCtx, cancelFinalize := context.WithTimeout(context.WithoutCancel(ctx), emailAuditFinalizationTimeout)
		auditErr := c.store.MarkEmailSendAttemptFailed(finalizeCtx, record.ID, err.Error())
		cancelFinalize()
		if auditErr != nil {
			slog.Warn("loops: email audit failure update failed", "attempt_id", record.ID, "error", auditErr)
			return record, &EmailAuditFinalizationError{ProviderError: err, AuditError: auditErr}
		}
		return record, err
	}
	finalizeCtx, cancelFinalize := context.WithTimeout(context.WithoutCancel(ctx), emailAuditFinalizationTimeout)
	auditErr := c.store.MarkEmailSendAttemptSent(finalizeCtx, record.ID)
	cancelFinalize()
	if auditErr != nil {
		slog.Warn("loops: email audit sent update failed", "attempt_id", record.ID, "error", auditErr)
	}
	return record, nil
}

func (c *AuditedClient) createAttempt(ctx context.Context, email TransactionalEmail) (EmailSendAttemptRecord, error) {
	provider := strings.TrimSpace(email.Audit.Provider)
	if provider == "" {
		provider = "loops"
	}
	auditIdempotencyKey := strings.TrimSpace(email.Audit.AttemptIdempotencyKey)
	if auditIdempotencyKey == "" {
		auditIdempotencyKey = email.IdempotencyKey
	}
	record, err := c.store.CreateEmailSendAttempt(ctx, EmailSendAttempt{
		EventKey:           email.Audit.EventKey,
		RecipientUserID:    email.UserID,
		RecipientEmail:     email.Email,
		WorkspaceID:        email.Audit.WorkspaceID,
		Provider:           provider,
		ProviderTemplateID: email.TransactionalID,
		IdempotencyKey:     auditIdempotencyKey,
		DeliveryClass:      email.Audit.DeliveryClass,
		SubjectSnapshot:    email.Audit.Subject,
		DataVariables:      email.DataVariables,
		TriggerSource:      email.Audit.TriggerSource,
		TriggerReferenceID: email.Audit.TriggerReferenceID,
	})
	if err != nil {
		slog.Warn("loops: email audit create failed", "event_key", email.Audit.EventKey, "email", email.Email, "error", err)
		return EmailSendAttemptRecord{}, err
	}
	return record, nil
}

func lifecycleTransactionalAudit(event LifecycleEvent) EmailAudit {
	registryEvent, ok := emailregistry.LookupByLoopsEventName(strings.TrimSpace(event.EventName))
	eventKey := lifecycleEventKey(event.EventName)
	deliveryClass := lifecycleDeliveryClass(event.EventName)
	if ok {
		eventKey = registryEvent.Key
		deliveryClass = string(registryEvent.DeliveryClass)
	}
	return EmailAudit{
		EventKey:           eventKey,
		WorkspaceID:        event.WorkspaceID,
		Provider:           "loops",
		DeliveryClass:      deliveryClass,
		TriggerSource:      lifecycleTriggerSource(event.EventName),
		TriggerReferenceID: lifecycleTriggerReferenceID(event),
	}
}

func lifecycleEventKey(eventName string) string {
	switch strings.TrimSpace(eventName) {
	case "plan_changed":
		return "email.billing.plan_changed.v1"
	case "billing_payment_failed":
		return "email.billing.payment_failed.v1"
	case "billing_payment_recovered":
		return "email.billing.payment_recovered.v1"
	case "billing_subscription_canceled":
		return "email.billing.subscription_canceled.v1"
	case "account_disconnected":
		return "email.account.disconnected.v1"
	case "user_account_canceled":
		return "email.user.account_canceled.v1"
	case "post_failed":
		return "email.post.failed.v1"
	default:
		return ""
	}
}

func lifecycleDeliveryClass(eventName string) string {
	switch strings.TrimSpace(eventName) {
	case "account_disconnected", "post_failed":
		return "service_alert"
	default:
		return "critical_transactional"
	}
}

func lifecycleTriggerSource(eventName string) string {
	switch strings.TrimSpace(eventName) {
	case "plan_changed", "billing_trial_ending", "billing_payment_failed", "billing_payment_recovered", "billing_subscription_canceled":
		return "stripe_webhook"
	case "account_disconnected", "post_failed":
		return "worker"
	case "user_account_canceled":
		return "handler"
	default:
		return ""
	}
}

func lifecycleTriggerReferenceID(event LifecycleEvent) string {
	switch strings.TrimSpace(event.EventName) {
	case "post_failed":
		return stringProp(event.Properties, "post_id")
	case "account_disconnected":
		return stringProp(event.Properties, "social_account_id")
	default:
		return strings.TrimSpace(event.IdempotencyKey)
	}
}

func stringProp(props map[string]any, key string) string {
	if props == nil {
		return ""
	}
	return strings.TrimSpace(toString(props[key]))
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
