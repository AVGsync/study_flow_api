package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/AVGsync/study_flow_api/internal/auth"
	"github.com/AVGsync/study_flow_api/internal/database"
	"github.com/AVGsync/study_flow_api/internal/models"
)

type UserHandler struct {
	user *database.UserRepository
}

func NewUserHandler(userRepository *database.UserRepository) *UserHandler {
	return &UserHandler{user: userRepository}
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

func (h *UserHandler) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		
		//Записываем в updateData данные из тела запроса, которые нужно обновить
		updateData := &models.UserUpdate{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		//Проверяем, что хотя бы одно поле для обновления было передано
		if updateData.Login == nil && updateData.Email == nil {
			http.Error(w, "nothing to update", http.StatusBadRequest)
			return
		}

		//Вызываем метод Update репозитория для обновления данных пользователя
		err := h.user.Update(r.Context(),userID, updateData)
		if err != nil {
			http.Error(w, "failed to update user", http.StatusInternalServerError)
			return
		}

		//После успешного обновления возвращаем обновленные данные пользователя
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(updateData); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

func (h *UserHandler) ChangePassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		changePass := &models.ChangePasswordRequest{}
		if err := json.NewDecoder(r.Body).Decode(changePass); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err := h.user.ChangePassword(r.Context(), userID, changePass.OldPassword, changePass.NewPassword)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

