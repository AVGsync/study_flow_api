package database

import (
	"github.com/AVGsync/study_flow_api/internal/models"
)

type UserRepository struct {
	database *DB
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	u := &models.User{}
	if err := r.database.db.QueryRow(
		"SELECT id, username, email, password FROM users WHERE email = $1",
		email,
	).Scan(
		&u.ID,
		&u.Username,
		&u.Email,
		&u.Password,
	); err != nil {
		return nil, err
	}
	return u, nil
}