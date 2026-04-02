package main

import (
	"context"
	"log"
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

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/worker"
)

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// Initialize AES encryptor for token encryption
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY is required")
	}
	encryptor, err := crypto.NewAESEncryptor(encryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize encryptor: %v", err)
	}

	// Register platform adapters
	platform.Register(platform.NewBlueskyAdapter())

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Run database migrations
	if err := db.RunMigrations(databaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	queries := db.New(pool)

	// Start background workers
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	tokenWorker := worker.NewTokenRefreshWorker(queries, encryptor)
	go tokenWorker.Start(workerCtx)

	webhookWorker := worker.NewWebhookDeliveryWorker(queries)
	go webhookWorker.Start(workerCtx)

	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://app.unipost.dev", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Handlers
	healthHandler := handler.NewHealthHandler()
	webhookHandler := handler.NewWebhookHandler(queries)
	projectHandler := handler.NewProjectHandler(queries)
	apiKeyHandler := handler.NewAPIKeyHandler(queries)
	socialAccountHandler := handler.NewSocialAccountHandler(queries, encryptor)
	socialPostHandler := handler.NewSocialPostHandler(queries, encryptor)
	webhookSubHandler := handler.NewWebhookSubscriptionHandler(queries)

	// Public routes
	r.Get("/health", healthHandler.Health)

	// Webhook routes (verified by Clerk webhook secret)
	r.Post("/webhooks/clerk", webhookHandler.HandleClerk)

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
		log.Printf("Server starting on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}
