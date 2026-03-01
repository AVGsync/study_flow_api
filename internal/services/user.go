package services

import (
	"context"
	"errors"

	"github.com/AVGsync/study_flow_api/internal/models"
)

var ErrInvalidOldPassword = errors.New("old password is incorrect")

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*models.UserResponse, error)
	Update(ctx context.Context, id string, upd *models.UserUpdate) error
	GetPasswordHashByID(ctx context.Context, id string) (string, error)
	UpdatePasswordHash(ctx context.Context, id, hashedPassword string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(plain, hashed string) bool
}

type UserService struct {
	repo   UserRepository
	hasher PasswordHasher
}

func NewUserService(repo UserRepository, hasher PasswordHasher) *UserService {
	return &UserService{
		repo:   repo,
		hasher: hasher,
	}
}

func (s *UserService) FindByID(ctx context.Context, id string) (*models.UserResponse, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) Update(ctx context.Context, id string, upd *models.UserUpdate) error {
	return s.repo.Update(ctx, id, upd)
}

func (s *UserService) ChangePassword(ctx context.Context, id, oldPassword, newPassword string) error {
	currentHash, err := s.repo.GetPasswordHashByID(ctx, id)
	if err != nil {
		return err
	}

	if !s.hasher.Compare(oldPassword, currentHash) {
		return ErrInvalidOldPassword
	}

	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}

	return s.repo.UpdatePasswordHash(ctx, id, newHash)
}
