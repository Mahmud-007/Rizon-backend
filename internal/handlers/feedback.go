package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"rizon-backend/internal/middleware"
	"rizon-backend/internal/models"
	"rizon-backend/internal/repository"
	"rizon-backend/internal/slack"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type FeedbackHandler struct {
	feedbackRepo *repository.FeedbackRepo
	notifier     slack.Notifier
}

func NewFeedbackHandler(feedbackRepo *repository.FeedbackRepo, notifier slack.Notifier) *FeedbackHandler {
	return &FeedbackHandler{
		feedbackRepo: feedbackRepo,
		notifier:     notifier,
	}
}

type SubmitFeedbackRequest struct {
	Text           string `json:"text"`
	Rating         int    `json:"rating"`
	IdempotencyKey string `json:"idempotency_key"`
}

// --- POST /feedback ---

func (h *FeedbackHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	userIDHex := middleware.GetUserID(r.Context())
	if userIDHex == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req SubmitFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "feedback text is required"})
		return
	}

	if req.IdempotencyKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "idempotency_key is required"})
		return
	}

	// Idempotency check — prevent duplicate submissions
	existing, err := h.feedbackRepo.FindByIdempotencyKey(r.Context(), req.IdempotencyKey)
	if err != nil {
		log.Printf("Error checking idempotency: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if existing != nil {
		// Already submitted — return the existing feedback (idempotent behavior)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":  "feedback already submitted",
			"feedback": existing,
		})
		return
	}

	userID, err := bson.ObjectIDFromHex(userIDHex)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	feedback := &models.Feedback{
		UserID:         userID,
		Text:           req.Text,
		Rating:         req.Rating,
		IdempotencyKey: req.IdempotencyKey,
	}

	if err := h.feedbackRepo.Create(r.Context(), feedback); err != nil {
		log.Printf("Error creating feedback: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to submit feedback"})
		return
	}

	// Fire Slack notification in a background goroutine (non-blocking)
	go func() {
		message := formatSlackMessage(userIDHex, req.Text, req.Rating)
		if err := h.notifier.Publish(context.Background(), message); err != nil {
			log.Printf("Error publishing to Slack: %v", err)
		}
	}()

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":  "feedback submitted successfully",
		"feedback": feedback,
	})
}

func formatSlackMessage(userID, text string, rating int) string {
	stars := ""
	for i := 0; i < rating; i++ {
		stars += "⭐"
	}
	return "📝 *New Feedback Received*\n" +
		"User: `" + userID + "`\n" +
		"Rating: " + stars + "\n" +
		"Feedback: " + text
}
