package services

import (
	"context"
	"errors"

	"github.com/AVGsync/study_flow_api/internal/auth"
	"github.com/AVGsync/study_flow_api/internal/models"
)

var ErrInvalidOldPassword = errors.New("old password is incorrect")

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*models.UserResponse, error)
	Update(ctx context.Context, id string, upd *models.UserUpdateRequest) error
	GetPasswordHashByID(ctx context.Context, id string) (string, error)
	UpdatePasswordHash(ctx context.Context, id, hashedPassword string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(plain, hashed string) bool
}

type Cache interface {
	Set(ctx context.Context, user *models.UserResponse) error
	Get(ctx context.Context, id string) (*models.UserResponse, error)
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

func (s *UserService) FindByID(ctx context.Context, id string) (*models.UserResponse, error) {
	user, err := s.cache.Get(ctx, id)
	if err == nil {
		return user, nil
	}

	user, err = s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cache.Set(ctx, user)

	return user, nil
}

func (s *UserService) Update(ctx context.Context, id string, upd *models.UserUpdateRequest) error {
	return s.repo.Update(ctx, id, upd)
}

func (s *UserService) ChangePassword(ctx context.Context, id, oldPassword, newPassword string) error {
	currentHash, err := s.repo.GetPasswordHashByID(ctx, id)
	if err != nil {
		return err
	}


	if !auth.IsAdminFromContext(ctx) {
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
