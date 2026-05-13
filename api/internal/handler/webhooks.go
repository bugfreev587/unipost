package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	svix "github.com/svix/svix-webhooks/go"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/mail"
)

type WebhookHandler struct {
	queries    *db.Queries
	mailer     mail.Mailer
	appBaseURL string
}

func NewWebhookHandler(queries *db.Queries, mailer mail.Mailer, appBaseURL string) *WebhookHandler {
	if mailer == nil {
		mailer = mail.NoopMailer{}
	}
	return &WebhookHandler{
		queries:    queries,
		mailer:     mailer,
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
	}
}

type clerkWebhookEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type clerkUserData struct {
	ID             string `json:"id"`
	PrimaryEmailID string `json:"primary_email_address_id"`
	EmailAddresses []struct {
		ID           string `json:"id"`
		EmailAddress string `json:"email_address"`
	} `json:"email_addresses"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (h *WebhookHandler) HandleClerk(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("CLERK_WEBHOOK_SECRET")
	if secret == "" {
		log.Println("CLERK_WEBHOOK_SECRET not configured")
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Webhook not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to read body")
		return
	}

	wh, err := svix.NewWebhook(secret)
	if err != nil {
		log.Printf("Failed to create webhook verifier: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	headers := http.Header{}
	headers.Set("svix-id", r.Header.Get("svix-id"))
	headers.Set("svix-timestamp", r.Header.Get("svix-timestamp"))
	headers.Set("svix-signature", r.Header.Get("svix-signature"))

	err = wh.Verify(body, headers)
	if err != nil {
		log.Printf("Webhook verification failed: %v", err)
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid webhook signature")
		return
	}

	var event clerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid event payload")
		return
	}

	switch event.Type {
	case "user.created", "user.updated":
		h.handleUserUpsert(w, r, event.Type, event.Data)
	case "user.deleted":
		h.handleUserDeleted(w, r, event.Data)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (h *WebhookHandler) handleUserUpsert(w http.ResponseWriter, r *http.Request, eventType string, data json.RawMessage) {
	var userData clerkUserData
	if err := json.Unmarshal(data, &userData); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid user data")
		return
	}

	email := ""
	for _, e := range userData.EmailAddresses {
		if e.ID == userData.PrimaryEmailID {
			email = e.EmailAddress
			break
		}
	}
	if email == "" && len(userData.EmailAddresses) > 0 {
		email = userData.EmailAddresses[0].EmailAddress
	}

	name := userData.FirstName
	if userData.LastName != "" {
		if name != "" {
			name += " "
		}
		name += userData.LastName
	}

	_, err := h.queries.UpsertUser(r.Context(), db.UpsertUserParams{
		ID:    userData.ID,
		Email: email,
		Name:  pgtype.Text{String: name, Valid: name != ""},
	})
	if err != nil {
		// Same email-conflict recovery as me.go Bootstrap: if the email
		// is locked by an orphan row from a prior Clerk-deleted account
		// whose user.deleted webhook was never delivered, drop it and
		// retry with the new id. Without this, every "delete account
		// then sign up again" flow fails until someone manually clears
		// the row.
		var pgErr *pgconn.PgError
		if email != "" && errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
			if delErr := h.queries.DeleteUserByEmail(r.Context(), email); delErr != nil {
				log.Printf("Failed to clear orphaned user row for %s: %v", email, delErr)
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to sync user")
				return
			}
			log.Printf("Cleared orphaned user row for email %s before re-syncing as %s", email, userData.ID)
			_, err = h.queries.UpsertUser(r.Context(), db.UpsertUserParams{
				ID:    userData.ID,
				Email: email,
				Name:  pgtype.Text{String: name, Valid: name != ""},
			})
		}
		if err != nil {
			log.Printf("Failed to upsert user: %v", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to sync user")
			return
		}
	}

	// On user.created: seed a "{name} Workspace" with a "Default" profile
	// if the user doesn't already have a workspace. This ensures the
	// dashboard has something to show on first login.
	existing, _ := h.queries.ListWorkspacesByUser(r.Context(), userData.ID)
	createdWorkspace := false
	workspaceName := ""
	if len(existing) == 0 {
		wsName := "Default Workspace"
		if name != "" {
			wsName = name + "'s Workspace"
		}
		ws, wsErr := h.queries.CreateWorkspace(r.Context(), db.CreateWorkspaceParams{
			UserID: userData.ID,
			Name:   wsName,
		})
		if wsErr != nil {
			log.Printf("Failed to create workspace for %s: %v", userData.ID, wsErr)
		} else {
			// RBAC migration 060 added workspace_members; the auth path
			// resolves the user's role from this table on every Clerk
			// session. Without an owner row here, GetActiveMembership
			// returns ErrNoRows and the dashboard renders NO_WORKSPACE
			// even though the workspace itself was created. Best-effort:
			// log on failure but keep going so we still create the
			// profile (the dualauth self-heal can recover the row).
			if _, memErr := h.queries.CreateMembership(r.Context(), db.CreateMembershipParams{
				WorkspaceID: ws.ID,
				UserID:      userData.ID,
				Role:        "owner",
				InvitedBy:   pgtype.Text{},
			}); memErr != nil {
				log.Printf("Failed to create owner membership for %s in %s: %v", userData.ID, ws.ID, memErr)
			}

			prof, profErr := h.queries.CreateProfile(r.Context(), db.CreateProfileParams{
				WorkspaceID: ws.ID,
				Name:        "Default",
			})
			if profErr != nil {
				log.Printf("Failed to create default profile for %s: %v", userData.ID, profErr)
			} else {
				// Stamp both default_profile_id and last_profile_id
				_ = h.queries.SetUserDefaultProfile(r.Context(), db.SetUserDefaultProfileParams{
					ID:               userData.ID,
					DefaultProfileID: pgtype.Text{String: prof.ID, Valid: true},
				})
				_ = h.queries.SetUserLastProfile(r.Context(), db.SetUserLastProfileParams{
					ID:            userData.ID,
					LastProfileID: pgtype.Text{String: prof.ID, Valid: true},
				})
				log.Printf("Created workspace '%s' + Default profile for user %s", wsName, userData.ID)
				createdWorkspace = true
				workspaceName = wsName
			}
		}
	}

	if eventType == "user.created" && createdWorkspace && email != "" {
		if err := h.mailer.Send(r.Context(), renderWelcomeEmail(email, name, workspaceName, h.appBaseURL)); err != nil {
			log.Printf("Failed to send welcome email to %s: %v", email, err)
		}
	}

	log.Printf("Synced user %s (%s)", userData.ID, email)
	w.WriteHeader(http.StatusOK)
}

func renderWelcomeEmail(to, userName, workspaceName, appBaseURL string) mail.Message {
	if strings.TrimSpace(userName) == "" {
		userName = "there"
	}
	if strings.TrimSpace(workspaceName) == "" {
		workspaceName = "your workspace"
	}
	if strings.TrimSpace(appBaseURL) == "" {
		appBaseURL = "https://app.unipost.dev"
	}

	subject := "[UniPost] Welcome aboard"
	htmlBody := fmt.Sprintf(
		`<p>Hi %s,</p>
<p>Welcome to UniPost. We created <strong>%s</strong> for you so you can start connecting accounts and publishing right away.</p>
<p>If you want setup help or product support, join our Discord support channel: <a href="https://discord.gg/HDBAhYpuQu">https://discord.gg/HDBAhYpuQu</a></p>
<p><a href="%s">Open UniPost →</a></p>`,
		html.EscapeString(userName),
		html.EscapeString(workspaceName),
		appBaseURL,
	)
	textBody := fmt.Sprintf(
		"Hi %s,\n\nWelcome to UniPost. We created %s for you so you can start connecting accounts and publishing right away.\n\nNeed help? Join our Discord support channel: https://discord.gg/HDBAhYpuQu\n\nOpen UniPost: %s\n",
		userName,
		workspaceName,
		appBaseURL,
	)

	return mail.Message{
		To:      to,
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
	}
}

func (h *WebhookHandler) handleUserDeleted(w http.ResponseWriter, r *http.Request, data json.RawMessage) {
	var userData struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &userData); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid user data")
		return
	}

	if err := h.queries.DeleteUser(r.Context(), userData.ID); err != nil {
		log.Printf("Failed to delete user: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete user")
		return
	}

	log.Printf("Deleted user %s", userData.ID)
	w.WriteHeader(http.StatusOK)
}
