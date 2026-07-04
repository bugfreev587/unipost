package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	slogbetterstack "github.com/samber/slog-betterstack"

	"github.com/xiaoboyu/unipost-api/internal/aiproviders"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/changelog"
	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
	"github.com/xiaoboyu/unipost-api/internal/errortriage"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/handler"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
	"github.com/xiaoboyu/unipost-api/internal/loops"
	"github.com/xiaoboyu/unipost-api/internal/mail"
	"github.com/xiaoboyu/unipost-api/internal/metrics"
	mw "github.com/xiaoboyu/unipost-api/internal/middleware"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/quotaemail"
	"github.com/xiaoboyu/unipost-api/internal/ratelimit"
	appredis "github.com/xiaoboyu/unipost-api/internal/redis"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
	"github.com/xiaoboyu/unipost-api/internal/storage"
	"github.com/xiaoboyu/unipost-api/internal/worker"
	"github.com/xiaoboyu/unipost-api/internal/ws"
)

func main() {
	_ = godotenv.Load()

	// Set up structured JSON logging with optional BetterStack
	stdoutHandler := slog.NewJSONHandler(os.Stdout, nil)

	var logHandler slog.Handler
	if bsToken := os.Getenv("BETTERSTACK_TOKEN"); bsToken != "" {
		bsHandler := slogbetterstack.Option{Token: bsToken}.NewBetterstackHandler()
		logHandler = fanoutHandler{handlers: []slog.Handler{stdoutHandler, bsHandler}}
	} else {
		logHandler = stdoutHandler
	}

	logger := slog.New(logHandler)
	slog.SetDefault(logger)
	slog.Info("runtime environment detected", "env", runtimeenv.Current(), "production", runtimeenv.IsProduction())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	// Initialize AES encryptor for token encryption
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		slog.Error("ENCRYPTION_KEY is required")
		os.Exit(1)
	}
	encryptor, err := crypto.NewAESEncryptor(encryptionKey)
	if err != nil {
		slog.Error("failed to initialize encryptor", "error", err)
		os.Exit(1)
	}

	// Stripe billing manager is constructed below, after the DB pool is
	// ready, because SUPER_ADMINS entries can be email addresses that
	// the manager resolves via a DB lookup.
	var stripeMgr *billing.Manager

	// Register platform adapters
	platform.Register(platform.NewBlueskyAdapter())
	platform.Register(platform.NewLinkedInAdapter())
	platform.Register(platform.NewInstagramAdapter())
	platform.Register(platform.NewThreadsAdapter())

	platform.Register(platform.NewTwitterAdapter()) // Native mode only — requires user's own API credentials

	ctx := context.Background()

	// Optional R2-backed media proxy. Required for TikTok photo posts
	// (TikTok's photo Direct Post only accepts PULL_FROM_URL from
	// developer-verified domains, so we stage user-supplied images on a
	// single bucket whose URL prefix is registered in our TikTok dev
	// portal). Other adapters can use it later if needed.
	var storageClient *storage.Client
	if os.Getenv("R2_ACCOUNT_ID") != "" {
		mp, mpErr := storage.New(ctx, storage.Config{
			AccountID:       os.Getenv("R2_ACCOUNT_ID"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			Bucket:          os.Getenv("R2_BUCKET_NAME"),
			PublicDomain:    os.Getenv("R2_PUBLIC_DOMAIN"),
		})
		if mpErr != nil {
			slog.Error("media proxy init failed; tiktok photo posts will be disabled", "error", mpErr)
		} else {
			storageClient = mp
			slog.Info("media proxy initialized", "bucket", os.Getenv("R2_BUCKET_NAME"))
		}
	} else {
		slog.Info("R2_ACCOUNT_ID not set; tiktok photo posts will be disabled")
	}

	pinterestAdapter := platform.NewPinterestAdapter()
	pinterestAdapter.SetMediaProxy(storageClient)
	platform.Register(pinterestAdapter)
	slog.Info("pinterest adapter registered", "media_proxy", storageClient != nil)

	// Conditionally register adapters that need credentials
	if os.Getenv("TIKTOK_CLIENT_KEY") != "" {
		tiktokAdapter := platform.NewTikTokAdapter()
		tiktokAdapter.SetMediaProxy(storageClient)
		platform.Register(tiktokAdapter)
		slog.Info("tiktok adapter registered", "media_proxy", storageClient != nil)
	}

	// Facebook adapter is registered here (not above with the
	// non-credential adapters) because it needs the media proxy to
	// stage video uploads — without a public, long-lived R2 URL,
	// FB's async /videos fetch races our 15-minute presigned URLs
	// and leaves videos stuck in video_status=uploading.
	fbAdapter := platform.NewFacebookAdapter()
	fbAdapter.SetMediaProxy(storageClient)
	platform.Register(fbAdapter)
	slog.Info("facebook adapter registered", "media_proxy", storageClient != nil)
	if os.Getenv("YOUTUBE_CLIENT_ID") != "" {
		platform.Register(platform.NewYouTubeAdapter())
		slog.Info("youtube adapter registered")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("unable to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// Run database migrations
	if err := db.RunMigrations(databaseURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	queries := db.New(pool)
	aiProviderService := aiproviders.NewService(queries, encryptor)
	integrationLogger := integrationlogs.NewLogger(queries, func(ctx context.Context, row db.IntegrationLog) {
		ws.NotifyLog(ctx, pool, ws.LogEnvelope(row))
	})

	// Build the Stripe billing manager now that the DB is ready. The
	// SUPER_ADMINS list may contain email addresses, which the manager
	// resolves to Clerk user IDs lazily via a closure over queries.GetUser.
	if os.Getenv("STRIPE_SECRET_KEY") != "" {
		userLookup := func(ctx context.Context, userID string) (string, error) {
			user, err := queries.GetUser(ctx, userID)
			if err != nil {
				return "", err
			}
			return user.Email, nil
		}
		mgr, mgrErr := billing.NewManager(userLookup)
		if mgrErr != nil {
			slog.Error("stripe billing manager init failed", "error", mgrErr)
			os.Exit(1)
		}
		stripeMgr = mgr
	}

	// Sync Stripe price IDs from env vars into plans table
	syncStripePriceIDs(ctx, queries)
	quotaChecker := quota.NewChecker(queries)

	// Rate-limit / queue-admission infrastructure (April 2026 PRD).
	// REDIS_URL points at Railway-internal Redis; absence is allowed
	// for local dev and falls through to NoopLimiter, with a loud
	// startup warning so misconfigured prod is obvious.
	redisClient, err := appredis.New(ctx, os.Getenv("REDIS_URL"))
	if err != nil {
		slog.Error("redis: connection failed; rate limiter will run in degraded mode", "error", err)
		// Don't os.Exit — the limiter falls open via its circuit
		// breaker and Postgres-backed depth check still works. We'd
		// rather a degraded-but-running API than a full outage on a
		// Redis blip during boot.
	}
	var limiter ratelimit.Limiter
	if redisClient != nil {
		limiter = ratelimit.NewRedisLimiter(redisClient, queries)
		slog.Info("ratelimit: redis-backed limiter active")
	} else {
		limiter = ratelimit.NoopLimiter{}
		slog.Warn("ratelimit: REDIS_URL unset; admission control disabled (NoopLimiter)")
	}

	// Start background workers
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	go integrationLogger.Start(workerCtx)

	tokenWorker := worker.NewTokenRefreshWorker(queries, encryptor)
	go tokenWorker.Start(workerCtx)

	// Sprint 3 PR3/PR4/PR7: managed Connect registry. Built early so
	// the managed token refresh worker can take it as a dependency.
	// Connectors return nil from their constructors when env vars are
	// missing, so a half-configured environment simply doesn't have
	// those platforms registered.
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "https://api.unipost.dev"
	}
	connectors := []connect.Connector{}
	if tw := connect.NewTwitterConnector(os.Getenv("TWITTER_CLIENT_ID"), os.Getenv("TWITTER_CLIENT_SECRET"), apiBaseURL); tw != nil {
		connectors = append(connectors, tw)
	}
	if li := connect.NewLinkedInConnector(os.Getenv("LINKEDIN_CLIENT_ID"), os.Getenv("LINKEDIN_CLIENT_SECRET"), apiBaseURL); li != nil {
		connectors = append(connectors, li)
	}
	if ig := connect.NewInstagramConnector(os.Getenv("INSTAGRAM_APP_ID"), os.Getenv("INSTAGRAM_APP_SECRET"), apiBaseURL); ig != nil {
		connectors = append(connectors, ig)
	}
	if tt := connect.NewTikTokConnector(os.Getenv("TIKTOK_CLIENT_KEY"), os.Getenv("TIKTOK_CLIENT_SECRET"), apiBaseURL); tt != nil {
		connectors = append(connectors, tt)
	}
	if th := connect.NewThreadsConnector(os.Getenv("THREADS_APP_ID"), os.Getenv("THREADS_APP_SECRET"), apiBaseURL); th != nil {
		connectors = append(connectors, th)
	}
	if fb := connect.NewFacebookConnector(firstEnv("FACEBOOK_APP_ID", "INSTAGRAM_APP_ID"), firstEnv("FACEBOOK_APP_SECRET", "INSTAGRAM_APP_SECRET"), apiBaseURL); fb != nil {
		connectors = append(connectors, fb)
	}
	if pin := connect.NewPinterestConnector(firstEnv("PINTEREST_APP_ID", "PINTEREST_CLIENT_ID"), firstEnv("PINTEREST_APP_SECRET", "PINTEREST_CLIENT_SECRET"), apiBaseURL); pin != nil {
		connectors = append(connectors, pin)
	}
	if yt := connect.NewYouTubeConnector(os.Getenv("YOUTUBE_CLIENT_ID"), os.Getenv("YOUTUBE_CLIENT_SECRET"), apiBaseURL); yt != nil {
		connectors = append(connectors, yt)
	}
	connectRegistry := connect.NewRegistry(connectors...)

	// Sprint 3 PR7: managed token refresh worker. Runs every 5 min,
	// refreshes tokens within a 30 min window of expiry, uses
	// FOR UPDATE SKIP LOCKED so concurrent API instances pick disjoint
	// slices. Started after webhookWorker so it has a real bus to
	// fire account.disconnected events into. Defer the goroutine
	// start until after webhookWorker is constructed below.

	// Webhook delivery worker doubles as the EventBus implementation
	// for the publish path. Constructed before the scheduler /
	// handlers so they can be wired with it as their bus dependency.
	webhookWorker := worker.NewWebhookDeliveryWorker(queries, integrationLogger)
	go webhookWorker.Start(workerCtx)

	// User-facing notifications (migration 040). Dispatcher receives
	// events alongside the webhook bus via events.MultiBus below; the
	// delivery worker drains the queue in the background and sends via
	// the configured mailer. Unset RESEND_API_KEY → NoopMailer so local
	// dev can't accidentally email real users.
	var mailer mail.Mailer = mail.NoopMailer{}
	if key := os.Getenv("RESEND_API_KEY"); key != "" {
		from := os.Getenv("RESEND_FROM")
		if from == "" {
			from = "UniPost <notifications@unipost.dev>"
		}
		mailer = mail.NewResendMailer(key, from)
	} else {
		slog.Info("notifications: RESEND_API_KEY unset, using NoopMailer")
	}

	var loopsClient *loops.Client
	var auditedLoopsClient *loops.AuditedClient
	var loopsSyncer *loops.Syncer
	if key := os.Getenv("LOOPS_API_KEY"); key != "" {
		loopsClient = loops.NewClient(loops.Config{
			APIKey:  key,
			BaseURL: os.Getenv("LOOPS_BASE_URL"),
		})
		emailAuditStore := loops.NewPostgresEmailAuditStore(queries)
		auditedLoopsClient = loops.NewAuditedClient(loopsClient, emailAuditStore)
		loopsSyncer = loops.NewSyncer(loopsClient, loops.Options{
			TransactionalIDs: loops.TransactionalIDs{
				PlanChanged:                 os.Getenv("LOOPS_PLAN_CHANGED_TRANSACTIONAL_ID"),
				BillingPaymentFailed:        os.Getenv("LOOPS_BILLING_PAYMENT_FAILED_TRANSACTIONAL_ID"),
				BillingPaymentRecovered:     os.Getenv("LOOPS_BILLING_PAYMENT_RECOVERED_TRANSACTIONAL_ID"),
				BillingSubscriptionCanceled: os.Getenv("LOOPS_BILLING_SUBSCRIPTION_CANCELED_TRANSACTIONAL_ID"),
				AccountDisconnected:         os.Getenv("LOOPS_ACCOUNT_DISCONNECTED_TRANSACTIONAL_ID"),
				AccountCanceled:             os.Getenv("LOOPS_ACCOUNT_CANCELED_TRANSACTIONAL_ID"),
				PostFailed:                  os.Getenv("LOOPS_POST_FAILED_TRANSACTIONAL_ID"),
			},
			EmailAuditStore: emailAuditStore,
			EmailPolicy: emailpolicy.NewService(
				emailpolicy.NewPostgresPreferenceReader(queries),
				os.Getenv("APP_BASE_URL"),
			),
		})
		slog.Info("loops: lifecycle sync configured")
	} else {
		slog.Info("loops: LOOPS_API_KEY unset, lifecycle sync disabled")
	}
	var freePlanQuotaEmailService *quotaemail.Service
	if loopsClient != nil && os.Getenv("LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID") != "" {
		freePlanQuotaEmailService = quotaemail.NewService(quotaemail.Config{
			Store:           quotaemail.NewPostgresStore(queries),
			Sender:          loopsClient,
			TransactionalID: os.Getenv("LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID"),
			PricingURL:      "https://unipost.dev/pricing",
			AppBaseURL:      os.Getenv("APP_BASE_URL"),
		})
		slog.Info("quota email: free plan quota reminder configured")
	} else {
		slog.Info("quota email: free plan quota reminder disabled; Loops client or transactional ID missing")
	}

	notificationDispatcher := worker.NewNotificationDispatcher(queries)
	loopsNotificationBus := worker.NewLoopsNotificationEmailBus(queries, loopsSyncer, os.Getenv("APP_BASE_URL"))
	notificationWorker := worker.NewNotificationDeliveryWorker(queries, mailer, os.Getenv("APP_BASE_URL"))
	go notificationWorker.Start(workerCtx)

	// One Publish() call feeds both the developer webhook system and
	// the user notification system. Handler code depends on
	// events.EventBus so nothing else has to change.
	eventBus := events.NewMultiBus(webhookWorker, notificationDispatcher, loopsNotificationBus)
	socialPostHandler := handler.NewSocialPostHandler(queries, encryptor, quotaChecker, eventBus, storageClient, limiter, integrationLogger).
		SetAppBaseURL(os.Getenv("APP_BASE_URL")).
		SetLoopsSyncer(loopsSyncer).
		SetQuotaEmailService(freePlanQuotaEmailService)

	// Sprint 3 PR7: managed token refresh worker. Started here so
	// the bus dependency (eventBus) is already wired.
	managedTokenWorker := worker.NewManagedTokenRefreshWorker(queries, encryptor, connectRegistry, eventBus, apiBaseURL)
	go managedTokenWorker.Start(workerCtx)

	schedulerWorker := worker.NewSchedulerWorker(queries, socialPostHandler)
	go schedulerWorker.Start(workerCtx)

	dispatchWorker := worker.NewPostDispatchWorker(queries, socialPostHandler)
	go dispatchWorker.Start(workerCtx)

	retryWorker := worker.NewPostRetryWorker(queries, socialPostHandler)
	go retryWorker.Start(workerCtx)

	postDeliveryCleanupWorker := worker.NewPostDeliveryCleanupWorker(socialPostHandler)
	go postDeliveryCleanupWorker.Start(workerCtx)

	analyticsRefreshWorker := worker.NewAnalyticsRefreshWorker(queries, encryptor, storageClient)
	go analyticsRefreshWorker.Start(workerCtx)

	mediaCleanupWorker := worker.NewMediaCleanupWorker(queries, storageClient)
	go mediaCleanupWorker.Start(workerCtx)

	logRetentionWorker := worker.NewIntegrationLogRetentionWorker(pool, queries)
	go logRetentionWorker.Start(workerCtx)

	errorTriageStore := errortriage.NewPostgresStore(pool)
	errorTriageAnalyzer := errortriage.NewProviderAnalyzer(aiProviderService, errortriage.DeterministicAnalyzer{})
	errorTriageService := errortriage.NewService(errorTriageStore, errorTriageAnalyzer)
	errorTriageEmailService := errortriage.NewEmailSendService(
		errorTriageStore,
		errortriage.NewLoopsSender(loopsClient),
		os.Getenv("LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID"),
	)
	errorTriageWorker := worker.NewErrorTriageWorker(errorTriageService)
	go errorTriageWorker.Start(workerCtx)

	// Facebook's /videos endpoint returns a video_id immediately and
	// finishes asynchronously. The initial publish poll waits 60s;
	// beyond that the row sits in `processing` until someone opens the
	// post in the dashboard. This worker picks up the slack so the
	// flip to published/failed happens on its own.
	facebookVideoStatusWorker := worker.NewFacebookVideoStatusWorker(queries, encryptor)
	go facebookVideoStatusWorker.Start(workerCtx)

	inboxSyncWorker := worker.NewInboxSyncWorker(queries, encryptor, pool)
	go inboxSyncWorker.Start(workerCtx)

	// WebSocket hubs for real-time inbox and logs delivery.
	inboxHub := ws.NewHub()
	logsHub := ws.NewHub()
	pgListener := ws.NewPGListener(inboxHub, logsHub, pool)
	go pgListener.Start(workerCtx)

	r := chi.NewRouter()

	// Global middleware
	r.Use(mw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsAllowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "X-Request-Id", "X-UniPost-Usage", "X-UniPost-Warning", "X-UniPost-RateLimit-Limit", "X-UniPost-RateLimit-Remaining", "X-UniPost-RateLimit-Reset", "X-UniPost-QueueDepth", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Handlers
	healthHandler := handler.NewHealthHandler()
	webhookHandler := handler.NewWebhookHandler(queries, mailer, os.Getenv("APP_BASE_URL")).SetLoopsSyncer(loopsSyncer)
	if auditedLoopsClient != nil {
		webhookHandler.SetWelcomeEmailSender(auditedLoopsClient, os.Getenv("LOOPS_USER_WELCOME_TRANSACTIONAL_ID"))
	}
	profileHandler := handler.NewProfileHandler(queries, quotaChecker)
	if storageClient != nil {
		profileHandler.SetBrandingLogoStore(storageClient)
	}
	workspaceHandler := handler.NewWorkspaceHandler(queries)
	apiKeyHandler := handler.NewAPIKeyHandler(queries)
	cliSetupTokenHandler := handler.NewCLISetupTokenHandler(queries).WithAPIBaseURL(os.Getenv("API_BASE_URL"))
	webhookSubHandler := handler.NewWebhookSubscriptionHandler(queries)
	superAdminChecker := auth.NewSuperAdminChecker(queries)
	socialAccountHandler := handler.NewSocialAccountHandler(queries, encryptor, eventBus, superAdminChecker)
	oauthHandler := handler.NewOAuthHandler(queries, encryptor, superAdminChecker).SetIntegrationLogger(integrationLogger)
	platformCredHandler := handler.NewPlatformCredentialHandler(queries, encryptor, quotaChecker)
	billingHandler := handler.NewBillingHandler(queries, quotaChecker, stripeMgr)
	stripeWebhookHandler := handler.NewStripeWebhookHandler(queries, stripeMgr, eventBus, os.Getenv("APP_BASE_URL")).SetLoopsSyncer(loopsSyncer)
	analyticsHandler := handler.NewAnalyticsHandler(queries, encryptor)
	// Sprint 5 PR1: GET /v1/analytics/rollup uses raw pgx for the
	// dynamic GROUP BY clause sqlc can't model.
	analyticsRollupHandler := handler.NewAnalyticsRollupHandler(pool)
	analyticsExplorerHandler := handler.NewAnalyticsExplorerHandler(pool)
	platformHandler := handler.NewPlatformHandler(queries)
	mediaHandler := handler.NewMediaHandler(queries, storageClient)
	mediaAudioOverlayHandler := handler.NewMediaAudioOverlayHandler(queries, storageClient)
	apiMetricsHandler := handler.NewAPIMetricsHandler(queries)
	adminAPIMetricsHandler := handler.NewAdminAPIMetricsHandler(queries)
	adminSearchHistoryHandler := handler.NewAdminSearchHistoryHandler(queries, superAdminChecker)
	apiMetricsRecorder := metrics.NewRecorder(queries)
	landingAttributionHandler := handler.NewLandingAttributionHandler(pool)
	adminChecker := auth.NewAdminChecker(queries)
	meHandler := handler.NewMeHandler(queries, adminChecker, superAdminChecker).SetQuotaChecker(quotaChecker).SetLoopsSyncer(loopsSyncer)
	aiPostAssistHandler := handler.NewAIPostAssistHandler(queries, superAdminChecker).WithAIProviders(aiProviderService)
	// Sprint 3 PR2: Connect sessions handler. Reuses NEXT_PUBLIC_APP_URL
	// for the hosted-page origin so the same env var that drives the
	// preview link drives the connect link.
	connectSessionHandler := handler.NewConnectSessionHandler(queries, os.Getenv("NEXT_PUBLIC_APP_URL"), quotaChecker).SetIntegrationLogger(integrationLogger)
	// Sprint 3 PR5: Bluesky Connect form handler. No API key — the
	// session id + oauth_state act as the bearer. Server-renders an
	// HTML form so the app password never touches dashboard JS.
	connectBlueskyHandler := handler.NewConnectBlueskyHandler(queries, encryptor, eventBus)
	// Sprint 4 PR5: Managed Users view (one row per end user across
	// all their connected social accounts).
	managedUsersHandler := handler.NewManagedUsersHandler(queries)
	// Sprint 4 PR7: Meta App Review data-deletion callback. Endpoint
	// is mandatory for App Review submission. META_APP_SECRET will be
	// set in Railway when business verification clears + the Meta
	// integration goes live; until then the handler returns 503
	// NOT_CONFIGURED for any inbound requests.
	metaDataDeletionHandler := handler.NewMetaDataDeletionHandler(
		queries,
		os.Getenv("META_APP_SECRET"),
		"https://app.unipost.dev/meta/data-deletion-status",
	)
	// Meta platform webhooks (Instagram + Threads). Single endpoint
	// for both products since they share the same Meta App. Uses
	// META_APP_SECRET for HMAC verification (already in env) and
	// META_WEBHOOK_VERIFY_TOKEN for the subscribe handshake.
	metaWebhookHandler := handler.NewMetaWebhookHandler(
		queries,
		pool,
		encryptor,
		os.Getenv("META_APP_SECRET"),
		os.Getenv("META_WEBHOOK_VERIFY_TOKEN"),
	)
	// connectRegistry was built in the worker section above so the
	// managed token refresh worker could take it as a dependency.
	// We just hand the same registry to the callback handler here.
	connectCallbackHandler := handler.NewConnectCallbackHandler(queries, encryptor, webhookWorker, connectRegistry, apiBaseURL, superAdminChecker).SetIntegrationLogger(integrationLogger)
	// Preview handler shares the dashboard origin (B3) and reuses
	// the ENCRYPTION_KEY value as the HMAC secret with an audience
	// claim for domain separation (B2). No new env var.
	previewHandler := handler.NewPreviewHandler(queries, storageClient, []byte(encryptionKey), os.Getenv("NEXT_PUBLIC_APP_URL"))
	adminHandler := handler.NewAdminHandler(pool, stripeMgr, queries)
	supportBundleHandler := handler.NewSupportBundleHandler(queries)
	aiProviderHandler := handler.NewAIProviderHandler(aiProviderService)
	errorTriageHandler := handler.NewErrorTriageHandler(errorTriageStore, errorTriageService, errorTriageEmailService)
	changelogStore := changelog.NewPostgresStore(pool)
	changelogService := changelog.NewService(
		changelogStore,
		changelog.NewSigner(os.Getenv("CHANGELOG_ACTION_SIGNING_SECRET")),
		&changelog.GitHubDispatcher{
			Token: os.Getenv("CHANGELOG_RELEASE_GITHUB_TOKEN"),
			Repo:  os.Getenv("CHANGELOG_GITHUB_REPO"),
		},
		changelog.ServiceConfig{
			DashboardBaseURL: os.Getenv("NEXT_PUBLIC_APP_URL"),
			GitHubRef:        os.Getenv("CHANGELOG_PUBLISH_REF"),
			GitHubWorkflow:   firstNonEmpty(os.Getenv("CHANGELOG_PUBLISH_WORKFLOW"), "changelog-publish.yml"),
			DryRun:           strings.EqualFold(os.Getenv("CHANGELOG_PUBLISH_DRY_RUN"), "true"),
		},
	)
	changelogAutomationHandler := handler.NewChangelogAutomationHandler(
		changelogStore,
		changelogService,
		os.Getenv("CHANGELOG_AUTOMATION_TOKEN"),
	)

	// Public routes
	r.Get("/health", healthHandler.Health)
	r.Get("/v1/plans", billingHandler.ListPlans)
	// Platform capabilities map — public, cacheable, no auth.
	// LLM clients use this BEFORE generating drafts to know each
	// platform's caption / media limits. See AGENTPOST_HANDOFF §7.2.
	r.Get("/v1/platforms/capabilities", platformHandler.GetGlobalCapabilities)

	// Webhook routes (no API key auth, verified by signatures)
	r.Post("/webhooks/clerk", webhookHandler.HandleClerk)
	r.Post("/webhooks/stripe", stripeWebhookHandler.HandleStripe)
	r.Get("/webhooks/meta", metaWebhookHandler.Verify)
	r.Post("/webhooks/meta", metaWebhookHandler.Handle)

	// WebSocket — auth via ?token= query param (browser WS API
	// doesn't support custom headers). Handler validates Clerk JWT.
	inboxWSHandler := ws.NewHandler(inboxHub, queries).WithInboxPlanGate(quotaChecker)
	logsWSHandler := ws.NewHandler(logsHub, queries)
	r.Get("/v1/inbox/ws", inboxWSHandler.ServeHTTP)
	r.Get("/v1/logs/ws", logsWSHandler.ServeHTTP)

	// OAuth callback routes (no auth — called by OAuth providers)
	r.Get("/v1/oauth/callback/{platform}", oauthHandler.Callback)

	// Public preview endpoint — no auth, JWT in query string. The
	// dashboard preview page hits this route to render a draft.
	r.Get("/v1/public/drafts/{id}", previewHandler.PublicGet)
	r.Post("/v1/public/landing-visit", landingAttributionHandler.RecordVisit)

	// Sprint 3 PR2: public Connect session lookup — no API key, the
	// hosted dashboard page reads it via ?state=<oauth_state> as the
	// bearer. Returns a minimal projection of the session.
	r.Get("/v1/public/connect/sessions/{id}", connectSessionHandler.PublicGet)

	// RBAC Phase 4: invite preview. The dashboard's /invite/{token}
	// page calls this BEFORE the user signs in to display "Acme Inc.
	// invited you to join as editor". The token in the URL is the only
	// authentication; an invalid / expired / revoked token returns 404
	// to avoid leaking which tokens existed.
	configureMembersEmail := func(h *handler.MembersHandler) *handler.MembersHandler {
		if auditedLoopsClient != nil {
			return h.SetInviteEmailSender(auditedLoopsClient, os.Getenv("LOOPS_WORKSPACE_MEMBER_INVITED_TRANSACTIONAL_ID"))
		}
		return h
	}
	publicMembersHandler := configureMembersEmail(handler.NewMembersHandler(queries, quotaChecker, mailer, os.Getenv("NEXT_PUBLIC_APP_URL")))
	r.Get("/v1/public/invites/{token}", publicMembersHandler.GetInvite)

	// Sprint 3 PR5: Bluesky Connect form submission. Native HTML form
	// POST from the hosted dashboard page (cross-origin, no JS). Server
	// renders an HTML response — success → 302 to return_url; failure
	// → re-rendered form with inline error.
	r.Post("/v1/public/connect/sessions/{id}/bluesky", connectBlueskyHandler.SubmitForm)

	// Sprint 3 PR3: OAuth Connect — authorize bridge + callback.
	// /authorize is called by the hosted page when the user clicks
	// the platform button. /callback is the OAuth provider's redirect
	// target. Both are unauthenticated; oauth_state + the URL session
	// id are the bearer.
	r.Get("/v1/public/connect/sessions/{id}/authorize", connectCallbackHandler.Authorize)
	r.Get("/v1/connect/callback/{platform}", connectCallbackHandler.Callback)

	// Sprint 4 PR7: Meta App Review data-deletion callback. No auth —
	// the signed_request JWT body is the bearer. Mandatory for the
	// Meta App Review submission process; ships in Sprint 4 even
	// though Meta business verification + the actual integration
	// land later.
	r.Post("/v1/meta/data-deletion", metaDataDeletionHandler.HandleDataDeletion)

	// CLI setup-token exchange. The setup token itself is the short-lived
	// bearer, so this endpoint stays outside the normal API-key auth group.
	r.Post("/v1/cli/setup-tokens/exchange", cliSetupTokenHandler.Exchange)

	// Internal changelog automation endpoints. These are intentionally
	// outside Clerk/API-key auth because GitHub Actions calls them with
	// a dedicated automation token. Human actions still go through the
	// super-admin routes below.
	r.Get("/internal/changelog-candidates", changelogAutomationHandler.ListInternalCandidates)
	r.Post("/internal/changelog-candidates", changelogAutomationHandler.CreateInternalCandidate)
	r.Get("/internal/changelog-candidates/{id}", changelogAutomationHandler.GetInternalCandidate)

	// User-identity routes (Clerk session only — these are about the
	// signed-in human, not a workspace, so no API-key counterpart).
	r.Group(func(r chi.Router) {
		r.Use(auth.ClerkSessionMiddleware)

		r.Get("/v1/me", meHandler.Get)
		r.Get("/v1/me/plan-gates", meHandler.PlanGates)
		r.Get("/v1/me/features", meHandler.FeatureFlagsCompat)
		r.Get("/v1/me/bootstrap", meHandler.Bootstrap)
		r.Post("/v1/me/landing-attribution", landingAttributionHandler.BindSessionToUser)
		r.Patch("/v1/me/onboarding", meHandler.CompleteOnboarding)
		r.Patch("/v1/me/intent", meHandler.SetIntent)
		r.Post("/v1/me/onboarding-shown", meHandler.MarkShown)
		r.Delete("/v1/me", meHandler.Delete)

		activationHandler := handler.NewActivationHandler(queries)
		r.Get("/v1/me/activation", activationHandler.Get)
		r.Post("/v1/me/activation/dismiss", activationHandler.Dismiss)

		notificationHandler := handler.NewNotificationHandler(queries, mailer, os.Getenv("APP_BASE_URL"))
		if auditedLoopsClient != nil {
			notificationHandler.SetNotificationTestEmailSender(auditedLoopsClient, os.Getenv("LOOPS_NOTIFICATION_TEST_TRANSACTIONAL_ID"))
		}
		r.Get("/v1/me/notifications/events", notificationHandler.ListEvents)
		r.Get("/v1/me/notifications/channels", notificationHandler.ListChannels)
		r.Post("/v1/me/notifications/channels", notificationHandler.CreateChannel)
		r.Delete("/v1/me/notifications/channels/{id}", notificationHandler.DeleteChannel)
		r.Post("/v1/me/notifications/channels/{id}/test", notificationHandler.TestChannel)
		r.Get("/v1/me/notifications/subscriptions", notificationHandler.ListSubscriptions)
		r.Put("/v1/me/notifications/subscriptions", notificationHandler.UpsertSubscription)
		r.Delete("/v1/me/notifications/subscriptions/{id}", notificationHandler.DeleteSubscription)
		r.Get("/v1/me/notifications/email-preferences", notificationHandler.ListEmailPreferences)
		r.Put("/v1/me/notifications/email-preferences/{category}", notificationHandler.UpdateEmailPreference)

		tutorialsHandler := handler.NewTutorialsHandler(queries)
		r.Get("/v1/me/tutorials", tutorialsHandler.List)
		r.Post("/v1/me/tutorials/{id}/complete", tutorialsHandler.Complete)
		r.Post("/v1/me/tutorials/{id}/dismiss", tutorialsHandler.Dismiss)
		r.Post("/v1/me/tutorials/{id}/reopen", tutorialsHandler.Reopen)

		// RBAC Phase 4: invite acceptance. Requires a Clerk session (the
		// user clicking the email link) but NOT a workspace context —
		// they may not be a member of any workspace yet. The handler
		// creates the membership and stamps the invite accepted.
		clerkOnlyMembersHandler := configureMembersEmail(handler.NewMembersHandler(queries, quotaChecker, mailer, os.Getenv("NEXT_PUBLIC_APP_URL")))
		r.Post("/v1/invites/{token}/accept", clerkOnlyMembersHandler.AcceptInvite)
	})

	// Admin routes — Clerk session + ADMIN_USERS gate. The middleware
	// stack runs Clerk first to populate userID in ctx, then the admin
	// check resolves the user against the ADMIN_USERS allowlist (which
	// accepts both Clerk user IDs and emails).
	r.Group(func(r chi.Router) {
		r.Use(auth.ClerkSessionMiddleware)
		r.Use(auth.AdminMiddleware(adminChecker))

		r.Get("/v1/admin/stats", adminHandler.GetStats)
		r.Get("/v1/admin/landing-sources", landingAttributionHandler.GetAdminSources)
		r.Get("/v1/admin/landing-visitors", landingAttributionHandler.GetAdminVisitors)
		r.Get("/v1/admin/posts", adminHandler.ListPosts)
		r.Get("/v1/admin/posts/aggregates", adminHandler.ListPostsAggregates)
		r.Get("/v1/admin/email-notifications", adminHandler.ListEmailNotifications)
		r.Get("/v1/admin/billing", adminHandler.ListBilling)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Admin logs are restricted to super admins")).
			Get("/v1/admin/logs", adminHandler.ListLogs)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Admin logs are restricted to super admins")).
			Get("/v1/admin/logs/{id}", adminHandler.GetLog)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Support bundles are restricted to super admins")).
			Get("/v1/admin/support-bundles", supportBundleHandler.ListAdmin)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Support bundles are restricted to super admins")).
			Get("/v1/admin/support-bundles/{id}", supportBundleHandler.GetAdmin)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "AI provider keys are restricted to super admins")).
			Get("/v1/admin/ai-providers", aiProviderHandler.List)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "AI provider keys are restricted to super admins")).
			Put("/v1/admin/ai-providers/{provider}", aiProviderHandler.Update)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "AI provider keys are restricted to super admins")).
			Post("/v1/admin/ai-providers/{provider}/test", aiProviderHandler.Test)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "AI provider keys are restricted to super admins")).
			Post("/v1/admin/ai-providers/{provider}/disable", aiProviderHandler.Disable)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "AI provider keys are restricted to super admins")).
			Put("/v1/admin/ai-provider-routing/{surface}", aiProviderHandler.Route)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "AI provider keys are restricted to super admins")).
			Delete("/v1/admin/ai-provider-routing/{surface}", aiProviderHandler.Unroute)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "AI provider keys are restricted to super admins")).
			Get("/v1/admin/ai-providers/events", aiProviderHandler.Events)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Changelog release actions are restricted to super admins")).
			Get("/v1/admin/changelog-candidates/{id}", changelogAutomationHandler.GetAdminCandidate)
		r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Changelog release actions are restricted to super admins")).
			Post("/v1/admin/changelog-candidates/{id}/actions", changelogAutomationHandler.ConfirmAdminAction)
		// Dev / QA: flip a workspace's plan_id directly without going
		// through Stripe Checkout. Useful for testing the plan-feature
		// gates end-to-end. Already protected by the admin middleware
		// guarding this group.
		r.Post("/v1/admin/workspaces/{workspaceID}/plan", adminHandler.SetPlan)
		r.Get("/v1/admin/post-failures", adminHandler.ListPostFailures)
		r.Get("/v1/admin/error-triage/runs", errorTriageHandler.ListRuns)
		r.Post("/v1/admin/error-triage/runs", errorTriageHandler.CreateRun)
		r.Get("/v1/admin/error-triage/runs/{id}", errorTriageHandler.GetRun)
		r.Post("/v1/admin/error-triage/runs/{id}/rerun", errorTriageHandler.Rerun)
		r.Patch("/v1/admin/error-triage/items/{id}", errorTriageHandler.UpdateItem)
		r.Post("/v1/admin/error-triage/items/{id}/approve-bug-plan", errorTriageHandler.ApproveBugPlan)
		r.Post("/v1/admin/error-triage/items/{id}/recipients/{recipientID}/send-email", errorTriageHandler.SendEmail)
		r.Post("/v1/admin/error-triage/items/{id}/recipients/{recipientID}/dismiss", errorTriageHandler.DismissRecipient)
		r.Get("/v1/admin/users", adminHandler.ListUsers)
		r.Get("/v1/admin/users/signups", adminHandler.GetUserSignups)
		r.Get("/v1/admin/users/{id}", adminHandler.GetUser)
		r.Get("/v1/admin/users/{id}/scheduled-posts", adminHandler.ListUserScheduledPosts)
		r.Get("/v1/admin/users/{id}/post-failures", adminHandler.ListUserPostFailures)
		r.Get("/v1/admin/api-metrics/overall", adminAPIMetricsHandler.Overall)
		r.Get("/v1/admin/api-metrics/summary", adminAPIMetricsHandler.Summary)
		r.Get("/v1/admin/api-metrics/trend", adminAPIMetricsHandler.Trend)
		r.Get("/v1/admin/api-metrics/status-codes", adminAPIMetricsHandler.StatusCodes)
		r.Get("/v1/admin/api-metrics/workspaces", adminAPIMetricsHandler.Workspaces)
		r.Get("/v1/admin/search-history", adminSearchHistoryHandler.List)
		r.Post("/v1/admin/search-history", adminSearchHistoryHandler.Save)
		r.Delete("/v1/admin/search-history/{id}", adminSearchHistoryHandler.Delete)
	})

	// All workspace-scoped routes — accept either a Bearer API key or
	// a Clerk session JWT. DualAuthMiddleware resolves the token and
	// stamps workspaceID into the request context (and apiKeyID for
	// API-key paths). Handlers always read workspaceID from context;
	// it is never carried in the URL anymore.
	r.Group(func(r chi.Router) {
		r.Use(auth.DualAuthMiddleware(queries))
		r.Use(integrationlogs.Middleware(integrationLogger))
		r.Use(apiMetricsRecorder.Middleware)

		// Workspace info.
		r.Get("/v1/workspace", workspaceHandler.Get)
		// Workspace settings (per_account_monthly_limit, name) — admin+
		// only. Owner-only would also be defensible; using admin to
		// match the pattern of "operational settings = admin".
		r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/v1/workspace", workspaceHandler.Update)

		// API keys.
		// API keys. List is workspace-wide; create/revoke is admin+ to
		// keep rogue editors from minting bearer tokens. Future invite
		// flow may relax this to "editors create their own keys" once
		// keys carry created_by_user_id (RBAC Phase 4+).
		r.Get("/v1/api-keys", apiKeyHandler.List)
		r.With(auth.RequireRole(auth.RoleAdmin)).Post("/v1/api-keys", apiKeyHandler.Create)
		r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/v1/api-keys/{keyID}", apiKeyHandler.Revoke)
		r.With(auth.RequireRole(auth.RoleAdmin)).Post("/v1/cli/setup-tokens", cliSetupTokenHandler.Issue)

		// Profiles.
		r.Get("/v1/profiles", profileHandler.APIList)
		r.Post("/v1/profiles", profileHandler.APICreate)
		r.Get("/v1/profiles/{id}", profileHandler.APIGet)
		r.Patch("/v1/profiles/{id}", profileHandler.APIUpdate)
		r.Post("/v1/profiles/{id}/branding/logo", profileHandler.UploadBrandingLogo)
		r.Delete("/v1/profiles/{id}/branding/logo", profileHandler.DeleteBrandingLogo)
		r.Delete("/v1/profiles/{id}", profileHandler.APIDelete)

		// Accounts (workspace-wide).
		r.Get("/v1/accounts", socialAccountHandler.List)
		r.Post("/v1/accounts/connect", socialAccountHandler.Connect)
		r.Delete("/v1/accounts/{id}", socialAccountHandler.Disconnect)
		r.Post("/v1/accounts/{id}/dismiss", socialAccountHandler.Dismiss)
		r.Get("/v1/accounts/{id}/capabilities", platformHandler.GetAccountCapabilities)
		r.Get("/v1/accounts/{id}/health", socialAccountHandler.AccountHealth)
		r.Get("/v1/accounts/{id}/metrics", socialAccountHandler.AccountMetrics)
		r.Get("/v1/accounts/{id}/instagram/profile", socialAccountHandler.InstagramProfile)
		r.Get("/v1/accounts/{id}/instagram/media", socialAccountHandler.InstagramMedia)
		r.Get("/v1/accounts/{id}/threads/profile", socialAccountHandler.ThreadsProfile)
		r.Get("/v1/accounts/{id}/threads/posts", socialAccountHandler.ThreadsPosts)
		r.Get("/v1/accounts/{id}/tiktok/creator-info", socialAccountHandler.TikTokCreatorInfo)
		r.Get("/v1/accounts/{id}/tiktok/profile", socialAccountHandler.TikTokProfile)
		r.Get("/v1/accounts/{id}/tiktok/videos", socialAccountHandler.TikTokVideos)
		r.Get("/v1/accounts/{id}/youtube/analytics/summary", socialAccountHandler.YouTubeAnalyticsSummary)
		r.Get("/v1/accounts/{id}/youtube/analytics/trend", socialAccountHandler.YouTubeAnalyticsTrend)
		r.Get("/v1/accounts/{id}/youtube/analytics/videos", socialAccountHandler.YouTubeAnalyticsVideos)
		r.With(handler.RequirePlanAnalytics(quotaChecker), auth.AdminMiddleware(adminChecker)).
			Get("/v1/accounts/{id}/facebook/page-analytics", socialAccountHandler.FacebookPageAnalytics)
		r.Get("/v1/accounts/{id}/pinterest/boards", socialAccountHandler.PinterestBoards)
		r.Post("/v1/accounts/{id}/pinterest/boards", socialAccountHandler.CreatePinterestBoard)
		r.With(auth.RequireFacebookSuperAdmin(superAdminChecker)).
			Get("/v1/accounts/{id}/facebook/page-insights", socialAccountHandler.FacebookPageInsights)
		// FB webhook subscription diagnose / repair. Read endpoint
		// shows whether the Page's webhook subscription with our App
		// is healthy; the POST re-runs SubscribePageToWebhooks for
		// Pages that fell off (silent failures during connect-finalize).
		r.Get("/v1/accounts/{id}/facebook/webhook-status", socialAccountHandler.FacebookWebhookStatus)
		r.Post("/v1/accounts/{id}/facebook/resubscribe-webhooks", socialAccountHandler.FacebookResubscribeWebhooks)

		// Profile-nested account / user views (used by the dashboard's
		// profile switcher to scope by current profile).
		r.Get("/v1/profiles/{profileID}/accounts", socialAccountHandler.List)
		r.Post("/v1/profiles/{profileID}/accounts/connect", socialAccountHandler.Connect)
		r.Delete("/v1/profiles/{profileID}/accounts/{accountID}", socialAccountHandler.Disconnect)
		r.Post("/v1/profiles/{profileID}/accounts/{accountID}/dismiss", socialAccountHandler.Dismiss)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/metrics", socialAccountHandler.AccountMetrics)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/instagram/profile", socialAccountHandler.InstagramProfile)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/instagram/media", socialAccountHandler.InstagramMedia)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/threads/profile", socialAccountHandler.ThreadsProfile)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/threads/posts", socialAccountHandler.ThreadsPosts)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/tiktok/creator-info", socialAccountHandler.TikTokCreatorInfo)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/tiktok/profile", socialAccountHandler.TikTokProfile)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/tiktok/videos", socialAccountHandler.TikTokVideos)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/youtube/analytics/summary", socialAccountHandler.YouTubeAnalyticsSummary)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/youtube/analytics/trend", socialAccountHandler.YouTubeAnalyticsTrend)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/youtube/analytics/videos", socialAccountHandler.YouTubeAnalyticsVideos)
		r.With(handler.RequirePlanAnalytics(quotaChecker), auth.AdminMiddleware(adminChecker)).
			Get("/v1/profiles/{profileID}/accounts/{accountID}/facebook/page-analytics", socialAccountHandler.FacebookPageAnalytics)
		r.Get("/v1/profiles/{profileID}/accounts/{accountID}/pinterest/boards", socialAccountHandler.PinterestBoards)
		r.Post("/v1/profiles/{profileID}/accounts/{accountID}/pinterest/boards", socialAccountHandler.CreatePinterestBoard)
		r.With(auth.RequireFacebookSuperAdmin(superAdminChecker)).
			Get("/v1/profiles/{profileID}/accounts/{accountID}/facebook/page-insights", socialAccountHandler.FacebookPageInsights)
		r.Get("/v1/profiles/{profileID}/users", managedUsersHandler.List)
		r.Get("/v1/profiles/{profileID}/users/{external_user_id}", managedUsersHandler.Get)
		r.Post("/v1/profiles/{profileID}/users/{external_user_id}/dismiss", managedUsersHandler.DismissDisconnected)
		r.Get("/v1/profiles/{profileID}/oauth/connect/{platform}", oauthHandler.Connect)

		// Media — two-step upload (POST returns presigned URL, client
		// PUTs to R2 directly), then reference the media_id in
		// platform_posts[].media_ids on subsequent /v1/posts.
		r.Post("/v1/media/audio-overlays", mediaAudioOverlayHandler.Create)
		r.Get("/v1/media/audio-overlays/{id}", mediaAudioOverlayHandler.Get)
		r.Post("/v1/media", mediaHandler.Create)
		r.Get("/v1/media/{id}", mediaHandler.Get)
		r.Delete("/v1/media/{id}", mediaHandler.Delete)

		// AI compose assist — currently super-admin-only while the
		// drawer workflow is in development. Ships as a workspace-scoped
		// route so it can later reuse existing publish context
		// (accounts, profiles, media, validation) without a second auth
		// surface.
		r.Post("/v1/ai/post-assist", aiPostAssistHandler.PostAssist)

		// Connect sessions.
		r.Post("/v1/connect/sessions", connectSessionHandler.Create)
		r.Get("/v1/connect/sessions/{id}", connectSessionHandler.Get)

		// Platform credentials. List is open to any role; mutations
		// require admin+. Plan-specific custom platform limits are
		// enforced in the handler so Basic can use its one shared slot.
		r.Get("/v1/platform-credentials", platformCredHandler.List)
		r.With(auth.RequireRole(auth.RoleAdmin)).
			Post("/v1/platform-credentials", platformCredHandler.Create)
		r.With(auth.RequireRole(auth.RoleAdmin)).
			Delete("/v1/platform-credentials/{platform}", platformCredHandler.Delete)

		// Posts.
		r.Get("/v1/posts", socialPostHandler.List)
		r.Get("/v1/posts/summaries", socialPostHandler.ListSummaries)
		r.Post("/v1/posts", socialPostHandler.Create)
		r.Post("/v1/posts/bulk", socialPostHandler.CreateBulk)
		r.Post("/v1/posts/validate", socialPostHandler.Validate)
		r.Get("/v1/posts/{id}", socialPostHandler.Get)
		r.Patch("/v1/posts/{id}", socialPostHandler.UpdateDraft)
		r.Delete("/v1/posts/{id}", socialPostHandler.Delete)
		r.Post("/v1/posts/{id}/archive", socialPostHandler.Archive)
		r.Post("/v1/posts/{id}/restore", socialPostHandler.Restore)
		r.Post("/v1/posts/{id}/cancel", socialPostHandler.CancelPost)
		r.Post("/v1/posts/{id}/publish", socialPostHandler.PublishDraft)
		r.Post("/v1/posts/{id}/preview-link", previewHandler.CreateLink)
		r.Get("/v1/posts/{id}/queue", socialPostHandler.GetPostQueue)
		r.With(handler.RequirePlanAnalytics(quotaChecker)).Get("/v1/posts/{id}/analytics", analyticsHandler.GetAnalytics)
		r.Post("/v1/posts/{id}/results/{resultID}/retry", socialPostHandler.RetryResult)

		// Post delivery jobs.
		r.Get("/v1/post-delivery-jobs", socialPostHandler.ListDeliveryJobs)
		r.Get("/v1/post-delivery-jobs/summary", socialPostHandler.GetDeliveryJobsSummary)
		r.Post("/v1/post-delivery-jobs/{jobID}/retry", socialPostHandler.RetryDeliveryJob)
		r.Post("/v1/post-delivery-jobs/{jobID}/retry-now", socialPostHandler.RetryDeliveryJobNow)
		r.Post("/v1/post-delivery-jobs/{jobID}/cancel", socialPostHandler.CancelDeliveryJobHandler)
		r.Post("/v1/post-delivery-jobs/{jobID}/dismiss", socialPostHandler.DismissDeliveryJobHandler)

		// Managed users.
		r.Get("/v1/users", managedUsersHandler.List)
		r.Get("/v1/users/{external_user_id}", managedUsersHandler.Get)
		r.Post("/v1/users/{external_user_id}/dismiss", managedUsersHandler.DismissDisconnected)

		// Webhooks. Read OK for any role; mutations require admin+.
		r.Get("/v1/webhooks", webhookSubHandler.List)
		r.Get("/v1/webhooks/{id}", webhookSubHandler.Get)
		r.With(auth.RequireRole(auth.RoleAdmin)).Post("/v1/webhooks", webhookSubHandler.Create)
		r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/v1/webhooks/{id}", webhookSubHandler.Update)
		r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/v1/webhooks/{id}", webhookSubHandler.Delete)
		r.With(auth.RequireRole(auth.RoleAdmin)).Post("/v1/webhooks/{id}/rotate", webhookSubHandler.Rotate)

		// Analytics. Plan-gated (migration 059): Free returns 402.
		r.Group(func(r chi.Router) {
			r.Use(handler.RequirePlanAnalytics(quotaChecker))
			r.Get("/v1/analytics/posts", analyticsExplorerHandler.ListPosts)
			r.Get("/v1/analytics/posts/export", analyticsExplorerHandler.ExportPostsCSV)
			r.Get("/v1/analytics/platforms", analyticsExplorerHandler.ListPlatforms)
			r.Get("/v1/analytics/platforms/{platform}", analyticsExplorerHandler.GetPlatform)
			r.Post("/v1/analytics/refresh", analyticsExplorerHandler.RequestRefresh)
			r.Get("/v1/analytics/summary", analyticsHandler.GetSummary)
			r.Get("/v1/analytics/trend", analyticsHandler.GetTrend)
			r.Get("/v1/analytics/by-platform", analyticsHandler.GetByPlatform)
			r.Get("/v1/analytics/rollup", analyticsRollupHandler.GetRollup)
		})

		// API metrics.
		r.Get("/v1/api-metrics/summary", apiMetricsHandler.Summary)
		r.Get("/v1/api-metrics/trend", apiMetricsHandler.Trend)
		r.Get("/v1/api-metrics/overall", apiMetricsHandler.Overall)
		r.Get("/v1/api-metrics/status-codes", apiMetricsHandler.StatusCodes)

		// Billing. Read is workspace-wide (any role); checkout / portal
		// are owner-only because they touch the payment method.
		r.Get("/v1/billing", billingHandler.GetBilling)
		r.With(auth.RequireRole(auth.RoleOwner)).Post("/v1/billing/checkout", billingHandler.CreateCheckout)
		r.With(auth.RequireRole(auth.RoleOwner)).Post("/v1/billing/portal", billingHandler.CreatePortal)
		r.Get("/v1/usage", billingHandler.GetUsage)

		// Rate-limit / queue-admission visibility — public-facing
		// read of the workspace's runtime safety caps + a live
		// snapshot of current queue depth. Drives the dashboard's
		// API Limits settings page.
		apiLimitsHandler := handler.NewApiLimitsHandler(queries, quotaChecker)
		r.Get("/v1/limits", apiLimitsHandler.Get)

		// OAuth (workspace-scoped, non-profile entry).
		r.Post("/v1/oauth/connect", oauthHandler.Connect)

		// Facebook Page picker — pending-connection read + finalize.
		// Still gated by ENABLE_FACEBOOK_PAGES super-admin while the
		// integration is in audit.
		r.Route("/v1/pending-connections", func(r chi.Router) {
			r.Use(auth.RequireFacebookSuperAdmin(superAdminChecker))
			r.Get("/{id}", oauthHandler.PendingConnectionGet)
			r.Post("/{id}/finalize", oauthHandler.PendingConnectionFinalize)
		})

		// Members & invites (RBAC Phase 4-5). Read access is open to
		// any role; mutations are admin+ except transfer-ownership
		// which is owner only. The accept-invite endpoint is mounted
		// outside this group below — it needs Clerk auth but no role
		// (the user accepting may not yet be a member of any
		// workspace).
		membersHandler := configureMembersEmail(handler.NewMembersHandler(queries, quotaChecker, mailer, os.Getenv("NEXT_PUBLIC_APP_URL")))
		r.Get("/v1/members", membersHandler.List)
		r.With(auth.RequireRole(auth.RoleAdmin)).Post("/v1/members/invite", membersHandler.Invite)
		r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/v1/members/invites/{id}", membersHandler.RevokeInvite)
		r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/v1/members/{userID}/role", membersHandler.ChangeRole)
		r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/v1/members/{userID}", membersHandler.Remove)
		r.With(auth.RequireRole(auth.RoleOwner)).Post("/v1/members/{userID}/transfer-ownership", membersHandler.TransferOwnership)

		// Audit log (RBAC Phase 6). Read access only — writes happen
		// inline at every mutation site via internal/audit.Log().
		auditHandler := handler.NewAuditHandler(queries)
		r.Get("/v1/audit-log", auditHandler.List)
		logsHandler := handler.NewLogsHandler(queries)
		r.Get("/v1/logs", logsHandler.List)
		// Mount the static /stream route before the /{id} param route so
		// chi does not treat "stream" as a log id.
		logsStreamHandler := handler.NewLogsStreamHandler(logsHub, queries)
		r.Get("/v1/logs/stream", logsStreamHandler.Stream)
		r.Get("/v1/logs/{id}", logsHandler.Get)
		r.Post("/v1/support-bundles", supportBundleHandler.Create)

		// Inbox — unified Instagram comments/DMs and Threads replies.
		// Plan-gated (migration 059): Free + API plans get 402.
		inboxHandler := handler.NewInboxHandler(queries, encryptor, pool)
		r.Route("/v1/inbox", func(r chi.Router) {
			r.Use(handler.RequirePlanInbox(quotaChecker))
			r.Get("/", inboxHandler.List)
			r.Get("/unread-count", inboxHandler.UnreadCount)
			r.Post("/mark-all-read", inboxHandler.MarkAllRead)
			r.Post("/sync", inboxHandler.Sync)
			r.Get("/{id}", inboxHandler.Get)
			r.Get("/{id}/media-context", inboxHandler.MediaContext)
			r.Post("/{id}/read", inboxHandler.MarkRead)
			r.Post("/{id}/reply", inboxHandler.Reply)
			r.Post("/{id}/thread-state", inboxHandler.UpdateThreadState)
		})
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func corsAllowedOrigins() []string {
	origins := []string{
		"https://app.unipost.dev",
		"https://dev-app.unipost.dev",
		"https://dev.unipost.dev",
		"https://unipost.dev",
		"http://localhost:3000",
	}

	extraOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	for _, origin := range extraOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	return origins
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

// syncStripePriceIDs writes the LIVE Stripe price IDs from env vars into the
// plans.stripe_price_id column on startup. The column is now a legacy cache —
// the actual price ID used at checkout time is resolved per-mode by
// internal/billing.Manager (which holds separate live + sandbox maps in
// memory). The DB sync stays for now in case other tooling reads the column
// for inspection; it does NOT touch the sandbox env vars by design.
//
// Env var naming mirrors internal/billing.Manager: STRIPE_PRICE_ID_<amount>
// where the amount is the monthly dollar price.
func syncStripePriceIDs(ctx context.Context, queries *db.Queries) {
	// Migration 058 (May 2026) replaced the per-volume tier IDs
	// (p10..p1000) with product tiers (api/basic/growth/team). The
	// env-var token scheme follows the new names. 'enterprise' is
	// out-of-band (no Stripe Checkout) and intentionally not synced.
	planEnvMap := map[string]string{
		"api":    "STRIPE_PRICE_ID_API",
		"basic":  "STRIPE_PRICE_ID_BASIC",
		"growth": "STRIPE_PRICE_ID_GROWTH",
		"team":   "STRIPE_PRICE_ID_TEAM",
	}

	for planID, envVar := range planEnvMap {
		priceID := os.Getenv(envVar)
		if priceID != "" {
			queries.UpdatePlanStripePriceID(ctx, db.UpdatePlanStripePriceIDParams{
				ID:            planID,
				StripePriceID: pgtype.Text{String: priceID, Valid: true},
			})
			slog.Info("synced stripe price", "plan", planID, "env", envVar)
		}
	}
}

// fanoutHandler sends log records to multiple slog.Handlers.
type fanoutHandler struct {
	handlers []slog.Handler
}

func (h fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (h fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return fanoutHandler{handlers: handlers}
}

func (h fanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return fanoutHandler{handlers: handlers}
}
