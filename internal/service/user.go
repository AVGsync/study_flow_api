package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/AVGsync/study_flow_api/internal/authctx"
	"github.com/AVGsync/study_flow_api/internal/model"
)

var ErrInvalidOldPassword = errors.New("old password is incorrect")

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*model.UserResponse, error)
	Update(ctx context.Context, id string, upd *model.UserUpdateRequest) error
	GetPasswordHashByID(ctx context.Context, id string) (string, error)
	UpdatePasswordHash(ctx context.Context, id, hashedPassword string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(plain, hashed string) bool
}

type Cache interface {
	SetUser(ctx context.Context, user *model.UserResponse) error
	GetUser(ctx context.Context, id string) (*model.UserResponse, error)
	DeleteUser(ctx context.Context, id string) error
}

type UserService struct {
	repo   UserRepository
	hasher PasswordHasher
	cache  Cache
}

func NewUserService(repo UserRepository, hasher PasswordHasher, cache Cache) *UserService {
	return &UserService{
		repo:   repo,
		hasher: hasher,
		cache:  cache,
	}
}

func (s *UserService) FindByID(ctx context.Context, id string) (*model.UserResponse, error) {
	user, err := s.cache.GetUser(ctx, id)
	if err == nil {
		return user, nil
	}

	user, err = s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cache.SetUser(ctx, user)

	return user, nil
}

func (s *UserService) Update(ctx context.Context, id string, upd *model.UserUpdateRequest) error {
	err := s.cache.DeleteUser(ctx, id)
	if err != nil {
		slog.Warn("failed to delete user from cache before update", "id", id, "error", err)
	}
	return s.repo.Update(ctx, id, upd)
}

func (s *UserService) ChangePassword(ctx context.Context, id, oldPassword, newPassword string) error {
	currentHash, err := s.repo.GetPasswordHashByID(ctx, id)
	if err != nil {
		return err
	}

	if !authctx.IsAdminFromContext(ctx) {
		if !s.hasher.Compare(oldPassword, currentHash) {
			return ErrInvalidOldPassword
		}
	}

	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}

	return s.repo.UpdatePasswordHash(ctx, id, newHash)
}
