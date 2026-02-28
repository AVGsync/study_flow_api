package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/AVGsync/study_flow_api/internal/database"
	"github.com/AVGsync/study_flow_api/internal/auth"
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
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			slog.Error("failed to find user by email", "email", email, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ") 

		if err := enc.Encode(u); err != nil {
				http.Error(w, "failed to encode user", http.StatusInternalServerError)
				return
		}
	}
}

func (h *UserHandler) Me() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID, ok := auth.UserIDFromContext(r.Context())
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        u, err := h.user.FindByID(r.Context(), userID)
        if err != nil {
            http.Error(w, "user not found", http.StatusNotFound)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        enc := json.NewEncoder(w)
        enc.SetIndent("", "  ")
        if err := enc.Encode(u); err != nil {
            http.Error(w, "failed to encode user", http.StatusInternalServerError)
            return
        }
    }
}
