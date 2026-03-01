package database

import (
	"context"
	"database/sql"
	"log/slog"
    "errors"

	"github.com/AVGsync/study_flow_api/internal/models"
	"github.com/AVGsync/study_flow_api/internal/security"
)

type UserRepository struct {
	database *DB
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.UserResponse, error) {
    u := &models.UserResponse{}
    err := r.database.db.QueryRowContext(
        ctx,
        "SELECT id, login, email, hashed_password, role FROM users WHERE id = $1",
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

func (r *UserRepository) ChangePassword(ctx context.Context, id string, oldPassword, newPassword string) error {
    u := &models.User{}
    err := r.database.db.QueryRowContext(
        ctx,
        "SELECT hashed_password FROM users WHERE id = $1",
        id,
    ).Scan(&u.Password)
    if err != nil {
        slog.Error("failed to get user before update", "id", id, "err", err)
        return err
    }

    if !security.CheckPassword(oldPassword, u.Password) {
        return errors.New("old password is incorrect")
    }

    newHashedPassword, err := security.HashPassword(newPassword)
    if err != nil {
        slog.Error("failed to hash new password", "id", id, "err", err)
        return err
    }

    _, err = r.database.db.ExecContext(
        ctx,
        "UPDATE users SET hashed_password = $1 WHERE id = $2",
        newHashedPassword,
        id,
    )
    if err != nil {
        slog.Error("failed to update user password", "id", id, "err", err)
        return err
    }

    return nil
}