package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
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

	// Set up structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
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
	quotaChecker := quota.NewChecker(queries)

	// Start background workers
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	tokenWorker := worker.NewTokenRefreshWorker(queries, encryptor)
	go tokenWorker.Start(workerCtx)

	webhookWorker := worker.NewWebhookDeliveryWorker(queries)
	go webhookWorker.Start(workerCtx)

	r := chi.NewRouter()

	// Global middleware
	r.Use(mw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://app.unipost.dev", "http://localhost:3000"},
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
		r.Delete("/v1/social-posts/{id}", socialPostHandler.Delete)

		r.Post("/v1/webhooks", webhookSubHandler.Create)
		r.Get("/v1/webhooks", webhookSubHandler.List)

		r.Get("/v1/oauth/connect/{platform}", oauthHandler.Connect)
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
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
