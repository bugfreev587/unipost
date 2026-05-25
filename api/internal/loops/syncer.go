package loops

import (
	"context"
	"log/slog"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type LifecycleClient interface {
	Enabled() bool
	UpsertContact(ctx context.Context, contact Contact) error
	SendEvent(ctx context.Context, event Event) error
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

type Options struct {
	Enabled func(context.Context, DashboardUser) bool
}

type Syncer struct {
	client  LifecycleClient
	enabled func(context.Context, DashboardUser) bool
}

func NewSyncer(client LifecycleClient, opts Options) *Syncer {
	enabled := opts.Enabled
	if enabled == nil {
		enabled = func(ctx context.Context, user DashboardUser) bool {
			return featureflags.Enabled(ctx, featureflags.LoopsIntegrationV1, featureflags.Target{
				UserID:      user.ID,
				UserEmail:   user.Email,
				WorkspaceID: user.WorkspaceID,
			})
		}
	}
	return &Syncer{client: client, enabled: enabled}
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
