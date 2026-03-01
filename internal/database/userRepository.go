package database

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/AVGsync/study_flow_api/internal/models"
)

type UserRepository struct {
	database *DB
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.UserResponse, error) {
	u := &models.UserResponse{}
	err := r.database.db.QueryRowContext(
		ctx,
		"SELECT id, login, email, role FROM users WHERE id = $1",
		id,
	).Scan(
		&u.ID,
		&u.Login,
		&u.Email,
		&u.Role,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			slog.Info("user not found", "id", id)
			return nil, err
		}
		slog.Error("failed to find user by id", "id", id, "error", err)
		return nil, err
	}

	return u, nil
}

func (r *UserRepository) Update(ctx context.Context, id string, upd *models.UserUpdate) error {
	u := &models.User{}
	err := r.database.db.QueryRowContext(
		ctx,
		"SELECT login, email FROM users WHERE id = $1",
		id,
	).Scan(&u.Login, &u.Email)
	if err != nil {
		slog.Error("failed to get user before update", "id", id, "err", err)
		return err
	}

	if upd.Login != nil {
		u.Login = *upd.Login
	}
	if upd.Email != nil {
		u.Email = *upd.Email
	}

	_, err = r.database.db.ExecContext(
		ctx,
		"UPDATE users SET login = $1, email = $2 WHERE id = $3",
		u.Login,
		u.Email,
		id,
	)
	if err != nil {
		slog.Error("failed to update user", "id", id, "err", err)
		return err
	}

	return nil
}

func (r *UserRepository) GetPasswordHashByID(ctx context.Context, id string) (string, error) {
	var hashedPassword string
	err := r.database.db.QueryRowContext(
		ctx,
		"SELECT hashed_password FROM users WHERE id = $1",
		id,
	).Scan(&hashedPassword)
	if err != nil {
		slog.Error("failed to get user password hash", "id", id, "err", err)
		return "", err
	}

	return hashedPassword, nil
}

func (r *UserRepository) UpdatePasswordHash(ctx context.Context, id, hashedPassword string) error {
	_, err := r.database.db.ExecContext(
		ctx,
		"UPDATE users SET hashed_password = $1 WHERE id = $2",
		hashedPassword,
		id,
	)
	if err != nil {
		slog.Error("failed to update user password", "id", id, "err", err)
		return err
	}

	return nil
}
