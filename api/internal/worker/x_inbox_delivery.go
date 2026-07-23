package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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

const (
	xInboxReconcileInterval = time.Minute
	xInboxCleanupBatchLimit = 10
	xInboxCleanupBudget     = 30 * time.Second
	xInboxCleanupLease      = 2 * time.Minute
)

type XInboxCipher interface {
	Decrypt(string) (string, error)
}

type XInboxDeliveryAPI interface {
	EnsureFilteredStreamRule(context.Context, string, string, string) (xinbox.StreamRule, error)
	DeleteFilteredStreamRule(context.Context, string, string) error
	EnsureWebhook(context.Context, string, string) (xinbox.Webhook, error)
	ListWebhooks(context.Context, string) ([]xinbox.Webhook, error)
	DeleteWebhook(context.Context, string, string) error
	EnsureDMSubscription(context.Context, string, string, string, string) (xinbox.ActivitySubscription, error)
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
	TryAcquire(context.Context, string, context.CancelFunc) (XInboxLeaderLease, bool, error)
}

type XInboxDeliveryStore interface {
	ListAccounts(context.Context) ([]XInboxDeliveryAccount, error)
	SaveState(context.Context, XInboxDeliveryState) error
	ClaimCleanupIntents(context.Context, string, time.Time, time.Time, int) ([]XInboxCleanupIntent, error)
	ReleaseCleanupIntent(context.Context, XInboxCleanupIntent, time.Time) error
	CompleteCleanupIntent(context.Context, string, string) error
}

type XInboxDeliveryAccount struct {
	SocialAccountID                    string
	WorkspaceID                        string
	WebhookRouteKey                    string
	Handle                             string
	ExternalAccountID                  string
	AppMode                            xinbox.AppMode
	AppBearerTokenEncrypted            string
	ConsumerSecretConfigured           bool
	Scopes                             []string
	AccountActive                      bool
	PlanAllowsInbox                    bool
	FilteredStreamRuleID               string
	ActivityDMSubscriptionID           string
	ActivityWebhookRouteKey            string
	DMSubscriptionForbiddenFingerprint string
}

type XInboxDeliveryState struct {
	SocialAccountID                    string
	FilteredStreamRuleID               string
	ActivityDMSubscriptionID           string
	ActivityWebhookRouteKey            string
	DMSubscriptionForbiddenFingerprint string
	DeliveryStatus                     string
	LastError                          string
	LastSyncedAt                       time.Time
}

type XInboxCleanupIntent struct {
	ID                       string
	CleanupKey               string
	SocialAccountID          string
	AppMode                  xinbox.AppMode
	SourceAppIdentity        string
	AppBearerTokenEncrypted  string
	WebhookRouteKey          string
	ConsumerSecretEncrypted  string
	FilteredStreamRuleID     string
	ActivityDMSubscriptionID string
	LastError                string
	Attempts                 int
	LeaseOwner               string
	LeaseUntil               time.Time
	NextAttemptAt            time.Time
}

type XInboxAppStream struct {
	Identity                 string
	WebhookRouteKey          string
	BearerToken              string
	ConsumerSecretConfigured bool
}

type XInboxDeliveryConfig struct {
	Store                           XInboxDeliveryStore
	API                             XInboxDeliveryAPI
	Cipher                          XInboxCipher
	Usage                           XInboxUsageReader
	Leader                          XInboxLeaderElector
	Stream                          XInboxStreamRunner
	ManagedAppBearer                string
	ManagedConsumerSecretConfigured bool
	WebhookURL                      string
	DMsAvailable                    func(context.Context, string) (bool, error)
	DMCanaryAccountIDs              map[string]struct{}
	Now                             func() time.Time
	EventHandler                    func(context.Context, string, xinbox.StreamEvent) error
	CleanupOwner                    string
}

type XInboxDeliveryWorker struct {
	store                           XInboxDeliveryStore
	api                             XInboxDeliveryAPI
	cipher                          XInboxCipher
	usage                           XInboxUsageReader
	leader                          XInboxLeaderElector
	stream                          XInboxStreamRunner
	managedAppBearer                string
	managedConsumerSecretConfigured bool
	webhookURL                      string
	dmsAvailable                    func(context.Context, string) (bool, error)
	dmCanaryAccountIDs              map[string]struct{}
	now                             func() time.Time
	eventHandler                    func(context.Context, string, xinbox.StreamEvent) error
	cleanupOwner                    string

	streamsMu        sync.Mutex
	streams          map[string]managedXInboxStream
	streamGeneration uint64
}

type managedXInboxStream struct {
	cancel      context.CancelFunc
	done        <-chan struct{}
	fingerprint string
	generation  uint64
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
	dmCanaryAccountIDs := make(map[string]struct{}, len(config.DMCanaryAccountIDs))
	for accountID := range config.DMCanaryAccountIDs {
		dmCanaryAccountIDs[accountID] = struct{}{}
	}
	return &XInboxDeliveryWorker{
		store:                           config.Store,
		api:                             config.API,
		cipher:                          config.Cipher,
		usage:                           config.Usage,
		leader:                          config.Leader,
		stream:                          config.Stream,
		managedAppBearer:                strings.TrimSpace(config.ManagedAppBearer),
		managedConsumerSecretConfigured: config.ManagedConsumerSecretConfigured,
		webhookURL:                      strings.TrimSpace(config.WebhookURL),
		dmsAvailable:                    config.DMsAvailable,
		dmCanaryAccountIDs:              dmCanaryAccountIDs,
		now:                             now,
		eventHandler:                    config.EventHandler,
		cleanupOwner:                    cleanupOwner,
		streams:                         make(map[string]managedXInboxStream),
	}
}

func NewPostgresXInboxDeliveryWorker(
	databaseURL string,
	pool *pgxpool.Pool,
	queries *db.Queries,
	encryptor *crypto.AESEncryptor,
	usage XInboxUsageReader,
	client *xinbox.Client,
	managedAppBearer string,
	managedConsumerSecretConfigured bool,
	managedWebhookRouteKey string,
	webhookURL string,
) *XInboxDeliveryWorker {
	store := &postgresXInboxDeliveryStore{
		pool:                   pool,
		queries:                queries,
		managedWebhookRouteKey: strings.TrimSpace(managedWebhookRouteKey),
	}
	return NewXInboxDeliveryWorker(XInboxDeliveryConfig{
		Store:                           store,
		API:                             client,
		Cipher:                          encryptor,
		Usage:                           usage,
		Leader:                          NewPostgresStreamLockManager(databaseURL),
		Stream:                          xinbox.NewStreamSupervisor(client, xinbox.StreamSupervisorConfig{}),
		ManagedAppBearer:                managedAppBearer,
		ManagedConsumerSecretConfigured: managedConsumerSecretConfigured,
		WebhookURL:                      webhookURL,
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
	defer w.stopAllStreams()
	defer func() {
		if closer, ok := w.leader.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				slog.Warn("close X inbox lock manager", "error", err)
			}
		}
	}()
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
	apps, complete, desiredErr := w.reconcileDesiredCycle(ctx)
	if !complete {
		apps = nil
	} else {
		if w.eventHandler == nil {
			apps = nil
		}
		w.syncDesiredStreams(ctx, apps)
	}
	cleanupErr := w.processCleanupBudget(ctx)
	if err := errors.Join(desiredErr, cleanupErr); err != nil {
		slog.Warn("X inbox delivery reconciliation completed with errors", "error", err)
	}
}

func (w *XInboxDeliveryWorker) ReconcileOnce(ctx context.Context) error {
	_, _, desiredErr := w.reconcileDesiredCycle(ctx)
	return errors.Join(desiredErr, w.processCleanupBudget(ctx))
}

func (w *XInboxDeliveryWorker) reconcile(ctx context.Context) ([]XInboxAppStream, error) {
	apps, _, desiredErr := w.reconcileDesiredCycle(ctx)
	return apps, errors.Join(desiredErr, w.processCleanupBudget(ctx))
}

func (w *XInboxDeliveryWorker) reconcileDesiredCycle(
	ctx context.Context,
) (apps []XInboxAppStream, complete bool, resultErr error) {
	if w.leader != nil {
		lease, acquired, err := w.leader.TryAcquire(ctx, "x-inbox-reconcile", nil)
		if err != nil {
			return nil, false, fmt.Errorf("acquire X inbox reconciliation lock: %w", err)
		}
		if !acquired {
			return nil, false, nil
		}
		defer func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := lease.Release(releaseCtx); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("release X inbox reconciliation lock: %w", err))
				complete = false
			}
		}()
	}
	apps, complete, resultErr = w.reconcileDesiredUnlocked(ctx)
	return
}

func (w *XInboxDeliveryWorker) reconcileDesiredUnlocked(
	ctx context.Context,
) ([]XInboxAppStream, bool, error) {
	if w.store == nil || w.api == nil || w.cipher == nil {
		return nil, false, errors.New("X inbox delivery worker is not fully configured")
	}
	var joined error
	accounts, err := w.store.ListAccounts(ctx)
	if err != nil {
		return nil, false, errors.Join(joined, fmt.Errorf("list X inbox delivery accounts: %w", err))
	}
	appsByIdentity := make(map[string]XInboxAppStream)
	desiredComplete := true
	for _, account := range accounts {
		app, streamDesired, desiredKnown, err := w.reconcileAccount(ctx, account)
		if !desiredKnown {
			desiredComplete = false
		}
		if streamDesired {
			appsByIdentity[app.Identity] = app
		}
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("reconcile X inbox account %s: %w", account.SocialAccountID, err))
		}
	}
	apps := make([]XInboxAppStream, 0, len(appsByIdentity))
	for _, app := range appsByIdentity {
		apps = append(apps, app)
	}
	return apps, desiredComplete, joined
}

func (w *XInboxDeliveryWorker) processCleanupBudget(ctx context.Context) error {
	if w.store == nil || w.api == nil || w.cipher == nil {
		return errors.New("X inbox cleanup worker is not fully configured")
	}
	cleanupCtx, cancel := context.WithTimeout(ctx, xInboxCleanupBudget)
	defer cancel()
	cleanupNow := w.now().UTC()
	cleanups, err := w.store.ClaimCleanupIntents(
		cleanupCtx,
		w.cleanupOwner,
		cleanupNow,
		cleanupNow.Add(xInboxCleanupLease),
		xInboxCleanupBatchLimit,
	)
	if err != nil {
		return fmt.Errorf("claim X inbox cleanup intents: %w", err)
	}
	var joined error
	for index, intent := range cleanups {
		if cleanupCtx.Err() != nil {
			for _, unprocessed := range cleanups[index:] {
				joined = errors.Join(
					joined,
					w.releaseFailedCleanup(
						context.Background(),
						unprocessed,
						fmt.Errorf("X inbox cleanup budget exhausted: %w", cleanupCtx.Err()),
					),
				)
			}
			break
		}
		if err := w.reconcileCleanupIntent(cleanupCtx, intent); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func (w *XInboxDeliveryWorker) reconcileAccount(
	ctx context.Context,
	account XInboxDeliveryAccount,
) (XInboxAppStream, bool, bool, error) {
	now := w.now().UTC()
	state := XInboxDeliveryState{
		SocialAccountID:                    account.SocialAccountID,
		FilteredStreamRuleID:               account.FilteredStreamRuleID,
		ActivityDMSubscriptionID:           account.ActivityDMSubscriptionID,
		ActivityWebhookRouteKey:            account.ActivityWebhookRouteKey,
		DMSubscriptionForbiddenFingerprint: account.DMSubscriptionForbiddenFingerprint,
		DeliveryStatus:                     xinbox.DeliveryStatusPending,
		LastSyncedAt:                       now,
	}
	targetStatus := xinbox.DeliveryStatusPending
	if !account.PlanAllowsInbox {
		targetStatus = xinbox.DeliveryStatusPausedPlan
	}
	commentsEligible := account.AccountActive && account.PlanAllowsInbox &&
		hasXInboxScopes(account.Scopes, "tweet.read", "tweet.write", "users.read")
	dmScopeEligible := account.AccountActive && account.PlanAllowsInbox &&
		hasXInboxScopes(account.Scopes, "dm.read")

	_, dmCanaryEligible := w.dmCanaryAccountIDs[account.SocialAccountID]
	appBearerToken, webhookRouteKey, consumerSecretConfigured, appTokenErr := w.resolveAccountAppToken(account)
	app := XInboxAppStream{
		Identity:                 safeAppIdentity(webhookRouteKey),
		WebhookRouteKey:          webhookRouteKey,
		BearerToken:              appBearerToken,
		ConsumerSecretConfigured: consumerSecretConfigured,
	}
	appModeSupported := account.AppMode == xinbox.AppModeUniPostManaged || account.AppMode == xinbox.AppModeWorkspace
	dmsAvailable := false
	var dmEvaluatorErr error
	dmEvaluationNeeded := dmScopeEligible && dmCanaryEligible && appModeSupported && appTokenErr == nil
	if dmEvaluationNeeded && w.dmsAvailable != nil {
		dmsAvailable, dmEvaluatorErr = w.dmsAvailable(ctx, account.WorkspaceID)
	}
	if (dmEvaluationNeeded && dmEvaluatorErr == nil && !dmsAvailable) || !dmCanaryEligible {
		state.DMSubscriptionForbiddenFingerprint = ""
	}
	commentsDesired := commentsEligible && appModeSupported && appTokenErr == nil
	dmGatesDesired := dmEvaluationNeeded && dmsAvailable && dmEvaluatorErr == nil &&
		dmCanaryEligible && appModeSupported && appTokenErr == nil

	var dmConfigErr error
	if dmGatesDesired {
		dmConfigErr = errors.Join(
			dmCredentialError(account.AppMode, app.ConsumerSecretConfigured),
			requiredDMConfigurationError(w.webhookURL, account.ExternalAccountID),
		)
	}
	dmsDesired := dmGatesDesired && dmConfigErr == nil

	if account.AppMode == xinbox.AppModeUniPostManaged && (commentsDesired || dmsDesired) {
		if w.usage == nil {
			cause := errors.New("spend safety source: usage reader is not configured")
			logXInboxSourceError(account, "shared", "evaluate_spend", cause)
			return app, commentsEligible && state.FilteredStreamRuleID != "", false, w.saveAccountError(ctx, state, cause)
		}
		snapshot, err := w.usage.Snapshot(ctx, account.WorkspaceID, now)
		if err != nil {
			cause := fmt.Errorf("spend safety source: %w", err)
			logXInboxSourceError(account, "shared", "evaluate_spend", cause)
			return app, commentsEligible && state.FilteredStreamRuleID != "", false, w.saveAccountError(ctx, state, cause)
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

	if appTokenErr != nil && (state.FilteredStreamRuleID != "" || state.ActivityDMSubscriptionID != "" || commentsEligible || dmScopeEligible) {
		cause := fmt.Errorf("shared app authentication source: %w", appTokenErr)
		logXInboxSourceError(account, "shared", "resolve_app_bearer", cause)
		return app, false, true, w.saveAccountError(ctx, state, cause)
	}
	streamDesired := func() bool {
		return commentsDesired && state.FilteredStreamRuleID != ""
	}
	var commentsErr error
	var dmErr error
	var dmCleanupErr error
	persist := func() error {
		state.LastSyncedAt = w.now().UTC()
		return w.store.SaveState(ctx, state)
	}

	if !commentsDesired && state.FilteredStreamRuleID != "" {
		if err := w.api.DeleteFilteredStreamRule(ctx, appBearerToken, state.FilteredStreamRuleID); err != nil {
			commentsErr = err
		} else {
			state.FilteredStreamRuleID = ""
			state.DeliveryStatus = targetStatus
			if err := persist(); err != nil {
				return app, streamDesired(), true, err
			}
		}
	}
	if commentsDesired && state.FilteredStreamRuleID == "" {
		if account.Handle == "" {
			commentsErr = errors.New("connected X account has no handle")
		} else {
			rule, err := w.api.EnsureFilteredStreamRule(ctx, appBearerToken, account.SocialAccountID, account.Handle)
			if err != nil {
				commentsErr = err
			} else {
				state.FilteredStreamRuleID = rule.ID
				if err := persist(); err != nil {
					return app, streamDesired(), true, err
				}
			}
		}
	}

	if !dmsDesired {
		if state.ActivityDMSubscriptionID != "" {
			if err := w.api.DeleteActivitySubscription(ctx, appBearerToken, state.ActivityDMSubscriptionID); err != nil {
				dmCleanupErr = err
			} else {
				state.ActivityDMSubscriptionID = ""
				state.ActivityWebhookRouteKey = ""
				state.DeliveryStatus = targetStatus
				if err := persist(); err != nil {
					return app, streamDesired(), true, err
				}
			}
		}
		if dmEvaluationNeeded && dmEvaluatorErr != nil {
			dmErr = fmt.Errorf("evaluate workspace DM availability: %w", dmEvaluatorErr)
		} else if dmConfigErr != nil {
			dmErr = dmConfigErr
		}
	} else {
		appWebhookURL, err := xinbox.AppWebhookURL(w.webhookURL, webhookRouteKey)
		if err != nil {
			dmErr = err
		} else {
			fingerprint := dmForbiddenFingerprint(account, app, appWebhookURL)
			if state.DMSubscriptionForbiddenFingerprint == fingerprint {
				dmErr = errors.New("DM subscription provisioning is latched after X returned HTTP 403")
			} else {
				if state.DMSubscriptionForbiddenFingerprint != "" {
					state.DMSubscriptionForbiddenFingerprint = ""
					if err := persist(); err != nil {
						return app, streamDesired(), true, err
					}
				}
				oldRouteKey := state.ActivityWebhookRouteKey
				if state.ActivityDMSubscriptionID != "" && oldRouteKey != webhookRouteKey {
					if err := w.api.DeleteActivitySubscription(ctx, appBearerToken, state.ActivityDMSubscriptionID); err != nil {
						dmCleanupErr = err
					} else {
						state.ActivityDMSubscriptionID = ""
						state.ActivityWebhookRouteKey = ""
						if err := persist(); err != nil {
							return app, streamDesired(), true, err
						}
					}
				}
				if dmCleanupErr == nil {
					webhook, ensureErr := w.api.EnsureWebhook(ctx, appBearerToken, appWebhookURL)
					if ensureErr != nil {
						dmErr = ensureErr
						if isDMProvisioningForbidden(ensureErr) {
							state.DMSubscriptionForbiddenFingerprint = fingerprint
							if err := persist(); err != nil {
								return app, streamDesired(), true, errors.Join(ensureErr, err)
							}
						}
					} else {
						subscription, ensureErr := w.api.EnsureDMSubscription(
							ctx,
							appBearerToken,
							account.SocialAccountID,
							account.ExternalAccountID,
							webhook.ID,
						)
						if ensureErr != nil {
							dmErr = ensureErr
							if isDMProvisioningForbidden(ensureErr) {
								state.DMSubscriptionForbiddenFingerprint = fingerprint
								if err := persist(); err != nil {
									return app, streamDesired(), true, errors.Join(ensureErr, err)
								}
							}
						} else {
							state.ActivityDMSubscriptionID = subscription.ID
							state.ActivityWebhookRouteKey = webhookRouteKey
							state.DMSubscriptionForbiddenFingerprint = ""
							if err := persist(); err != nil {
								return app, streamDesired(), true, err
							}
							dmCleanupErr = w.cleanupStaleDMWebhooks(
								ctx,
								appBearerToken,
								webhook,
								appWebhookURL,
								webhookRouteKey,
								oldRouteKey,
							)
						}
					}
				}
			}
		}
	}
	if commentsDesired || dmsDesired {
		state.DeliveryStatus = xinbox.DeliveryStatusActive
	} else {
		state.DeliveryStatus = targetStatus
	}
	var joined error
	lastError := ""
	if commentsErr != nil {
		logXInboxSourceError(account, "comments", "reconcile", commentsErr)
		sourceErr := fmt.Errorf("comments source: %w", commentsErr)
		joined = errors.Join(joined, sourceErr)
		lastError = sourceErr.Error()
	}
	if dmErr != nil {
		logXInboxSourceError(account, "dm", "reconcile", dmErr)
		sourceErr := fmt.Errorf("dm source: %w", dmErr)
		joined = errors.Join(joined, sourceErr)
		lastError = sourceErr.Error()
	}
	if dmCleanupErr != nil {
		logXInboxSourceError(account, "dm", "cleanup", dmCleanupErr)
		sourceErr := fmt.Errorf("dm cleanup source: %w", dmCleanupErr)
		joined = errors.Join(joined, sourceErr)
		lastError = sourceErr.Error()
	}
	if joined != nil {
		state.DeliveryStatus = xinbox.DeliveryStatusError
		state.LastError = lastError
	} else {
		state.LastError = ""
	}
	if err := persist(); err != nil {
		return app, streamDesired(), true, err
	}
	return app, streamDesired(), true, joined
}

func dmCredentialError(appMode xinbox.AppMode, consumerSecretConfigured bool) error {
	switch appMode {
	case xinbox.AppModeUniPostManaged:
		if !consumerSecretConfigured {
			return errors.New("TWITTER_CONSUMER_SECRET is not configured")
		}
	case xinbox.AppModeWorkspace:
		if !consumerSecretConfigured {
			return errors.New("workspace X app consumer_secret is not configured")
		}
	}
	return nil
}

func requiredDMConfigurationError(webhookURL, externalAccountID string) error {
	var joined error
	if strings.TrimSpace(webhookURL) == "" {
		joined = errors.Join(joined, errors.New("X_INBOX_WEBHOOK_URL is not configured"))
	}
	if strings.TrimSpace(externalAccountID) == "" {
		joined = errors.Join(joined, errors.New("connected X account has no provider user ID"))
	}
	return joined
}

func dmForbiddenFingerprint(
	account XInboxDeliveryAccount,
	app XInboxAppStream,
	appWebhookURL string,
) string {
	hash := sha256.New()
	for _, value := range []string{
		string(account.AppMode),
		app.Identity,
		account.SocialAccountID,
		account.ExternalAccountID,
		appWebhookURL,
		"dm.received",
	} {
		_, _ = fmt.Fprintf(hash, "%d:%s|", len(value), value)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func isDMProvisioningForbidden(err error) bool {
	var providerErr *xinbox.ProviderHTTPError
	if !errors.As(err, &providerErr) || providerErr == nil || providerErr.StatusCode != http.StatusForbidden {
		return false
	}
	switch providerErr.Method {
	case http.MethodGet, http.MethodPut, http.MethodPost:
		return true
	default:
		return false
	}
}

func (w *XInboxDeliveryWorker) cleanupStaleDMWebhooks(
	ctx context.Context,
	appBearerToken string,
	current xinbox.Webhook,
	currentURL string,
	currentRouteKey string,
	oldRouteKey string,
) error {
	webhooks, err := w.api.ListWebhooks(ctx, appBearerToken)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(webhooks))
	var joined error
	for _, candidate := range webhooks {
		if candidate.ID == current.ID {
			continue
		}
		if _, duplicate := seen[candidate.ID]; duplicate {
			continue
		}
		seen[candidate.ID] = struct{}{}
		routeKey, ok := webhookURLRouteKey(candidate.URL)
		if !ok {
			continue
		}
		staleOldRoute := oldRouteKey != "" && oldRouteKey != currentRouteKey && routeKey == oldRouteKey
		staleCurrentBase := routeKey == currentRouteKey && candidate.URL != currentURL
		if !staleOldRoute && !staleCurrentBase {
			continue
		}
		if err := w.api.DeleteWebhook(ctx, appBearerToken, candidate.ID); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func webhookURLRouteKey(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	escapedPath := parsed.EscapedPath()
	separator := strings.LastIndexByte(escapedPath, '/')
	if separator < 0 || separator == len(escapedPath)-1 {
		return "", false
	}
	routeKey, err := url.PathUnescape(escapedPath[separator+1:])
	if err != nil || routeKey == "" {
		return "", false
	}
	return routeKey, true
}

func logXInboxSourceError(
	account XInboxDeliveryAccount,
	source string,
	action string,
	err error,
) {
	status := 0
	var providerErr *xinbox.ProviderHTTPError
	if errors.As(err, &providerErr) && providerErr != nil {
		status = providerErr.StatusCode
	}
	slog.Warn(
		"X inbox delivery source reconciliation failed",
		"app_mode", account.AppMode,
		"app_identity", safeAppIdentity(account.WebhookRouteKey),
		"social_account_id", account.SocialAccountID,
		"workspace_id", account.WorkspaceID,
		"source", source,
		"action", action,
		"provider_http_status", status,
		"error", err,
	)
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
	releaseCtx := ctx
	var cancel context.CancelFunc
	if ctx.Err() != nil {
		releaseCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	if err := w.store.ReleaseCleanupIntent(releaseCtx, intent, nextAttemptAt); err != nil {
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

func (w *XInboxDeliveryWorker) resolveAccountAppToken(
	account XInboxDeliveryAccount,
) (string, string, bool, error) {
	switch account.AppMode {
	case xinbox.AppModeUniPostManaged:
		if w.managedAppBearer == "" {
			return "", "", w.managedConsumerSecretConfigured, errors.New("TWITTER_BEARER_TOKEN is not configured")
		}
		if account.WebhookRouteKey == "" {
			return "", "", w.managedConsumerSecretConfigured, errors.New("managed X webhook route key is not configured")
		}
		return w.managedAppBearer, account.WebhookRouteKey, w.managedConsumerSecretConfigured, nil
	case xinbox.AppModeWorkspace:
		if account.AppBearerTokenEncrypted == "" {
			return "", "", account.ConsumerSecretConfigured, errors.New("workspace X app bearer token is not configured")
		}
		token, err := w.cipher.Decrypt(account.AppBearerTokenEncrypted)
		if err != nil {
			return "", "", account.ConsumerSecretConfigured, fmt.Errorf("decrypt workspace X app bearer token: %w", err)
		}
		if account.WebhookRouteKey == "" {
			return "", "", account.ConsumerSecretConfigured, errors.New("workspace X app webhook route key is not configured")
		}
		return token, account.WebhookRouteKey, account.ConsumerSecretConfigured, nil
	default:
		return "", "", false, fmt.Errorf("unsupported X app mode %q", account.AppMode)
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

func (w *XInboxDeliveryWorker) syncDesiredStreams(ctx context.Context, apps []XInboxAppStream) {
	desired := make(map[string]XInboxAppStream, len(apps))
	for _, app := range apps {
		desired[app.Identity] = app
	}

	type stoppedStream struct {
		cancel context.CancelFunc
		done   <-chan struct{}
	}
	var stopped []stoppedStream
	w.streamsMu.Lock()
	for identity, active := range w.streams {
		app, exists := desired[identity]
		if exists && active.fingerprint == bearerFingerprint(app.BearerToken) {
			continue
		}
		stopped = append(stopped, stoppedStream{cancel: active.cancel, done: active.done})
	}
	w.streamsMu.Unlock()

	for _, active := range stopped {
		active.cancel()
		select {
		case <-active.done:
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}

	w.streamsMu.Lock()
	defer w.streamsMu.Unlock()
	for identity, app := range desired {
		fingerprint := bearerFingerprint(app.BearerToken)
		if active, exists := w.streams[identity]; exists {
			if active.fingerprint == fingerprint {
				continue
			}
			continue
		}
		w.startAppStreamLocked(ctx, app, fingerprint)
	}
}

func (w *XInboxDeliveryWorker) startAppStreamLocked(
	ctx context.Context,
	app XInboxAppStream,
	fingerprint string,
) {
	streamCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	w.streamGeneration++
	generation := w.streamGeneration
	w.streams[app.Identity] = managedXInboxStream{
		cancel:      cancel,
		done:        done,
		fingerprint: fingerprint,
		generation:  generation,
	}
	go func() {
		defer func() {
			w.streamsMu.Lock()
			if active, exists := w.streams[app.Identity]; exists && active.generation == generation {
				delete(w.streams, app.Identity)
			}
			w.streamsMu.Unlock()
			close(done)
		}()
		if err := w.runAppStreamWithCancel(streamCtx, app, cancel); err != nil &&
			streamCtx.Err() == nil {
			slog.Warn("X inbox filtered stream stopped", "app_identity", app.Identity, "error", err)
		}
	}()
}

func (w *XInboxDeliveryWorker) runAppStream(ctx context.Context, app XInboxAppStream) error {
	return w.runAppStreamWithCancel(ctx, app, func() {})
}

func (w *XInboxDeliveryWorker) runAppStreamWithCancel(
	ctx context.Context,
	app XInboxAppStream,
	cancel context.CancelFunc,
) error {
	if w.leader == nil || w.stream == nil {
		return nil
	}
	lease, acquired, err := w.leader.TryAcquire(
		ctx,
		"x-inbox-stream:"+app.Identity,
		cancel,
	)
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
		return w.eventHandler(ctx, app.WebhookRouteKey, event)
	}
	return w.stream.Run(ctx, app.Identity, app.BearerToken, handler)
}

func (w *XInboxDeliveryWorker) stopAllStreams() {
	w.streamsMu.Lock()
	streams := make([]managedXInboxStream, 0, len(w.streams))
	for _, stream := range w.streams {
		streams = append(streams, stream)
	}
	w.streamsMu.Unlock()
	for _, stream := range streams {
		stream.cancel()
	}
}

func bearerFingerprint(bearer string) string {
	sum := sha256.Sum256([]byte(bearer))
	return hex.EncodeToString(sum[:])
}

func safeAppIdentity(webhookRouteKey string) string {
	sum := sha256.Sum256([]byte(webhookRouteKey))
	return "x-app:" + hex.EncodeToString(sum[:])
}

type postgresXInboxDeliveryStore struct {
	pool                   *pgxpool.Pool
	queries                *db.Queries
	managedWebhookRouteKey string
}

func (s *postgresXInboxDeliveryStore) ListAccounts(ctx context.Context) ([]XInboxDeliveryAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			sa.id,
			p.workspace_id,
			CASE
			  WHEN COALESCE(sa.x_app_mode, 'legacy_unknown') = 'unipost_managed_app' THEN $1
			  ELSE COALESCE(pc.webhook_route_key, '')
			END,
			COALESCE(sa.account_name, ''),
			COALESCE(sa.external_account_id, ''),
			COALESCE(sa.x_app_mode, 'legacy_unknown'),
			COALESCE(pc.app_bearer_token, ''),
			COALESCE(NULLIF(pc.consumer_secret, ''), '') <> '',
			sa.scope,
			(sa.disconnected_at IS NULL AND sa.status = 'active') AS account_active,
			COALESCE(pl.allow_inbox, FALSE) AS plan_allows_inbox,
			COALESCE(r.filtered_stream_rule_id, ''),
			COALESCE(r.activity_dm_subscription_id, ''),
			COALESCE(r.activity_webhook_route_key, ''),
			COALESCE(r.dm_subscription_forbidden_fingerprint, '')
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
	`, s.managedWebhookRouteKey)
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
			&account.WebhookRouteKey,
			&account.Handle,
			&account.ExternalAccountID,
			&appMode,
			&account.AppBearerTokenEncrypted,
			&account.ConsumerSecretConfigured,
			&account.Scopes,
			&account.AccountActive,
			&account.PlanAllowsInbox,
			&account.FilteredStreamRuleID,
			&account.ActivityDMSubscriptionID,
			&account.ActivityWebhookRouteKey,
			&account.DMSubscriptionForbiddenFingerprint,
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
		SocialAccountID:                    state.SocialAccountID,
		FilteredStreamRuleID:               nullableText(state.FilteredStreamRuleID),
		ActivityDmSubscriptionID:           nullableText(state.ActivityDMSubscriptionID),
		ActivityWebhookRouteKey:            nullableText(state.ActivityWebhookRouteKey),
		DmSubscriptionForbiddenFingerprint: nullableText(state.DMSubscriptionForbiddenFingerprint),
		DeliveryStatus:                     state.DeliveryStatus,
		LastError:                          nullableText(state.LastError),
		LastSyncedAt:                       pgtype.Timestamptz{Time: state.LastSyncedAt, Valid: !state.LastSyncedAt.IsZero()},
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
			i.cleanup_key,
			i.social_account_id,
			i.x_app_mode,
			i.source_app_identity,
			COALESCE(i.app_bearer_token, ''),
			COALESCE(i.webhook_route_key, ''),
			COALESCE(i.consumer_secret, ''),
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
			&intent.CleanupKey,
			&intent.SocialAccountID,
			&appMode,
			&intent.SourceAppIdentity,
			&intent.AppBearerTokenEncrypted,
			&intent.WebhookRouteKey,
			&intent.ConsumerSecretEncrypted,
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
