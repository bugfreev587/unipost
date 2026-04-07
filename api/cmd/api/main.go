package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
	mw "github.com/xiaoboyu/unipost-api/internal/middleware"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
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

	// Initialize Stripe
	if sk := os.Getenv("STRIPE_SECRET_KEY"); sk != "" {
		stripe.Key = sk
		slog.Info("stripe initialized")
	}

	// Register platform adapters
	platform.Register(platform.NewBlueskyAdapter())
	platform.Register(platform.NewLinkedInAdapter())
	platform.Register(platform.NewInstagramAdapter())
	platform.Register(platform.NewThreadsAdapter())

	platform.Register(platform.NewTwitterAdapter()) // Native mode only — requires user's own API credentials

	// Conditionally register adapters that need credentials
	if os.Getenv("TIKTOK_CLIENT_KEY") != "" {
		platform.Register(platform.NewTikTokAdapter())
		slog.Info("tiktok adapter registered")
	}
	if os.Getenv("YOUTUBE_CLIENT_ID") != "" {
		platform.Register(platform.NewYouTubeAdapter())
		slog.Info("youtube adapter registered")
	}

	ctx := context.Background()

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

	// Sync Stripe price IDs from env vars into plans table
	syncStripePriceIDs(ctx, queries)
	quotaChecker := quota.NewChecker(queries)

	// Start background workers
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	tokenWorker := worker.NewTokenRefreshWorker(queries, encryptor)
	go tokenWorker.Start(workerCtx)

	webhookWorker := worker.NewWebhookDeliveryWorker(queries)
	go webhookWorker.Start(workerCtx)

	schedulerWorker := worker.NewSchedulerWorker(queries, encryptor)
	go schedulerWorker.Start(workerCtx)

	analyticsRefreshWorker := worker.NewAnalyticsRefreshWorker(queries, encryptor)
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
	projectHandler := handler.NewProjectHandler(queries)
	apiKeyHandler := handler.NewAPIKeyHandler(queries)
	socialAccountHandler := handler.NewSocialAccountHandler(queries, encryptor)
	socialPostHandler := handler.NewSocialPostHandler(queries, encryptor, quotaChecker)
	webhookSubHandler := handler.NewWebhookSubscriptionHandler(queries)
	oauthHandler := handler.NewOAuthHandler(queries, encryptor)
	platformCredHandler := handler.NewPlatformCredentialHandler(queries, encryptor)
	billingHandler := handler.NewBillingHandler(queries, quotaChecker)
	stripeWebhookHandler := handler.NewStripeWebhookHandler(queries)
	analyticsHandler := handler.NewAnalyticsHandler(queries, encryptor)

	// Public routes
	r.Get("/health", healthHandler.Health)
	r.Get("/v1/plans", billingHandler.ListPlans)

	// Webhook routes (no API key auth, verified by signatures)
	r.Post("/webhooks/clerk", webhookHandler.HandleClerk)
	r.Post("/webhooks/stripe", stripeWebhookHandler.HandleStripe)

	// OAuth callback routes (no auth — called by OAuth providers)
	r.Get("/v1/oauth/callback/{platform}", oauthHandler.Callback)

	// Dashboard routes (Clerk session auth)
	r.Group(func(r chi.Router) {
		r.Use(auth.ClerkSessionMiddleware)

		r.Get("/v1/projects", projectHandler.List)
		r.Post("/v1/projects", projectHandler.Create)
		r.Get("/v1/projects/{id}", projectHandler.Get)
		r.Patch("/v1/projects/{id}", projectHandler.Update)
		r.Delete("/v1/projects/{id}", projectHandler.Delete)

		r.Get("/v1/projects/{projectID}/api-keys", apiKeyHandler.List)
		r.Post("/v1/projects/{projectID}/api-keys", apiKeyHandler.Create)
		r.Delete("/v1/projects/{projectID}/api-keys/{keyID}", apiKeyHandler.Revoke)

		// Social accounts (dashboard)
		r.Get("/v1/projects/{projectID}/social-accounts", socialAccountHandler.List)
		r.Post("/v1/projects/{projectID}/social-accounts/connect", socialAccountHandler.Connect)
		r.Delete("/v1/projects/{projectID}/social-accounts/{accountID}", socialAccountHandler.Disconnect)

		// Social posts (dashboard)
		r.Get("/v1/projects/{projectID}/social-posts", socialPostHandler.List)
		r.Post("/v1/projects/{projectID}/social-posts", socialPostHandler.Create)

		// OAuth connect (dashboard)
		r.Get("/v1/projects/{projectID}/oauth/connect/{platform}", oauthHandler.Connect)

		// Platform credentials (White Label)
		r.Post("/v1/projects/{projectID}/platform-credentials", platformCredHandler.Create)
		r.Get("/v1/projects/{projectID}/platform-credentials", platformCredHandler.List)
		r.Delete("/v1/projects/{projectID}/platform-credentials/{platform}", platformCredHandler.Delete)

		// Billing (dashboard)
		r.Get("/v1/projects/{projectID}/billing", billingHandler.GetBilling)
		r.Post("/v1/projects/{projectID}/billing/checkout", billingHandler.CreateCheckout)
		r.Post("/v1/projects/{projectID}/billing/portal", billingHandler.CreatePortal)

		// Analytics (dashboard)
		r.Get("/v1/projects/{projectID}/analytics/summary", analyticsHandler.GetSummary)
		r.Get("/v1/projects/{projectID}/analytics/trend", analyticsHandler.GetTrend)
		r.Get("/v1/projects/{projectID}/analytics/by-platform", analyticsHandler.GetByPlatform)
		// Per-post analytics (project-scoped). Mirrors the API-key route below;
		// the dashboard has been calling this URL but it wasn't wired until now.
		r.Get("/v1/projects/{projectID}/social-posts/{id}/analytics", analyticsHandler.GetAnalytics)
	})

	// Public API routes (API key auth)
	r.Group(func(r chi.Router) {
		r.Use(auth.APIKeyMiddleware(queries))

		r.Get("/v1/social-accounts", socialAccountHandler.List)
		r.Post("/v1/social-accounts/connect", socialAccountHandler.Connect)
		r.Delete("/v1/social-accounts/{id}", socialAccountHandler.Disconnect)

		r.Get("/v1/social-posts", socialPostHandler.List)
		r.Post("/v1/social-posts", socialPostHandler.Create)
		r.Get("/v1/social-posts/{id}", socialPostHandler.Get)
		r.Get("/v1/social-posts/{id}/analytics", analyticsHandler.GetAnalytics)
		r.Delete("/v1/social-posts/{id}", socialPostHandler.Delete)

		// Analytics aggregations (API key)
		r.Get("/v1/analytics/summary", analyticsHandler.GetSummary)
		r.Get("/v1/analytics/trend", analyticsHandler.GetTrend)
		r.Get("/v1/analytics/by-platform", analyticsHandler.GetByPlatform)

		r.Post("/v1/webhooks", webhookSubHandler.Create)
		r.Get("/v1/webhooks", webhookSubHandler.List)

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

// syncStripePriceIDs reads Stripe price IDs from env vars and updates the plans table.
// Env var format: STRIPE_PRICE_ID_10, STRIPE_PRICE_ID_25, etc.
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
