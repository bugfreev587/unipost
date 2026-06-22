package quotaemail

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/loops"
)

var thresholds = []int{80, 85, 90, 95, 100}

type Sender interface {
	SendTransactional(context.Context, loops.TransactionalEmail) error
}

type Store interface {
	Snapshot(ctx context.Context, workspaceID, period string) (Snapshot, error)
	AttemptedThresholds(ctx context.Context, workspaceID, period string) (map[int]bool, error)
	CreatePending(ctx context.Context, reminder Reminder) (Reminder, bool, error)
	MarkSent(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, reason string) error
}

type Config struct {
	Store           Store
	Sender          Sender
	TransactionalID string
	PricingURL      string
	AppBaseURL      string
}

type Service struct {
	store           Store
	sender          Sender
	transactionalID string
	pricingURL      string
	appBaseURL      string
}

type Evaluation struct {
	WorkspaceID    string
	Period         string
	Blocked        bool
	RequestedUnits int
}

type Snapshot struct {
	WorkspaceID   string
	WorkspaceName string
	UserID        string
	OwnerEmail    string
	OwnerName     string
	PlanID        string
	Period        string
	Usage         int
	Reserved      int
	Limit         int
}

type Reminder struct {
	ID               string
	WorkspaceID      string
	UserID           string
	Email            string
	Period           string
	ThresholdPercent int
	Status           string
	TransactionalID  string
	IdempotencyKey   string
	EffectiveUsage   int
	CompletedUsage   int
	ReservedUsage    int
	PostLimit        int
	CreatedAt        time.Time
}

func NewService(cfg Config) *Service {
	return &Service{
		store:           cfg.Store,
		sender:          cfg.Sender,
		transactionalID: strings.TrimSpace(cfg.TransactionalID),
		pricingURL:      firstNonEmpty(strings.TrimSpace(cfg.PricingURL), "https://unipost.dev/pricing"),
		appBaseURL:      normalizeAppBaseURL(cfg.AppBaseURL),
	}
}

func (s *Service) EvaluateAndSend(ctx context.Context, eval Evaluation) error {
	if s == nil || s.store == nil || s.sender == nil || s.transactionalID == "" {
		return nil
	}
	workspaceID := strings.TrimSpace(eval.WorkspaceID)
	if workspaceID == "" {
		return nil
	}
	snap, err := s.store.Snapshot(ctx, workspaceID, eval.Period)
	if err != nil {
		return err
	}
	if !eligible(snap) {
		return nil
	}
	period := firstNonEmpty(strings.TrimSpace(snap.Period), strings.TrimSpace(eval.Period))
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}
	effectiveUsage := snap.Usage + snap.Reserved
	projectedUsage := effectiveUsage
	if eval.Blocked && eval.RequestedUnits > 0 {
		projectedUsage += eval.RequestedUnits
	}

	attempted, err := s.store.AttemptedThresholds(ctx, snap.WorkspaceID, period)
	if err != nil {
		return err
	}
	threshold, ok := highestUnattemptedThreshold(projectedUsage, snap.Limit, attempted)
	if !ok {
		return nil
	}

	reminder := Reminder{
		WorkspaceID:      snap.WorkspaceID,
		UserID:           snap.UserID,
		Email:            snap.OwnerEmail,
		Period:           period,
		ThresholdPercent: threshold,
		Status:           "pending",
		TransactionalID:  s.transactionalID,
		IdempotencyKey:   fmt.Sprintf("free_plan_quota:%s:%s:%d", snap.WorkspaceID, period, threshold),
		EffectiveUsage:   projectedUsage,
		CompletedUsage:   snap.Usage,
		ReservedUsage:    snap.Reserved,
		PostLimit:        snap.Limit,
	}
	created, inserted, err := s.store.CreatePending(ctx, reminder)
	if err != nil || !inserted {
		return err
	}

	email := loops.TransactionalEmail{
		TransactionalID: s.transactionalID,
		Email:           snap.OwnerEmail,
		UserID:          snap.UserID,
		IdempotencyKey:  created.IdempotencyKey,
		DataVariables:   s.dataVariables(snap, created, threshold, projectedUsage, eval.Blocked),
	}
	if err := s.sender.SendTransactional(ctx, email); err != nil {
		_ = s.store.MarkFailed(ctx, created.ID, err.Error())
		return nil
	}
	return s.store.MarkSent(ctx, created.ID)
}

func eligible(snap Snapshot) bool {
	return strings.TrimSpace(snap.PlanID) == "free" &&
		strings.TrimSpace(snap.OwnerEmail) != "" &&
		strings.TrimSpace(snap.WorkspaceID) != "" &&
		snap.Limit > 0
}

func highestUnattemptedThreshold(effectiveUsage, limit int, attempted map[int]bool) (int, bool) {
	if limit <= 0 {
		return 0, false
	}
	maxAttempted := 0
	for threshold, ok := range attempted {
		if ok && threshold > maxAttempted {
			maxAttempted = threshold
		}
	}
	selected := 0
	for _, threshold := range thresholds {
		if threshold <= maxAttempted {
			continue
		}
		if effectiveUsage*100 >= threshold*limit {
			selected = threshold
		}
	}
	return selected, selected > 0
}

func (s *Service) dataVariables(snap Snapshot, reminder Reminder, threshold, effectiveUsage int, blocked bool) map[string]any {
	displayUsage := clamp(effectiveUsage, 0, snap.Limit)
	displayPercent := int(math.Round(float64(displayUsage) / float64(snap.Limit) * 100))
	if displayPercent > 100 {
		displayPercent = 100
	}
	remaining := snap.Limit - displayUsage
	if remaining < 0 {
		remaining = 0
	}

	copy := copyForThreshold(threshold, blocked)
	return map[string]any{
		"subject":                copy.subject,
		"preview_text":           copy.previewText,
		"headline":               copy.headline,
		"recipient_name":         recipientName(snap.OwnerName),
		"workspace_name":         firstNonEmpty(snap.WorkspaceName, "your workspace"),
		"body":                   copy.body,
		"status_label":           copy.statusLabel,
		"usage_percent":          strconv.Itoa(displayPercent),
		"posts_used_or_reserved": strconv.Itoa(displayUsage),
		"posts_limit":            strconv.Itoa(snap.Limit),
		"remaining_posts":        strconv.Itoa(remaining),
		"reset_message":          copy.resetMessage,
		"upgrade_message":        "Upgrade your plan to raise your monthly quota immediately and keep scheduled posts moving without interruption.",
		"pricing_url":            s.pricingURL,
		"billing_url":            s.appBaseURL + "/settings/billing",
		"cta_label":              "View pricing",
	}
}

type emailCopy struct {
	subject      string
	previewText  string
	headline     string
	statusLabel  string
	body         string
	resetMessage string
}

func copyForThreshold(threshold int, blocked bool) emailCopy {
	switch threshold {
	case 100:
		return emailCopy{
			subject:      "Warning: UniPost Free plan quota reached 100%",
			previewText:  "Your Free plan workspace is now blocked until reset or upgrade.",
			headline:     "Warning: your Free plan quota has been reached",
			statusLabel:  "Posting is blocked",
			body:         "Your workspace has reached 100% of its monthly Free plan quota. New publish requests are now blocked. Upgrade your plan to keep posting immediately, or wait for the monthly reset.",
			resetMessage: "Your Free plan quota resets on the first day of the next month. Until then, new publish requests remain blocked unless you upgrade.",
		}
	case 95:
		return emailCopy{
			subject:      "UniPost Free plan quota reached 95%",
			previewText:  "You are very close to being blocked on the Free plan.",
			headline:     "You are very close to the Free plan limit",
			statusLabel:  "Action recommended",
			body:         "Your workspace has reached 95% of its monthly Free plan quota. Once it reaches 100%, new publish requests will be blocked until the quota resets or you upgrade.",
			resetMessage: "Your Free plan quota resets on the first day of the next month.",
		}
	case 90:
		return emailCopy{
			subject:      "UniPost Free plan quota reached 90%",
			previewText:  "You have reached 90% of your Free plan quota.",
			headline:     "You have reached 90% of your Free plan quota",
			statusLabel:  "Almost at the limit",
			body:         "Your workspace is close to the Free plan monthly limit. New posts are not blocked yet, but you have only a small amount of quota left for this cycle.",
			resetMessage: "Your Free plan quota resets on the first day of the next month.",
		}
	case 85:
		return emailCopy{
			subject:      "UniPost Free plan quota reached 85%",
			previewText:  "You have reached 85% of your Free plan quota.",
			headline:     "You have reached 85% of your Free plan quota",
			statusLabel:  "Usage is climbing",
			body:         "Your workspace has moved past another Free plan usage checkpoint. Posting is still available, but upgrading now can help you avoid a last-minute interruption.",
			resetMessage: "Your Free plan quota resets on the first day of the next month.",
		}
	default:
		return emailCopy{
			subject:      "UniPost Free plan quota reached 80%",
			previewText:  "You have reached 80% of your Free plan quota.",
			headline:     "You have reached 80% of your Free plan quota",
			statusLabel:  "Heads up",
			body:         "Your workspace is getting close to its monthly Free plan limit. Nothing is blocked yet, but this is a good time to review usage or upgrade if you plan to keep posting this month.",
			resetMessage: "Your Free plan quota resets on the first day of the next month.",
		}
	}
}

func recipientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "there"
	}
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return "there"
	}
	return fields[0]
}

func normalizeAppBaseURL(appBaseURL string) string {
	appBaseURL = strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if appBaseURL == "" {
		return "https://app.unipost.dev"
	}
	return appBaseURL
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
