package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/AVGsync/study_flow_api/internal/database"
)

type UserHandler struct {
	user *database.UserRepository
}

func NewUserHandler(userRepository *database.UserRepository) *UserHandler {
	return &UserHandler{user: userRepository}
}

func (h *UserHandler) FindByEmail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.URL.Query().Get("email")
		if email == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}
		u, err := h.user.FindByEmail(email)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(u); err != nil {
			http.Error(w, "failed to encode user", http.StatusInternalServerError)
			return
		}
	}
}
