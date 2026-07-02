package emailpolicy

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/emailregistry"
)

const SkipReasonPreferenceDisabled = "preference_disabled"

var ErrPreferenceNotFound = errors.New("email preference not found")

type Preference struct {
	Enabled bool
}

type PreferenceReader interface {
	EmailPreference(ctx context.Context, userID, email string, category emailregistry.PreferenceCategory) (Preference, error)
}

type Request struct {
	EventKey      string
	UserID        string
	Email         string
	DataVariables map[string]any
}

type Decision struct {
	ShouldSend    bool
	SkipReason    string
	DataVariables map[string]any
	Event         emailregistry.Event
}

type Service struct {
	reader     PreferenceReader
	appBaseURL string
}

func NewService(reader PreferenceReader, appBaseURL string) *Service {
	appBaseURL = strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if appBaseURL == "" {
		appBaseURL = "https://app.unipost.dev"
	}
	return &Service{reader: reader, appBaseURL: appBaseURL}
}

func (s *Service) Prepare(ctx context.Context, request Request) (Decision, error) {
	vars := copyVariables(request.DataVariables)
	event, ok := emailregistry.Lookup(strings.TrimSpace(request.EventKey))
	if !ok {
		return Decision{ShouldSend: true, DataVariables: vars}, nil
	}

	vars = s.withFooterVariables(event, vars)
	if !event.PreferenceGated {
		return Decision{ShouldSend: true, DataVariables: vars, Event: event}, nil
	}

	if s == nil || s.reader == nil || strings.TrimSpace(request.UserID) == "" {
		return Decision{ShouldSend: true, DataVariables: vars, Event: event}, nil
	}

	preference, err := s.reader.EmailPreference(ctx, strings.TrimSpace(request.UserID), strings.TrimSpace(request.Email), event.PreferenceCategory)
	if err != nil {
		if errors.Is(err, ErrPreferenceNotFound) {
			return Decision{ShouldSend: true, DataVariables: vars, Event: event}, nil
		}
		return Decision{}, fmt.Errorf("read email preference for %s: %w", event.Key, err)
	}
	if !preference.Enabled {
		return Decision{
			ShouldSend:    false,
			SkipReason:    SkipReasonPreferenceDisabled,
			DataVariables: vars,
			Event:         event,
		}, nil
	}
	return Decision{ShouldSend: true, DataVariables: vars, Event: event}, nil
}

func (s *Service) withFooterVariables(event emailregistry.Event, vars map[string]any) map[string]any {
	if vars == nil {
		vars = map[string]any{}
	}
	category, ok := emailregistry.LookupPreferenceCategory(event.PreferenceCategory)
	categoryLabel := string(event.PreferenceCategory)
	if ok {
		categoryLabel = category.Label
	}
	manageURL := s.appBaseURL + "/settings/notifications"
	vars["footer_policy"] = string(event.FooterPolicy)
	vars["preference_category_key"] = string(event.PreferenceCategory)
	vars["preference_category_label"] = categoryLabel
	vars["manage_preferences_url"] = manageURL
	if strings.TrimSpace(event.RequiredReason) != "" {
		vars["footer_reason"] = event.RequiredReason
	}
	footerText := footerText(event, categoryLabel, manageURL)
	vars["footer_text"] = footerText
	vars["footer_html"] = strings.ReplaceAll(html.EscapeString(footerText), "\n", "<br>")
	return vars
}

func footerText(event emailregistry.Event, categoryLabel, manageURL string) string {
	switch event.FooterPolicy {
	case emailregistry.FooterRequiredNotice, emailregistry.FooterRequiredNoticeNoManage:
		reason := strings.TrimSpace(event.RequiredReason)
		if reason == "" {
			reason = "this message is required for your UniPost account."
		}
		return fmt.Sprintf("%s\nManage optional email preferences: %s", reason, manageURL)
	case emailregistry.FooterTestNotice:
		return fmt.Sprintf("You are receiving this because you requested a UniPost test email.\nManage notification settings: %s", manageURL)
	case emailregistry.FooterUnsubscribe:
		return fmt.Sprintf("You are receiving this because you signed up for UniPost updates and product guidance.\nManage all email preferences: %s", manageURL)
	default:
		return fmt.Sprintf("You are receiving this as part of %s.\nManage email preferences: %s", categoryLabel, manageURL)
	}
}

func copyVariables(vars map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range vars {
		out[key] = value
	}
	return out
}

type postgresPreferenceQueries interface {
	GetEmailPreferenceForSend(ctx context.Context, arg db.GetEmailPreferenceForSendParams) (db.EmailPreference, error)
}

type PostgresPreferenceReader struct {
	queries postgresPreferenceQueries
}

func NewPostgresPreferenceReader(queries postgresPreferenceQueries) *PostgresPreferenceReader {
	return &PostgresPreferenceReader{queries: queries}
}

func (r *PostgresPreferenceReader) EmailPreference(ctx context.Context, userID, _ string, category emailregistry.PreferenceCategory) (Preference, error) {
	if r == nil || r.queries == nil {
		return Preference{}, ErrPreferenceNotFound
	}
	row, err := r.queries.GetEmailPreferenceForSend(ctx, db.GetEmailPreferenceForSendParams{
		UserID:      userID,
		CategoryKey: string(category),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Preference{}, ErrPreferenceNotFound
		}
		return Preference{}, err
	}
	return Preference{Enabled: row.Enabled}, nil
}
