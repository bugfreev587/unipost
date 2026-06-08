package errortriage

import (
	"context"
	"errors"
	"strings"
)

type TransactionalEmail struct {
	TransactionalID string
	Email           string
	UserID          string
	IdempotencyKey  string
	DataVariables   map[string]any
}

type TransactionalSender interface {
	Enabled() bool
	SendTransactional(context.Context, TransactionalEmail) error
}

type EmailSendStore interface {
	LoadEmailSendContext(ctx context.Context, itemID, recipientID string) (EmailSendContext, error)
	CreateEmailSendAttempt(ctx context.Context, params CreateEmailSendAttemptParams) (EmailSendAttempt, error)
	MarkEmailSendSucceeded(ctx context.Context, attemptID, recipientID string) error
	MarkEmailSendFailed(ctx context.Context, attemptID, recipientID, message string) error
}

type EmailSendContext struct {
	ItemID            string
	RecipientID       string
	RecipientScopeKey string
	RecipientUserID   string
	CurrentEmail      string
	Item              ItemState
	Recipient         RecipientState
	Draft             EmailDraft
	CTAURL            string
	DraftVersion      int
}

type CreateEmailSendAttemptParams struct {
	ItemID            string
	RecipientID       string
	RecipientScopeKey string
	RecipientUserID   string
	RecipientEmail    string
	TransactionalID   string
	IdempotencyKey    string
	Subject           string
	Body              string
	CTAURL            string
	SentByAdminID     string
}

type EmailSendAttempt struct {
	ID            string `json:"id"`
	AttemptNumber int    `json:"attempt_number"`
}

type EmailSendResult struct {
	AttemptID       string `json:"attempt_id"`
	AttemptNumber   int    `json:"attempt_number"`
	IdempotencyKey  string `json:"idempotency_key"`
	RecipientEmail  string `json:"recipient_email"`
	RecipientUserID string `json:"recipient_user_id"`
}

type EmailSendService struct {
	store           EmailSendStore
	sender          TransactionalSender
	transactionalID string
}

func NewEmailSendService(store EmailSendStore, sender TransactionalSender, transactionalID string) *EmailSendService {
	return &EmailSendService{
		store:           store,
		sender:          sender,
		transactionalID: strings.TrimSpace(transactionalID),
	}
}

func (s *EmailSendService) Configured() bool {
	return s != nil && s.store != nil && s.sender != nil && s.sender.Enabled() && s.transactionalID != ""
}

func (s *EmailSendService) SendRecipient(ctx context.Context, itemID, recipientID, adminUserID string) (EmailSendResult, error) {
	if s == nil || s.store == nil {
		return EmailSendResult{}, errors.New("error triage email sender is not configured")
	}
	sendCtx, err := s.store.LoadEmailSendContext(ctx, itemID, recipientID)
	if err != nil {
		return EmailSendResult{}, err
	}
	ok, reason := CanSendRecipient(sendCtx.Item, sendCtx.Recipient, s.Configured(), sendCtx.CurrentEmail)
	if !ok {
		return EmailSendResult{}, errors.New(reason)
	}
	draftVersion := sendCtx.DraftVersion
	if draftVersion <= 0 {
		draftVersion = 1
	}
	idempotencyKey := SendIdempotencyKey(sendCtx.ItemID, sendCtx.RecipientScopeKey, draftVersion)
	attempt, err := s.store.CreateEmailSendAttempt(ctx, CreateEmailSendAttemptParams{
		ItemID:            sendCtx.ItemID,
		RecipientID:       sendCtx.RecipientID,
		RecipientScopeKey: sendCtx.RecipientScopeKey,
		RecipientUserID:   sendCtx.RecipientUserID,
		RecipientEmail:    sendCtx.CurrentEmail,
		TransactionalID:   s.transactionalID,
		IdempotencyKey:    idempotencyKey,
		Subject:           sendCtx.Draft.Subject,
		Body:              sendCtx.Draft.Body,
		CTAURL:            sendCtx.CTAURL,
		SentByAdminID:     strings.TrimSpace(adminUserID),
	})
	if err != nil {
		return EmailSendResult{}, err
	}

	err = s.sender.SendTransactional(ctx, TransactionalEmail{
		TransactionalID: s.transactionalID,
		Email:           sendCtx.CurrentEmail,
		UserID:          sendCtx.RecipientUserID,
		IdempotencyKey:  idempotencyKey,
		DataVariables: map[string]any{
			"subject": sendCtx.Draft.Subject,
			"body":    sendCtx.Draft.Body,
			"cta_url": sendCtx.CTAURL,
		},
	})
	if err != nil {
		_ = s.store.MarkEmailSendFailed(ctx, attempt.ID, sendCtx.RecipientID, err.Error())
		return EmailSendResult{}, err
	}
	if err := s.store.MarkEmailSendSucceeded(ctx, attempt.ID, sendCtx.RecipientID); err != nil {
		return EmailSendResult{}, err
	}
	return EmailSendResult{
		AttemptID:       attempt.ID,
		AttemptNumber:   attempt.AttemptNumber,
		IdempotencyKey:  idempotencyKey,
		RecipientEmail:  sendCtx.CurrentEmail,
		RecipientUserID: sendCtx.RecipientUserID,
	}, nil
}
