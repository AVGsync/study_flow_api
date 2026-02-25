package database

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type DB struct {
	config *Config
	db     *sql.DB
	userRepository *UserRepository
}

func New(config *Config) *DB {
	return &DB{
		config: config,
	}
}

func (d *DB) Open() error {
	db, err := sql.Open("postgres", d.config.DatabaseURL)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}

	d.db = db
	return nil
}

func (d *DB) Close() {
	d.db.Close()
}

func (d *DB) User() *UserRepository {
	if d.userRepository != nil {
		return d.userRepository
	}

	d.userRepository = &UserRepository{database: d}
	return d.userRepository
}
