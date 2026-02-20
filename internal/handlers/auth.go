package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"rizon-backend/internal/models"
	"rizon-backend/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/resend/resend-go/v2"
)

type AuthHandler struct {
	tokenRepo *repository.AuthTokenRepo
	userRepo  *repository.UserRepo
	jwtSecret string
}

func NewAuthHandler(tokenRepo *repository.AuthTokenRepo, userRepo *repository.UserRepo, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		tokenRepo: tokenRepo,
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

// --- Request / Response types ---

type RequestLoginRequest struct {
	Email string `json:"email"`
}

type VerifyResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// --- POST /auth/request ---

func (h *AuthHandler) RequestLogin(w http.ResponseWriter, r *http.Request) {
	var req RequestLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}

	// Rate limiting: max 5 requests per email in 10 minutes
	count, err := h.tokenRepo.CountRecentByEmail(r.Context(), req.Email, 10*time.Minute)
	if err != nil {
		log.Printf("Error checking rate limit: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if count >= 5 {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login requests, please try again later"})
		return
	}

	// Generate unique token
	tokenValue := uuid.New().String()

	// Store token in DB with 15-minute expiry
	authToken := &models.AuthToken{
		Email:     req.Email,
		Token:     tokenValue,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		IsUsed:    false,
	}
	if err := h.tokenRepo.Create(r.Context(), authToken); err != nil {
		log.Printf("Error creating auth token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create login token"})
		return
	}

	// Build the HTTPS redirect URL (email-safe) instead of rizon:// directly
	// Gmail/Outlook strip custom URL schemes, so we link to our server first
	// Dynamically detect the base URL from the incoming request
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}
	emailLink := fmt.Sprintf("%s/auth/redirect?token=%s", baseURL, tokenValue)

	if err := sendLoginEmail(req.Email, emailLink); err != nil {
		log.Printf("Error sending email: %v", err)
		// Don't fail the request — token is created, email sending is best-effort
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "login link generated (email delivery may be delayed)",
			"note":    "check server logs if email was not received",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "login link sent to your email",
	})
}

// --- GET /auth/verify ---

func (h *AuthHandler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	tokenValue := r.URL.Query().Get("token")
	if tokenValue == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
		return
	}

	// Find token in DB
	authToken, err := h.tokenRepo.FindByToken(r.Context(), tokenValue)
	if err != nil {
		log.Printf("Error finding token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if authToken == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	// Validate: not expired
	if authToken.IsExpired() {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token has expired"})
		return
	}

	// Validate: not already used (single-use)
	if authToken.IsUsed {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token has already been used"})
		return
	}

	// Mark token as used
	if err := h.tokenRepo.MarkUsed(r.Context(), tokenValue); err != nil {
		log.Printf("Error marking token as used: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Find or create user
	user, err := h.userRepo.FindOrCreate(r.Context(), authToken.Email)
	if err != nil {
		log.Printf("Error finding/creating user: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Generate JWT with 30-day expiry
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID.Hex(),
		"email":   user.Email,
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	tokenString, err := jwtToken.SignedString([]byte(h.jwtSecret))
	if err != nil {
		log.Printf("Error signing JWT: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, VerifyResponse{
		Token: tokenString,
		User:  user,
	})
}

// --- GET /auth/redirect ---
// This endpoint is clicked from the email. It serves an HTML page that
// redirects the user's phone to the rizon:// deep link (which opens the app).

func (h *AuthHandler) RedirectToApp(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	deepLink := fmt.Sprintf("rizon://login?token=%s", token)

	// Serve an HTML page that:
	// 1. Immediately tries to open the app via deep link
	// 2. Shows a fallback button if auto-redirect doesn't work
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Opening Rizon...</title>
	<style>
		body { font-family: -apple-system, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f3ff; }
		.card { text-align: center; padding: 40px; background: white; border-radius: 16px; box-shadow: 0 4px 24px rgba(0,0,0,0.1); max-width: 400px; }
		h1 { color: #333; font-size: 24px; }
		p { color: #666; font-size: 16px; line-height: 1.5; }
		.btn { display: inline-block; background: #6366f1; color: white; padding: 14px 32px; border-radius: 10px; text-decoration: none; font-weight: 600; font-size: 16px; margin-top: 16px; }
		.btn:hover { background: #4f46e5; }
		.spinner { width: 40px; height: 40px; border: 4px solid #e5e7eb; border-top: 4px solid #6366f1; border-radius: 50%%; animation: spin 1s linear infinite; margin: 0 auto 20px; }
		@keyframes spin { to { transform: rotate(360deg); } }
	</style>
</head>
<body>
	<div class="card">
		<div class="spinner"></div>
		<h1>Opening Rizon...</h1>
		<p>You should be redirected to the app automatically.</p>
		<p>If nothing happens, tap the button below:</p>
		<a href="%s" class="btn">Open Rizon App</a>
	</div>
	<script>
		// Auto-redirect to the app deep link
		window.location.href = "%s";
	</script>
</body>
</html>`, deepLink, deepLink)
}

// --- Helpers ---

func sendLoginEmail(to, link string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	fromEmail := os.Getenv("FROM_EMAIL")

	if apiKey == "" {
		log.Println("⚠️  RESEND_API_KEY not set, skipping email send")
		log.Printf("📧 [Dev Mode] Login link for %s: %s", to, link)
		return nil
	}

	client := resend.NewClient(apiKey)

	params := &resend.SendEmailRequest{
		From:    fromEmail,
		To:      []string{to},
		Subject: "Your Rizon Login Link",
		Html: fmt.Sprintf(`
			<div style="font-family: sans-serif; max-width: 480px; margin: 0 auto; padding: 24px;">
				<h2 style="color: #333;">Welcome to Rizon! 🚀</h2>
				<p>Click the button below to log in to your account:</p>
				<a href="%s" style="display: inline-block; background: #6366f1; color: white; padding: 12px 24px; border-radius: 8px; text-decoration: none; font-weight: 600;">
					Open Rizon App
				</a>
				<p style="color: #888; font-size: 14px; margin-top: 16px;">
					This link expires in 15 minutes and can only be used once.
				</p>
				<p style="color: #aaa; font-size: 12px;">
					If you didn't request this, you can safely ignore this email.
				</p>
			</div>
		`, link),
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	log.Printf("📧 Email sent successfully (ID: %s) — Link: %s", sent.Id, link)
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
