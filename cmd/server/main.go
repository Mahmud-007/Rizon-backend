package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"rizon-backend/internal/database"
	"rizon-backend/internal/handlers"
	customMiddleware "rizon-backend/internal/middleware"
	"rizon-backend/internal/repository"
	"rizon-backend/internal/slack"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env (ignore error in production — env vars set directly)
	_ = godotenv.Load()

	// Required env vars
	mongoURI := getEnv("MONGODB_URI", "")
	dbName := getEnv("DB_NAME", "rizon")
	jwtSecret := getEnv("JWT_SECRET", "")
	port := getEnv("PORT", "8080")

	if mongoURI == "" {
		log.Fatal("❌ MONGODB_URI is required")
	}
	if jwtSecret == "" {
		log.Fatal("❌ JWT_SECRET is required")
	}

	// Connect to MongoDB
	if err := database.Connect(mongoURI, dbName); err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepo()
	tokenRepo := repository.NewAuthTokenRepo()
	feedbackRepo := repository.NewFeedbackRepo()

	// Ensure indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := userRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("⚠️  Warning: failed to create user indexes: %v", err)
	}
	if err := tokenRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("⚠️  Warning: failed to create token indexes: %v", err)
	}
	if err := feedbackRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("⚠️  Warning: failed to create feedback indexes: %v", err)
	}

	// Initialize Slack notifier (mock)
	notifier := slack.NewMockSlack()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(tokenRepo, userRepo, jwtSecret)
	feedbackHandler := handlers.NewFeedbackHandler(feedbackRepo, notifier)
	userHandler := handlers.NewUserHandler(userRepo)

	// Setup chi router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"rizon-backend"}`))
	})

	// Public routes (no auth required)
	r.Post("/auth/request", authHandler.RequestLogin)
	r.Get("/auth/verify", authHandler.VerifyToken)
	r.Get("/auth/redirect", authHandler.RedirectToApp)

	// Protected routes (JWT required)
	r.Group(func(r chi.Router) {
		r.Use(customMiddleware.JWTAuth(jwtSecret))

		r.Post("/feedback", feedbackHandler.SubmitFeedback)
		r.Get("/user/status", userHandler.GetStatus)
		r.Patch("/user/onboarding", userHandler.CompleteOnboarding)
	})

	// Start server
	log.Printf("🚀 Rizon backend starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
