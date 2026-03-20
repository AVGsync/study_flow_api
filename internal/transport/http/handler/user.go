package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AVGsync/study_flow_api/internal/authctx"
	"github.com/AVGsync/study_flow_api/internal/model"
	"github.com/AVGsync/study_flow_api/internal/service"
	"github.com/google/uuid"
)

type UserUseCase interface {
	FindByID(ctx context.Context, id string) (*model.UserResponse, error)
	Update(ctx context.Context, id string, upd *model.UserUpdateRequest) error
	ChangePassword(ctx context.Context, id, oldPassword, newPassword string) error
}

type Validator interface {
	ValidateStruct(s interface{}) (bool, error)
}

type UserHandler struct {
	user UserUseCase
	v    Validator
}

func NewUserHandler(userUseCase UserUseCase, v Validator) *UserHandler {
	return &UserHandler{user: userUseCase, v: v}
}

func (h *UserHandler) UserByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := authctx.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if authctx.IsAdminFromContext(r.Context()) {
			if idStr := r.URL.Query().Get("id"); idStr != "" {
				uid, err := uuid.Parse(idStr)
				if err != nil {
					http.Error(w, "invalid id", http.StatusBadRequest)
					return
				}
				userID = uid.String()
			}
		}

		u, err := h.user.FindByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)

		if err := enc.Encode(u); err != nil {
			http.Error(w, "failed to encode user", http.StatusInternalServerError)
			return
		}
	}
}

func (h *UserHandler) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := authctx.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Если запрос идет от администратора, можно обновлять любого пользователя через query-параметр id.
		if authctx.IsAdminFromContext(r.Context()) {
			if idStr := r.URL.Query().Get("id"); idStr != "" {
				uid, err := uuid.Parse(idStr)
				if err != nil {
					http.Error(w, "invalid id", http.StatusBadRequest)
					return
				}
				userID = uid.String()
			}
		}

		// В updateData попадают только те поля, которые реально пришли в теле запроса.
		updateData := &model.UserUpdateRequest{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Проверяем, что запрос не пустой и в нем есть хотя бы одно поле для изменения.
		if updateData.Login == nil && updateData.Email == nil {
			http.Error(w, "nothing to update", http.StatusBadRequest)
			return
		}

		// Валидация остается на transport-слое, чтобы сервис получал уже нормализованный запрос.
		ok, errs := h.v.ValidateStruct(updateData)
		if !ok {
			http.Error(w, errs.Error(), http.StatusBadRequest)
			return
		}

		err := h.user.Update(r.Context(), userID, updateData)
		if err != nil {
			http.Error(w, "failed to update user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)

		if err := enc.Encode(updateData); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

func (h *UserHandler) ChangePassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := authctx.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if authctx.IsAdminFromContext(r.Context()) {
			if idStr := r.URL.Query().Get("id"); idStr != "" {
				uid, err := uuid.Parse(idStr)
				if err != nil {
					http.Error(w, "invalid id", http.StatusBadRequest)
					return
				}
				userID = uid.String()
			}
		}

		changePass := &model.ChangePasswordRequest{}
		if err := json.NewDecoder(r.Body).Decode(changePass); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err := h.user.ChangePassword(r.Context(), userID, changePass.OldPassword, changePass.NewPassword)
		if err != nil {
			if errors.Is(err, service.ErrInvalidOldPassword) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to change password", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
