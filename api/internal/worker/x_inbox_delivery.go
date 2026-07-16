package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

const xInboxReconcileInterval = time.Minute

type XInboxCipher interface {
	Decrypt(string) (string, error)
}

type XInboxDeliveryAPI interface {
	EnsureFilteredStreamRule(context.Context, string, string, string) (xinbox.StreamRule, error)
	DeleteFilteredStreamRule(context.Context, string, string) error
	EnsureWebhook(context.Context, string, string) (xinbox.Webhook, error)
	EnsureDMSubscription(context.Context, string, string, string, string, string) (xinbox.ActivitySubscription, error)
	DeleteActivitySubscription(context.Context, string, string) error
}

type XInboxUsageReader interface {
	Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error)
}

type XInboxStreamRunner interface {
	Run(context.Context, string, string, func(xinbox.StreamEvent) error) error
}

type XInboxLeaderLease interface {
	Release(context.Context) error
}

type XInboxLeaderElector interface {
	TryAcquire(context.Context, string) (XInboxLeaderLease, bool, error)
}

type XInboxDeliveryStore interface {
	ListAccounts(context.Context) ([]XInboxDeliveryAccount, error)
	SaveState(context.Context, XInboxDeliveryState) error
	ClaimCleanupIntents(context.Context, string, time.Time, time.Time, int) ([]XInboxCleanupIntent, error)
	ReleaseCleanupIntent(context.Context, XInboxCleanupIntent, time.Time) error
	CompleteCleanupIntent(context.Context, string, string) error
}

type XInboxDeliveryAccount struct {
	SocialAccountID          string
	WorkspaceID              string
	Handle                   string
	ExternalUserID           string
	AccessTokenEncrypted     string
	AppMode                  xinbox.AppMode
	AppBearerTokenEncrypted  string
	Scopes                   []string
	AccountActive            bool
	PlanAllowsInbox          bool
	FilteredStreamRuleID     string
	ActivityDMSubscriptionID string
}

type XInboxDeliveryState struct {
	SocialAccountID          string
	FilteredStreamRuleID     string
	ActivityDMSubscriptionID string
	DeliveryStatus           string
	LastError                string
	LastSyncedAt             time.Time
}

type XInboxCleanupIntent struct {
	ID                       string
	SocialAccountID          string
	AppMode                  xinbox.AppMode
	AppBearerTokenEncrypted  string
	FilteredStreamRuleID     string
	ActivityDMSubscriptionID string
	LastError                string
	Attempts                 int
	LeaseOwner               string
	LeaseUntil               time.Time
	NextAttemptAt            time.Time
}

type XInboxAppStream struct {
	Identity    string
	BearerToken string
}

type XInboxDeliveryConfig struct {
	Store            XInboxDeliveryStore
	API              XInboxDeliveryAPI
	Cipher           XInboxCipher
	Usage            XInboxUsageReader
	Leader           XInboxLeaderElector
	Stream           XInboxStreamRunner
	ManagedAppBearer string
	WebhookURL       string
	Now              func() time.Time
	EventHandler     func(context.Context, string, xinbox.StreamEvent) error
	CleanupOwner     string
}

type XInboxDeliveryWorker struct {
	store            XInboxDeliveryStore
	api              XInboxDeliveryAPI
	cipher           XInboxCipher
	usage            XInboxUsageReader
	leader           XInboxLeaderElector
	stream           XInboxStreamRunner
	managedAppBearer string
	webhookURL       string
	now              func() time.Time
	eventHandler     func(context.Context, string, xinbox.StreamEvent) error
	cleanupOwner     string

	streamsMu sync.Mutex
	streams   map[string]struct{}
}

func NewXInboxDeliveryWorker(config XInboxDeliveryConfig) *XInboxDeliveryWorker {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	cleanupOwner := strings.TrimSpace(config.CleanupOwner)
	if cleanupOwner == "" {
		cleanupOwner = "x-inbox-cleanup-" + uuid.NewString()
	}
	return &XInboxDeliveryWorker{
		store:            config.Store,
		api:              config.API,
		cipher:           config.Cipher,
		usage:            config.Usage,
		leader:           config.Leader,
		stream:           config.Stream,
		managedAppBearer: strings.TrimSpace(config.ManagedAppBearer),
		webhookURL:       strings.TrimSpace(config.WebhookURL),
		now:              now,
		eventHandler:     config.EventHandler,
		cleanupOwner:     cleanupOwner,
		streams:          make(map[string]struct{}),
	}
}

func NewPostgresXInboxDeliveryWorker(
	pool *pgxpool.Pool,
	queries *db.Queries,
	encryptor *crypto.AESEncryptor,
	usage XInboxUsageReader,
	client *xinbox.Client,
	managedAppBearer string,
	webhookURL string,
) *XInboxDeliveryWorker {
	store := &postgresXInboxDeliveryStore{pool: pool, queries: queries}
	return NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:            store,
		API:              client,
		Cipher:           encryptor,
		Usage:            usage,
		Leader:           &postgresXInboxLeader{pool: pool},
		Stream:           xinbox.NewStreamSupervisor(client, xinbox.StreamSupervisorConfig{}),
		ManagedAppBearer: managedAppBearer,
		WebhookURL:       webhookURL,
	})
}

func (w *XInboxDeliveryWorker) SetEventHandler(
	handler func(context.Context, string, xinbox.StreamEvent) error,
) *XInboxDeliveryWorker {
	w.eventHandler = handler
	return w
}

func (w *XInboxDeliveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(xInboxReconcileInterval)
	defer ticker.Stop()
	slog.Info("X inbox delivery worker started")
	w.reconcileAndStartStreams(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("X inbox delivery worker stopped")
			return
		case <-ticker.C:
			w.reconcileAndStartStreams(ctx)
		}
	}
}

func (w *XInboxDeliveryWorker) reconcileAndStartStreams(ctx context.Context) {
	apps, err := w.reconcile(ctx)
	if err != nil {
		slog.Warn("X inbox delivery reconciliation completed with errors", "error", err)
	}
	if w.eventHandler == nil {
		return
	}
	for _, app := range apps {
		w.startAppStream(ctx, app)
	}
}

func (w *XInboxDeliveryWorker) ReconcileOnce(ctx context.Context) error {
	_, err := w.reconcile(ctx)
	return err
}

func (w *XInboxDeliveryWorker) reconcile(ctx context.Context) ([]XInboxAppStream, error) {
	if w.store == nil || w.api == nil || w.cipher == nil {
		return nil, errors.New("X inbox delivery worker is not fully configured")
	}
	var joined error
	cleanupNow := w.now().UTC()
	cleanups, err := w.store.ClaimCleanupIntents(
		ctx,
		w.cleanupOwner,
		cleanupNow,
		cleanupNow.Add(2*time.Minute),
		100,
	)
	if err != nil {
		joined = errors.Join(joined, fmt.Errorf("claim X inbox cleanup intents: %w", err))
	} else {
		for _, intent := range cleanups {
			if err := w.reconcileCleanupIntent(ctx, intent); err != nil {
				joined = errors.Join(joined, err)
			}
		}
	}

	accounts, err := w.store.ListAccounts(ctx)
	if err != nil {
		return nil, errors.Join(joined, fmt.Errorf("list X inbox delivery accounts: %w", err))
	}
	appsByIdentity := make(map[string]XInboxAppStream)
	for _, account := range accounts {
		app, streamDesired, err := w.reconcileAccount(ctx, account)
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("reconcile X inbox account %s: %w", account.SocialAccountID, err))
			continue
		}
		if streamDesired {
			appsByIdentity[app.Identity] = app
		}
	}
	apps := make([]XInboxAppStream, 0, len(appsByIdentity))
	for _, app := range appsByIdentity {
		apps = append(apps, app)
	}
	return apps, joined
}

func (w *XInboxDeliveryWorker) reconcileAccount(
	ctx context.Context,
	account XInboxDeliveryAccount,
) (XInboxAppStream, bool, error) {
	now := w.now().UTC()
	state := XInboxDeliveryState{
		SocialAccountID:          account.SocialAccountID,
		FilteredStreamRuleID:     account.FilteredStreamRuleID,
		ActivityDMSubscriptionID: account.ActivityDMSubscriptionID,
		DeliveryStatus:           xinbox.DeliveryStatusPending,
		LastSyncedAt:             now,
	}

	targetStatus := xinbox.DeliveryStatusPending
	commentsDesired := account.AccountActive && account.PlanAllowsInbox && hasXInboxScopes(account.Scopes, "tweet.read", "tweet.write", "users.read")
	dmsDesired := commentsDesired && hasXInboxScopes(account.Scopes, "dm.read", "dm.write")
	if !account.PlanAllowsInbox {
		targetStatus = xinbox.DeliveryStatusPausedPlan
		commentsDesired = false
		dmsDesired = false
	}
	if account.AppMode == xinbox.AppModeLegacyUnknown {
		commentsDesired = false
		dmsDesired = false
	}
	if account.AppMode == xinbox.AppModeUniPostManaged && commentsDesired && w.usage != nil {
		snapshot, err := w.usage.Snapshot(ctx, account.WorkspaceID, now)
		if err != nil {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, err)
		}
		if snapshot.PausePaidSources {
			commentsDesired = false
			dmsDesired = false
			if snapshot.InboundPauseReason == xcredits.PauseReasonMonthlyAllowance {
				targetStatus = xinbox.DeliveryStatusPausedAllowance
			} else {
				targetStatus = xinbox.DeliveryStatusPausedCap
			}
		}
	}

	appBearerToken, appIdentity, appTokenErr := w.resolveAccountAppToken(account)
	if appTokenErr != nil && (state.FilteredStreamRuleID != "" || commentsDesired || dmsDesired) {
		return XInboxAppStream{}, false, w.saveAccountError(ctx, state, appTokenErr)
	}
	var userAccessToken string
	if dmsDesired {
		var err error
		userAccessToken, err = w.cipher.Decrypt(account.AccessTokenEncrypted)
		if err != nil {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, fmt.Errorf("decrypt connected X user token: %w", err))
		}
	}

	if !commentsDesired && state.FilteredStreamRuleID != "" {
		if err := w.api.DeleteFilteredStreamRule(ctx, appBearerToken, state.FilteredStreamRuleID); err != nil {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, err)
		}
		state.FilteredStreamRuleID = ""
		state.DeliveryStatus = targetStatus
		if err := w.store.SaveState(ctx, state); err != nil {
			return XInboxAppStream{}, false, err
		}
	}
	if !dmsDesired && state.ActivityDMSubscriptionID != "" {
		if err := w.api.DeleteActivitySubscription(ctx, appBearerToken, state.ActivityDMSubscriptionID); err != nil {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, err)
		}
		state.ActivityDMSubscriptionID = ""
		state.DeliveryStatus = targetStatus
		if err := w.store.SaveState(ctx, state); err != nil {
			return XInboxAppStream{}, false, err
		}
	}

	if commentsDesired && state.FilteredStreamRuleID == "" {
		if account.Handle == "" {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, errors.New("connected X account has no handle"))
		}
		rule, err := w.api.EnsureFilteredStreamRule(ctx, appBearerToken, account.SocialAccountID, account.Handle)
		if err != nil {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, err)
		}
		state.FilteredStreamRuleID = rule.ID
		if err := w.store.SaveState(ctx, state); err != nil {
			return XInboxAppStream{}, false, err
		}
	}
	if dmsDesired && state.ActivityDMSubscriptionID == "" {
		if w.webhookURL == "" {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, errors.New("X_INBOX_WEBHOOK_URL is not configured"))
		}
		webhook, err := w.api.EnsureWebhook(ctx, appBearerToken, w.webhookURL)
		if err != nil {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, err)
		}
		subscription, err := w.api.EnsureDMSubscription(
			ctx,
			userAccessToken,
			appBearerToken,
			account.SocialAccountID,
			account.ExternalUserID,
			webhook.ID,
		)
		if err != nil {
			return XInboxAppStream{}, false, w.saveAccountError(ctx, state, err)
		}
		state.ActivityDMSubscriptionID = subscription.ID
		if err := w.store.SaveState(ctx, state); err != nil {
			return XInboxAppStream{}, false, err
		}
	}

	if commentsDesired || dmsDesired {
		state.DeliveryStatus = xinbox.DeliveryStatusActive
	} else {
		state.DeliveryStatus = targetStatus
	}
	state.LastError = ""
	if err := w.store.SaveState(ctx, state); err != nil {
		return XInboxAppStream{}, false, err
	}
	return XInboxAppStream{Identity: appIdentity, BearerToken: appBearerToken}, commentsDesired && state.FilteredStreamRuleID != "", nil
}

func (w *XInboxDeliveryWorker) saveAccountError(
	ctx context.Context,
	state XInboxDeliveryState,
	cause error,
) error {
	state.DeliveryStatus = xinbox.DeliveryStatusError
	state.LastError = cause.Error()
	state.LastSyncedAt = w.now().UTC()
	if err := w.store.SaveState(ctx, state); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (w *XInboxDeliveryWorker) reconcileCleanupIntent(ctx context.Context, intent XInboxCleanupIntent) error {
	appToken, err := w.resolveCleanupAppToken(intent)
	if err != nil && (intent.FilteredStreamRuleID != "" || intent.ActivityDMSubscriptionID != "") {
		return w.releaseFailedCleanup(
			ctx,
			intent,
			fmt.Errorf("cleanup X inbox account %s: %w", intent.SocialAccountID, err),
		)
	}
	if intent.FilteredStreamRuleID != "" {
		if err := w.api.DeleteFilteredStreamRule(ctx, appToken, intent.FilteredStreamRuleID); err != nil {
			return w.releaseFailedCleanup(
				ctx,
				intent,
				fmt.Errorf("cleanup X inbox rule for account %s: %w", intent.SocialAccountID, err),
			)
		}
		intent.FilteredStreamRuleID = ""
		intent.LastError = ""
		if intent.ActivityDMSubscriptionID == "" {
			return w.store.CompleteCleanupIntent(ctx, intent.ID, intent.LeaseOwner)
		}
	}
	if intent.ActivityDMSubscriptionID != "" {
		if err := w.api.DeleteActivitySubscription(ctx, appToken, intent.ActivityDMSubscriptionID); err != nil {
			return w.releaseFailedCleanup(
				ctx,
				intent,
				fmt.Errorf("cleanup X inbox subscription for account %s: %w", intent.SocialAccountID, err),
			)
		}
		intent.ActivityDMSubscriptionID = ""
		intent.LastError = ""
		return w.store.CompleteCleanupIntent(ctx, intent.ID, intent.LeaseOwner)
	}
	return w.store.CompleteCleanupIntent(ctx, intent.ID, intent.LeaseOwner)
}

func (w *XInboxDeliveryWorker) releaseFailedCleanup(
	ctx context.Context,
	intent XInboxCleanupIntent,
	cause error,
) error {
	intent.LastError = cause.Error()
	nextAttemptAt := w.now().UTC().Add(cleanupRetryDelay(intent.Attempts))
	if err := w.store.ReleaseCleanupIntent(ctx, intent, nextAttemptAt); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func cleanupRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := time.Minute
	for attempt := 1; attempt < attempts && delay < time.Hour; attempt++ {
		delay *= 2
		if delay > time.Hour {
			return time.Hour
		}
	}
	return delay
}

func (w *XInboxDeliveryWorker) resolveAccountAppToken(account XInboxDeliveryAccount) (string, string, error) {
	switch account.AppMode {
	case xinbox.AppModeUniPostManaged:
		if w.managedAppBearer == "" {
			return "", "", errors.New("TWITTER_BEARER_TOKEN is not configured")
		}
		return w.managedAppBearer, string(xinbox.AppModeUniPostManaged), nil
	case xinbox.AppModeWorkspace:
		if account.AppBearerTokenEncrypted == "" {
			return "", "", errors.New("workspace X app bearer token is not configured")
		}
		token, err := w.cipher.Decrypt(account.AppBearerTokenEncrypted)
		if err != nil {
			return "", "", fmt.Errorf("decrypt workspace X app bearer token: %w", err)
		}
		return token, "workspace:" + account.WorkspaceID, nil
	default:
		return "", "", fmt.Errorf("unsupported X app mode %q", account.AppMode)
	}
}

func (w *XInboxDeliveryWorker) resolveCleanupAppToken(intent XInboxCleanupIntent) (string, error) {
	switch intent.AppMode {
	case xinbox.AppModeUniPostManaged:
		if w.managedAppBearer == "" {
			return "", errors.New("TWITTER_BEARER_TOKEN is not configured")
		}
		return w.managedAppBearer, nil
	case xinbox.AppModeWorkspace:
		if intent.AppBearerTokenEncrypted == "" {
			return "", errors.New("cleanup intent has no workspace X app bearer token")
		}
		return w.cipher.Decrypt(intent.AppBearerTokenEncrypted)
	default:
		return "", fmt.Errorf("unsupported cleanup X app mode %q", intent.AppMode)
	}
}

func hasXInboxScopes(scopes []string, required ...string) bool {
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

func (w *XInboxDeliveryWorker) startAppStream(ctx context.Context, app XInboxAppStream) {
	w.streamsMu.Lock()
	if _, active := w.streams[app.Identity]; active {
		w.streamsMu.Unlock()
		return
	}
	w.streams[app.Identity] = struct{}{}
	w.streamsMu.Unlock()
	go func() {
		defer func() {
			w.streamsMu.Lock()
			delete(w.streams, app.Identity)
			w.streamsMu.Unlock()
		}()
		if err := w.runAppStream(ctx, app); err != nil && ctx.Err() == nil {
			slog.Warn("X inbox filtered stream stopped", "app_identity", app.Identity, "error", err)
		}
	}()
}

func (w *XInboxDeliveryWorker) runAppStream(ctx context.Context, app XInboxAppStream) error {
	if w.leader == nil || w.stream == nil {
		return nil
	}
	lease, acquired, err := w.leader.TryAcquire(ctx, app.Identity)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := lease.Release(releaseCtx); err != nil {
			slog.Warn("release X inbox stream advisory lock", "app_identity", app.Identity, "error", err)
		}
	}()
	handler := func(event xinbox.StreamEvent) error {
		if w.eventHandler == nil {
			return nil
		}
		return w.eventHandler(ctx, app.Identity, event)
	}
	return w.stream.Run(ctx, app.Identity, app.BearerToken, handler)
}

type postgresXInboxDeliveryStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func (s *postgresXInboxDeliveryStore) ListAccounts(ctx context.Context) ([]XInboxDeliveryAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			sa.id,
			p.workspace_id,
			COALESCE(sa.account_name, ''),
			COALESCE(sa.external_user_id, ''),
			sa.access_token,
			COALESCE(sa.x_app_mode, 'legacy_unknown'),
			COALESCE(pc.app_bearer_token, ''),
			sa.scope,
			(sa.disconnected_at IS NULL AND sa.status = 'active') AS account_active,
			COALESCE(pl.allow_inbox, FALSE) AS plan_allows_inbox,
			COALESCE(r.filtered_stream_rule_id, ''),
			COALESCE(r.activity_dm_subscription_id, '')
		FROM social_accounts sa
		JOIN profiles p ON p.id = sa.profile_id
		LEFT JOIN subscriptions sub ON sub.workspace_id = p.workspace_id
		LEFT JOIN plans pl ON pl.id = COALESCE(sub.plan_id, 'free')
		LEFT JOIN platform_credentials pc
		  ON pc.workspace_id = p.workspace_id AND pc.platform = 'twitter'
		LEFT JOIN x_inbox_delivery_resources r ON r.social_account_id = sa.id
		WHERE sa.platform = 'twitter'
		  AND (
		    sa.disconnected_at IS NULL
		    OR r.filtered_stream_rule_id IS NOT NULL
		    OR r.activity_dm_subscription_id IS NOT NULL
		  )
		ORDER BY sa.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []XInboxDeliveryAccount
	for rows.Next() {
		var account XInboxDeliveryAccount
		var appMode string
		if err := rows.Scan(
			&account.SocialAccountID,
			&account.WorkspaceID,
			&account.Handle,
			&account.ExternalUserID,
			&account.AccessTokenEncrypted,
			&appMode,
			&account.AppBearerTokenEncrypted,
			&account.Scopes,
			&account.AccountActive,
			&account.PlanAllowsInbox,
			&account.FilteredStreamRuleID,
			&account.ActivityDMSubscriptionID,
		); err != nil {
			return nil, err
		}
		account.AppMode = xinbox.AppMode(appMode)
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *postgresXInboxDeliveryStore) SaveState(ctx context.Context, state XInboxDeliveryState) error {
	_, err := s.queries.UpsertXInboxDeliveryResource(ctx, db.UpsertXInboxDeliveryResourceParams{
		SocialAccountID:          state.SocialAccountID,
		FilteredStreamRuleID:     nullableText(state.FilteredStreamRuleID),
		ActivityDmSubscriptionID: nullableText(state.ActivityDMSubscriptionID),
		DeliveryStatus:           state.DeliveryStatus,
		LastError:                nullableText(state.LastError),
		LastSyncedAt:             pgtype.Timestamptz{Time: state.LastSyncedAt, Valid: !state.LastSyncedAt.IsZero()},
	})
	return err
}

func (s *postgresXInboxDeliveryStore) ClaimCleanupIntents(
	ctx context.Context,
	owner string,
	now time.Time,
	leaseUntil time.Time,
	limit int,
) ([]XInboxCleanupIntent, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM x_inbox_delivery_cleanup_intents
			WHERE next_attempt_at <= $2
			  AND (lease_until IS NULL OR lease_until <= $2)
			ORDER BY next_attempt_at, created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		)
		UPDATE x_inbox_delivery_cleanup_intents i
		SET lease_owner = $1,
		    lease_until = $3,
		    attempts = attempts + 1,
		    updated_at = NOW()
		FROM candidates c
		WHERE i.id = c.id
		RETURNING
			i.id,
			i.social_account_id,
			i.x_app_mode,
			COALESCE(i.app_bearer_token, ''),
			COALESCE(i.filtered_stream_rule_id, ''),
			COALESCE(i.activity_dm_subscription_id, ''),
			COALESCE(i.last_error, ''),
			i.attempts,
			COALESCE(i.lease_owner, ''),
			i.lease_until,
			i.next_attempt_at
	`, owner, now, leaseUntil, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var intents []XInboxCleanupIntent
	for rows.Next() {
		var intent XInboxCleanupIntent
		var appMode string
		if err := rows.Scan(
			&intent.ID,
			&intent.SocialAccountID,
			&appMode,
			&intent.AppBearerTokenEncrypted,
			&intent.FilteredStreamRuleID,
			&intent.ActivityDMSubscriptionID,
			&intent.LastError,
			&intent.Attempts,
			&intent.LeaseOwner,
			&intent.LeaseUntil,
			&intent.NextAttemptAt,
		); err != nil {
			return nil, err
		}
		intent.AppMode = xinbox.AppMode(appMode)
		intents = append(intents, intent)
	}
	return intents, rows.Err()
}

func (s *postgresXInboxDeliveryStore) ReleaseCleanupIntent(
	ctx context.Context,
	intent XInboxCleanupIntent,
	nextAttemptAt time.Time,
) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_inbox_delivery_cleanup_intents
		SET filtered_stream_rule_id = NULLIF($2, ''),
		    activity_dm_subscription_id = NULLIF($3, ''),
		    last_error = NULLIF($4, ''),
		    next_attempt_at = $5,
		    lease_owner = NULL,
		    lease_until = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND lease_owner = $6
	`, intent.ID, intent.FilteredStreamRuleID, intent.ActivityDMSubscriptionID, intent.LastError, nextAttemptAt, intent.LeaseOwner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X inbox cleanup lease was lost before retry scheduling")
	}
	return nil
}

func (s *postgresXInboxDeliveryStore) CompleteCleanupIntent(ctx context.Context, id, owner string) error {
	tag, err := s.pool.Exec(
		ctx,
		`DELETE FROM x_inbox_delivery_cleanup_intents WHERE id = $1 AND lease_owner = $2`,
		id,
		owner,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X inbox cleanup lease was lost before completion")
	}
	return nil
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

type postgresXInboxLeader struct {
	pool *pgxpool.Pool
}

type postgresXInboxLeaderLease struct {
	conn     *pgxpool.Conn
	lockKey  string
	released sync.Once
}

func (l *postgresXInboxLeader) TryAcquire(
	ctx context.Context,
	appIdentity string,
) (XInboxLeaderLease, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	var acquired bool
	if err := conn.QueryRow(
		ctx,
		`SELECT pg_try_advisory_lock(hashtextextended($1, 0))`,
		"x-inbox-stream:"+appIdentity,
	).Scan(&acquired); err != nil {
		conn.Release()
		return nil, false, err
	}
	if !acquired {
		conn.Release()
		return nil, false, nil
	}
	return &postgresXInboxLeaderLease{
		conn:    conn,
		lockKey: "x-inbox-stream:" + appIdentity,
	}, true, nil
}

func (l *postgresXInboxLeaderLease) Release(ctx context.Context) error {
	var releaseErr error
	l.released.Do(func() {
		var released bool
		releaseErr = l.conn.QueryRow(
			ctx,
			`SELECT pg_advisory_unlock(hashtextextended($1, 0))`,
			l.lockKey,
		).Scan(&released)
		l.conn.Release()
		if releaseErr == nil && !released {
			releaseErr = errors.New("X inbox stream advisory lock was not held")
		}
	})
	return releaseErr
}
