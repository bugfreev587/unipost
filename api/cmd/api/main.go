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

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
	mw "github.com/xiaoboyu/unipost-api/internal/middleware"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/storage"
	"github.com/xiaoboyu/unipost-api/internal/worker"
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

	// Conditionally register adapters that need credentials
	if os.Getenv("TIKTOK_CLIENT_KEY") != "" {
		tiktokAdapter := platform.NewTikTokAdapter()
		tiktokAdapter.SetMediaProxy(storageClient)
		platform.Register(tiktokAdapter)
		slog.Info("tiktok adapter registered", "media_proxy", storageClient != nil)
	}
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

	// Start background workers
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

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
	// Sprint 5 PR3: Instagram Connect, gated behind a feature flag.
	// Two doors must both be open before Instagram becomes a real
	// Connect platform: (a) the credentials must be present, and (b)
	// CONNECT_INSTAGRAM_ENABLED must be truthy. The flag exists so we
	// can ship this code to production well before launch and only
	// flip it on when the Meta App Review is approved — this avoids
	// a "what's that broken Instagram tile in the Connect picker?"
	// support thread on day 1. Keep the legacy fail-fast nil check
	// in NewInstagramConnector for the credentials half.
	if instagramConnectEnabled() {
		if ig := connect.NewInstagramConnector(os.Getenv("INSTAGRAM_APP_ID"), os.Getenv("INSTAGRAM_APP_SECRET"), apiBaseURL); ig != nil {
			connectors = append(connectors, ig)
		}
	}
	// Sprint 5 PR4: Threads Connect, gated behind a feature flag for
	// the same reasons as Instagram (Sprint 5 PR3) — Meta App Review
	// approval is decoupled from code shipping. Same THREADS_APP_ID /
	// THREADS_APP_SECRET env vars the BYO/dashboard path already
	// reads, so a single set of credentials covers both connection
	// types.
	if threadsConnectEnabled() {
		if th := connect.NewThreadsConnector(os.Getenv("THREADS_APP_ID"), os.Getenv("THREADS_APP_SECRET"), apiBaseURL); th != nil {
			connectors = append(connectors, th)
		}
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
	webhookWorker := worker.NewWebhookDeliveryWorker(queries)
	go webhookWorker.Start(workerCtx)

	// Sprint 3 PR7: managed token refresh worker. Started here so
	// the bus dependency (webhookWorker) is already wired.
	managedTokenWorker := worker.NewManagedTokenRefreshWorker(queries, encryptor, connectRegistry, webhookWorker)
	go managedTokenWorker.Start(workerCtx)

	schedulerWorker := worker.NewSchedulerWorker(queries, encryptor, webhookWorker)
	go schedulerWorker.Start(workerCtx)

	analyticsRefreshWorker := worker.NewAnalyticsRefreshWorker(queries, encryptor, storageClient)
	go analyticsRefreshWorker.Start(workerCtx)

	r := chi.NewRouter()

	// Global middleware
	r.Use(mw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://app.unipost.dev", "https://unipost.dev", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link", "X-UniPost-Usage", "X-UniPost-Warning"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Handlers
	healthHandler := handler.NewHealthHandler()
	webhookHandler := handler.NewWebhookHandler(queries)
	profileHandler := handler.NewProfileHandler(queries)
	workspaceHandler := handler.NewWorkspaceHandler(queries)
	apiKeyHandler := handler.NewAPIKeyHandler(queries)
	socialAccountHandler := handler.NewSocialAccountHandler(queries, encryptor, webhookWorker)
	socialPostHandler := handler.NewSocialPostHandler(queries, encryptor, quotaChecker, webhookWorker, storageClient)
	webhookSubHandler := handler.NewWebhookSubscriptionHandler(queries)
	oauthHandler := handler.NewOAuthHandler(queries, encryptor)
	platformCredHandler := handler.NewPlatformCredentialHandler(queries, encryptor)
	billingHandler := handler.NewBillingHandler(queries, quotaChecker, stripeMgr)
	stripeWebhookHandler := handler.NewStripeWebhookHandler(queries, stripeMgr)
	analyticsHandler := handler.NewAnalyticsHandler(queries, encryptor)
	// Sprint 5 PR1: GET /v1/analytics/rollup uses raw pgx for the
	// dynamic GROUP BY clause sqlc can't model.
	analyticsRollupHandler := handler.NewAnalyticsRollupHandler(pool)
	platformHandler := handler.NewPlatformHandler(queries)
	mediaHandler := handler.NewMediaHandler(queries, storageClient)
	adminChecker := auth.NewAdminChecker(queries)
	meHandler := handler.NewMeHandler(queries, adminChecker)
	// Sprint 3 PR2: Connect sessions handler. Reuses NEXT_PUBLIC_APP_URL
	// for the hosted-page origin so the same env var that drives the
	// preview link drives the connect link.
	connectSessionHandler := handler.NewConnectSessionHandler(queries, os.Getenv("NEXT_PUBLIC_APP_URL"))
	// Sprint 3 PR5: Bluesky Connect form handler. No API key — the
	// session id + oauth_state act as the bearer. Server-renders an
	// HTML form so the app password never touches dashboard JS.
	connectBlueskyHandler := handler.NewConnectBlueskyHandler(queries, encryptor, webhookWorker)
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
	// connectRegistry was built in the worker section above so the
	// managed token refresh worker could take it as a dependency.
	// We just hand the same registry to the callback handler here.
	connectCallbackHandler := handler.NewConnectCallbackHandler(queries, encryptor, webhookWorker, connectRegistry)
	// Preview handler shares the dashboard origin (B3) and reuses
	// the ENCRYPTION_KEY value as the HMAC secret with an audience
	// claim for domain separation (B2). No new env var.
	previewHandler := handler.NewPreviewHandler(queries, storageClient, []byte(encryptionKey), os.Getenv("NEXT_PUBLIC_APP_URL"))
	adminHandler := handler.NewAdminHandler(pool, stripeMgr)

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

	// OAuth callback routes (no auth — called by OAuth providers)
	r.Get("/v1/oauth/callback/{platform}", oauthHandler.Callback)

	// Public preview endpoint — no auth, JWT in query string. The
	// dashboard preview page hits this route to render a draft.
	r.Get("/v1/public/drafts/{id}", previewHandler.PublicGet)

	// Sprint 3 PR2: public Connect session lookup — no API key, the
	// hosted dashboard page reads it via ?state=<oauth_state> as the
	// bearer. Returns a minimal projection of the session.
	r.Get("/v1/public/connect/sessions/{id}", connectSessionHandler.PublicGet)

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

	// Dashboard routes (Clerk session auth)
	r.Group(func(r chi.Router) {
		r.Use(auth.ClerkSessionMiddleware)

		// Whoami — returns the authenticated user's identity plus an
		// is_admin flag derived from ADMIN_USERS. The dashboard reads
		// this on mount to decide whether to render the Admin link.
		r.Get("/v1/me", meHandler.Get)
		// Dashboard root resolver: returns default_profile_id +
		// last_profile_id, lazily creating a "Default" profile for
		// fresh signups and backfilling default_profile_id for legacy
		// users with existing profiles but no stamped default.
		r.Get("/v1/me/bootstrap", meHandler.Bootstrap)

		r.Get("/v1/profiles", profileHandler.List)
		r.Post("/v1/profiles", profileHandler.Create)
		r.Get("/v1/profiles/{id}", profileHandler.Get)
		r.Patch("/v1/profiles/{id}", profileHandler.Update)
		r.Delete("/v1/profiles/{id}", profileHandler.Delete)

		// Workspace-scoped dashboard routes
		r.Get("/v1/workspaces/{workspaceID}/api-keys", apiKeyHandler.List)
		r.Post("/v1/workspaces/{workspaceID}/api-keys", apiKeyHandler.Create)
		r.Delete("/v1/workspaces/{workspaceID}/api-keys/{keyID}", apiKeyHandler.Revoke)

		// Social accounts (dashboard, profile-scoped)
		r.Get("/v1/profiles/{profileID}/social-accounts", socialAccountHandler.List)
		r.Post("/v1/profiles/{profileID}/social-accounts/connect", socialAccountHandler.Connect)
		r.Delete("/v1/profiles/{profileID}/social-accounts/{accountID}", socialAccountHandler.Disconnect)

		// Managed Users view (dashboard, profile-scoped).
		r.Get("/v1/profiles/{profileID}/users", managedUsersHandler.List)
		r.Get("/v1/profiles/{profileID}/users/{external_user_id}", managedUsersHandler.Get)

		// Social posts (dashboard, workspace-scoped)
		r.Get("/v1/workspaces/{workspaceID}/social-posts", socialPostHandler.List)
		r.Post("/v1/workspaces/{workspaceID}/social-posts", socialPostHandler.Create)

		// OAuth connect (dashboard, profile-scoped)
		r.Get("/v1/profiles/{profileID}/oauth/connect/{platform}", oauthHandler.Connect)

		// Platform credentials (White Label, workspace-scoped)
		r.Post("/v1/workspaces/{workspaceID}/platform-credentials", platformCredHandler.Create)
		r.Get("/v1/workspaces/{workspaceID}/platform-credentials", platformCredHandler.List)
		r.Delete("/v1/workspaces/{workspaceID}/platform-credentials/{platform}", platformCredHandler.Delete)

		// Billing (dashboard, workspace-scoped)
		r.Get("/v1/workspaces/{workspaceID}/billing", billingHandler.GetBilling)
		r.Post("/v1/workspaces/{workspaceID}/billing/checkout", billingHandler.CreateCheckout)
		r.Post("/v1/workspaces/{workspaceID}/billing/portal", billingHandler.CreatePortal)

		// Analytics (dashboard, workspace-scoped)
		r.Get("/v1/workspaces/{workspaceID}/analytics/summary", analyticsHandler.GetSummary)
		r.Get("/v1/workspaces/{workspaceID}/analytics/trend", analyticsHandler.GetTrend)
		r.Get("/v1/workspaces/{workspaceID}/analytics/by-platform", analyticsHandler.GetByPlatform)
		// Per-post analytics (workspace-scoped). Mirrors the API-key route below.
		r.Get("/v1/workspaces/{workspaceID}/social-posts/{id}/analytics", analyticsHandler.GetAnalytics)
	})

	// Admin routes — Clerk session + ADMIN_USERS gate. The middleware
	// stack runs Clerk first to populate userID in ctx, then the admin
	// check resolves the user against the ADMIN_USERS allowlist (which
	// accepts both Clerk user IDs and emails).
	r.Group(func(r chi.Router) {
		r.Use(auth.ClerkSessionMiddleware)
		r.Use(auth.AdminMiddleware(adminChecker))

		r.Get("/v1/admin/stats", adminHandler.GetStats)
		r.Get("/v1/admin/users", adminHandler.ListUsers)
		r.Get("/v1/admin/users/{id}", adminHandler.GetUser)
	})

	// Public API routes (API key auth)
	r.Group(func(r chi.Router) {
		r.Use(auth.APIKeyMiddleware(queries))

		// Workspace info (API key scoped)
		r.Get("/v1/workspace", workspaceHandler.Get)
		r.Patch("/v1/workspace", workspaceHandler.Update)

		r.Get("/v1/social-accounts", socialAccountHandler.List)
		r.Post("/v1/social-accounts/connect", socialAccountHandler.Connect)
		r.Delete("/v1/social-accounts/{id}", socialAccountHandler.Disconnect)
		// Per-account capability lookup. Returns the platform's
		// capability scoped to one account; falls back to platform
		// defaults until Sprint 2 adds account-specific overrides.
		r.Get("/v1/social-accounts/{id}/capabilities", platformHandler.GetAccountCapabilities)
		// Account health (Sprint 2 PR7) — derived from the last 10
		// social_post_results rows. ok / degraded / disconnected.
		r.Get("/v1/social-accounts/{id}/health", socialAccountHandler.AccountHealth)

		// Media library — two-step upload (POST returns presigned URL,
		// client PUTs to R2 directly), then reference the media_id in
		// platform_posts[].media_ids on subsequent /v1/social-posts.
		r.Post("/v1/media", mediaHandler.Create)
		r.Get("/v1/media/{id}", mediaHandler.Get)
		r.Delete("/v1/media/{id}", mediaHandler.Delete)

		// Sprint 3 PR2: Connect sessions. Customer creates a session,
		// emails the URL to their end user, and polls (or waits for
		// the account.connected webhook) until completion.
		r.Post("/v1/connect/sessions", connectSessionHandler.Create)
		r.Get("/v1/connect/sessions/{id}", connectSessionHandler.Get)

		r.Get("/v1/social-posts", socialPostHandler.List)
		r.Post("/v1/social-posts", socialPostHandler.Create)
		// Pure preflight — runs the same checks Create() will run, but
		// without writing rows or hitting platform APIs. LLM clients
		// call this BEFORE publish to self-correct draft errors.
		r.Post("/v1/social-posts/validate", socialPostHandler.Validate)
		r.Get("/v1/social-posts/{id}", socialPostHandler.Get)
		r.Get("/v1/social-posts/{id}/analytics", analyticsHandler.GetAnalytics)
		r.Delete("/v1/social-posts/{id}", socialPostHandler.Delete)
		// Drafts API (Sprint 2). Drafts are social_posts rows in
		// status='draft' — no platform dispatch, no quota charge,
		// no webhook fired. Publish flips them via optimistic lock
		// then routes through the same publish loop the immediate
		// path uses.
		r.Post("/v1/social-posts/{id}/publish", socialPostHandler.PublishDraft)
		r.Patch("/v1/social-posts/{id}", socialPostHandler.UpdateDraft)
		// Sprint 3 PR8: cancel a draft or scheduled post. Optimistic-locked
		// to lose cleanly against a concurrent publish flip.
		r.Post("/v1/social-posts/{id}/cancel", socialPostHandler.CancelPost)
		// Sprint 4 PR2: bulk publish — up to 50 posts in one request,
		// per-post idempotency, partial-success semantics.
		r.Post("/v1/social-posts/bulk", socialPostHandler.CreateBulk)

		// Sprint 4 PR5: Managed Users view — list / detail of end
		// users onboarded via Connect, grouped by external_user_id.
		r.Get("/v1/users", managedUsersHandler.List)
		r.Get("/v1/users/{external_user_id}", managedUsersHandler.Get)
		// Hosted preview link (Sprint 2). Returns a 24h JWT-signed
		// URL the caller can share without exposing the API key.
		r.Post("/v1/social-posts/{id}/preview-link", previewHandler.CreateLink)

		// Analytics aggregations (API key)
		r.Get("/v1/analytics/summary", analyticsHandler.GetSummary)
		r.Get("/v1/analytics/trend", analyticsHandler.GetTrend)
		r.Get("/v1/analytics/by-platform", analyticsHandler.GetByPlatform)
		// Sprint 5 PR1: dimensional rollup with day/week/month
		// granularity + dynamic GROUP BY across platform / account /
		// external_user_id / status.
		r.Get("/v1/analytics/rollup", analyticsRollupHandler.GetRollup)

		r.Post("/v1/webhooks", webhookSubHandler.Create)
		r.Get("/v1/webhooks", webhookSubHandler.List)
		r.Get("/v1/webhooks/{id}", webhookSubHandler.Get)
		r.Patch("/v1/webhooks/{id}", webhookSubHandler.Update)
		r.Delete("/v1/webhooks/{id}", webhookSubHandler.Delete)
		r.Post("/v1/webhooks/{id}/rotate", webhookSubHandler.Rotate)

		r.Get("/v1/oauth/connect/{platform}", oauthHandler.Connect)
		r.Get("/v1/usage", billingHandler.GetUsage)
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
	planEnvMap := map[string]string{
		"p10":   "STRIPE_PRICE_ID_10",
		"p25":   "STRIPE_PRICE_ID_25",
		"p50":   "STRIPE_PRICE_ID_50",
		"p75":   "STRIPE_PRICE_ID_75",
		"p150":  "STRIPE_PRICE_ID_150",
		"p300":  "STRIPE_PRICE_ID_300",
		"p500":  "STRIPE_PRICE_ID_500",
		"p1000": "STRIPE_PRICE_ID_1000",
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

// instagramConnectEnabled is the Sprint 5 PR3 feature flag for the
// Instagram Connect path. Returns true when CONNECT_INSTAGRAM_ENABLED
// is set to a truthy value (1, true, yes, on — case-insensitive).
// Anything else (including the unset default) keeps the platform out
// of the Connect registry, so customer dashboards don't show an
// Instagram tile that bounces them off Meta App Review failures.
func instagramConnectEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CONNECT_INSTAGRAM_ENABLED")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// threadsConnectEnabled is the Sprint 5 PR4 feature flag for the
// Threads Connect path. Same shape and semantics as the Instagram
// gate above — keeps the platform out of the Connect registry until
// Meta App Review approves the Threads app.
func threadsConnectEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CONNECT_THREADS_ENABLED")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
