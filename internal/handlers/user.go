package handlers

import (
	"log"
	"net/http"

	"rizon-backend/internal/middleware"
	"rizon-backend/internal/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserHandler struct {
	userRepo *repository.UserRepo
}

func NewUserHandler(userRepo *repository.UserRepo) *UserHandler {
	return &UserHandler{
		userRepo: userRepo,
	}
}

// --- GET /user/status ---

func (h *UserHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userIDHex := middleware.GetUserID(r.Context())
	if userIDHex == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	userID, err := bson.ObjectIDFromHex(userIDHex)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		log.Printf("Error finding user: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"onboarding_completed": user.OnboardingCompleted,
	})
}

// --- PATCH /user/onboarding ---

func (h *UserHandler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	userIDHex := middleware.GetUserID(r.Context())
	if userIDHex == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	userID, err := bson.ObjectIDFromHex(userIDHex)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	if err := h.userRepo.UpdateOnboarding(r.Context(), userID, true); err != nil {
		log.Printf("Error updating onboarding: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update onboarding status"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "onboarding marked as completed",
	})
}
