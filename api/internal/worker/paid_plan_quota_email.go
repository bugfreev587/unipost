package worker

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
	"github.com/xiaoboyu/unipost-api/internal/loops"
)

var paidQuotaRetryDelays = []time.Duration{
	5 * time.Minute,
	time.Hour,
	6 * time.Hour,
}

type PaidQuotaDelivery struct {
	ID               string
	WorkspaceID      string
	WorkspaceName    string
	UserID           string
	OwnerEmail       string
	OwnerName        string
	PlanID           string
	PlanName         string
	Period           string
	ThresholdPercent int
	Severity         string
	EventKey         string
	TransactionalID  string
	IdempotencyKey   string
	CompletedUsage   int
	ScheduledUsage   int
	QuotaHoldUsage   int
	EffectiveUsage   int
	PostLimit        int
	AttemptCount     int
}

type PaidQuotaRetry struct {
	ID            string
	NextAttemptAt time.Time
	LastError     string
}

type PaidQuotaDeliveryStore interface {
	Claim(ctx context.Context, limit int) ([]PaidQuotaDelivery, error)
	MarkSent(ctx context.Context, id string) error
	MarkRetry(ctx context.Context, retry PaidQuotaRetry) error
	MarkFailed(ctx context.Context, id, lastError string) error
	MarkPreferenceDisabled(ctx context.Context, id string) error
	ReconciliationWorkspaces(ctx context.Context) ([]string, error)
}

type PaidQuotaSender interface {
	SendTransactional(ctx context.Context, email loops.TransactionalEmail) error
}

type PaidQuotaEvaluator interface {
	Evaluate(ctx context.Context, workspaceID, period string) error
}

type PaidPlanQuotaEmailWorker struct {
	store      PaidQuotaDeliveryStore
	sender     PaidQuotaSender
	evaluator  PaidQuotaEvaluator
	appBaseURL string
	policy     interface {
		Prepare(context.Context, emailpolicy.Request) (emailpolicy.Decision, error)
	}
	now func() time.Time
}

func NewPaidPlanQuotaEmailWorker(
	store PaidQuotaDeliveryStore,
	sender PaidQuotaSender,
	evaluator PaidQuotaEvaluator,
	now func() time.Time,
) *PaidPlanQuotaEmailWorker {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &PaidPlanQuotaEmailWorker{
		store:      store,
		sender:     sender,
		evaluator:  evaluator,
		appBaseURL: "https://app.unipost.dev",
		now:        now,
	}
}

func (w *PaidPlanQuotaEmailWorker) SetAppBaseURL(appBaseURL string) *PaidPlanQuotaEmailWorker {
	if w == nil {
		return w
	}
	appBaseURL = strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if appBaseURL != "" {
		w.appBaseURL = appBaseURL
	}
	return w
}

func (w *PaidPlanQuotaEmailWorker) SetEmailPolicy(policy interface {
	Prepare(context.Context, emailpolicy.Request) (emailpolicy.Decision, error)
}) *PaidPlanQuotaEmailWorker {
	w.policy = policy
	return w
}

func (w *PaidPlanQuotaEmailWorker) ProcessOnce(ctx context.Context) error {
	if w == nil || w.store == nil || w.sender == nil {
		return nil
	}
	deliveries, err := w.store.Claim(ctx, 20)
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		variables := paidQuotaEmailVariables(delivery, w.appBaseURL)
		if w.policy != nil {
			decision, err := w.policy.Prepare(ctx, emailpolicy.Request{
				EventKey:      delivery.EventKey,
				UserID:        delivery.UserID,
				Email:         delivery.OwnerEmail,
				DataVariables: variables,
			})
			if err != nil {
				if markErr := w.handleFailure(ctx, delivery, err); markErr != nil {
					return markErr
				}
				continue
			}
			variables = decision.DataVariables
			if !decision.ShouldSend && decision.SkipReason == emailpolicy.SkipReasonPreferenceDisabled {
				if err := w.store.MarkPreferenceDisabled(ctx, delivery.ID); err != nil {
					return err
				}
				continue
			}
		}
		err := w.sender.SendTransactional(ctx, loops.TransactionalEmail{
			TransactionalID: delivery.TransactionalID,
			Email:           delivery.OwnerEmail,
			UserID:          delivery.UserID,
			IdempotencyKey:  delivery.IdempotencyKey,
			DataVariables:   variables,
			Audit: loops.EmailAudit{
				EventKey:           delivery.EventKey,
				WorkspaceID:        delivery.WorkspaceID,
				Provider:           "loops",
				DeliveryClass:      "service_alert",
				TriggerSource:      "paid_quota_worker",
				TriggerReferenceID: delivery.ID,
				Subject:            fmt.Sprint(variables["subject"]),
			},
		})
		if err != nil {
			if markErr := w.handleFailure(ctx, delivery, err); markErr != nil {
				return markErr
			}
			continue
		}
		if err := w.store.MarkSent(ctx, delivery.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *PaidPlanQuotaEmailWorker) handleFailure(ctx context.Context, delivery PaidQuotaDelivery, sendErr error) error {
	reason := strings.TrimSpace(sendErr.Error())
	if delivery.AttemptCount >= 4 {
		return w.store.MarkFailed(ctx, delivery.ID, reason)
	}
	delayIndex := delivery.AttemptCount - 1
	if delayIndex < 0 {
		delayIndex = 0
	}
	if delayIndex >= len(paidQuotaRetryDelays) {
		return w.store.MarkFailed(ctx, delivery.ID, reason)
	}
	return w.store.MarkRetry(ctx, PaidQuotaRetry{
		ID:            delivery.ID,
		NextAttemptAt: w.now().UTC().Add(paidQuotaRetryDelays[delayIndex]),
		LastError:     reason,
	})
}

func (w *PaidPlanQuotaEmailWorker) ReconcileCurrentPeriod(ctx context.Context) error {
	if w == nil || w.store == nil || w.evaluator == nil {
		return nil
	}
	workspaceIDs, err := w.store.ReconciliationWorkspaces(ctx)
	if err != nil {
		return err
	}
	period := w.now().UTC().Format("2006-01")
	for _, workspaceID := range workspaceIDs {
		if err := w.evaluator.Evaluate(ctx, workspaceID, period); err != nil {
			return err
		}
	}
	return nil
}

func (w *PaidPlanQuotaEmailWorker) Start(ctx context.Context) {
	poll := time.NewTicker(15 * time.Second)
	defer poll.Stop()
	reconcileTimer := time.NewTimer(time.Until(nextPaidQuotaReconciliation(time.Now().UTC())))
	defer reconcileTimer.Stop()

	_ = w.ReconcileCurrentPeriod(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			_ = w.ProcessOnce(ctx)
		case <-reconcileTimer.C:
			_ = w.ReconcileCurrentPeriod(ctx)
			reconcileTimer.Reset(time.Until(nextPaidQuotaReconciliation(time.Now().UTC())))
		}
	}
}

func nextPaidQuotaReconciliation(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 15, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func paidQuotaEmailVariables(delivery PaidQuotaDelivery, appBaseURL string) map[string]any {
	appBaseURL = strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if appBaseURL == "" {
		appBaseURL = "https://app.unipost.dev"
	}
	resetsAt := ""
	if periodStart, err := time.Parse("2006-01", delivery.Period); err == nil {
		resetsAt = periodStart.AddDate(0, 1, 0).UTC().Format(time.RFC3339)
	}
	percentage := 0.0
	if delivery.PostLimit > 0 {
		percentage = float64(delivery.EffectiveUsage) / float64(delivery.PostLimit) * 100
	}
	subjectPrefix := "UniPost quota warning"
	headline := "Your monthly usage is rising"
	if delivery.ThresholdPercent >= 100 {
		subjectPrefix = "UniPost quota alert"
		headline = "Monthly scheduling capacity reached"
	}
	if delivery.ThresholdPercent >= 120 {
		subjectPrefix = "Critical UniPost quota alert"
		headline = "Your workspace is significantly over quota"
	}
	return map[string]any{
		"subject":                fmt.Sprintf("%s: %d%%", subjectPrefix, delivery.ThresholdPercent),
		"preview_text":           fmt.Sprintf("%s has reached %d%% of its monthly quota.", delivery.WorkspaceName, delivery.ThresholdPercent),
		"headline":               headline,
		"recipient_name":         firstNonEmptyWorker(delivery.OwnerName, "there"),
		"workspace_name":         firstNonEmptyWorker(delivery.WorkspaceName, "your workspace"),
		"plan_name":              firstNonEmptyWorker(delivery.PlanName, delivery.PlanID),
		"threshold_percent":      strconv.Itoa(delivery.ThresholdPercent),
		"severity":               delivery.Severity,
		"completed_posts":        strconv.Itoa(delivery.CompletedUsage),
		"scheduled_posts":        strconv.Itoa(delivery.ScheduledUsage),
		"quota_hold_posts":       strconv.Itoa(delivery.QuotaHoldUsage),
		"effective_usage":        strconv.Itoa(delivery.EffectiveUsage),
		"post_limit":             strconv.Itoa(delivery.PostLimit),
		"effective_percentage":   strconv.FormatFloat(math.Round(percentage*100)/100, 'f', -1, 64),
		"remaining_capacity":     strconv.Itoa(max(delivery.PostLimit-delivery.EffectiveUsage, 0)),
		"period":                 delivery.Period,
		"resets_at":              resetsAt,
		"scheduling_allowed":     strconv.FormatBool(delivery.EffectiveUsage < delivery.PostLimit),
		"immediate_publish_note": "Immediate publishing remains available on your paid plan.",
		"billing_url":            appBaseURL + "/settings/billing",
		"scheduled_posts_url":    appBaseURL + "/projects",
	}
}

func firstNonEmptyWorker(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
