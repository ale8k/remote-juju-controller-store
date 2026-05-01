package db

import (
	"database/sql"

	"github.com/ale8k/remote-juju-controller-store/internal/db/sqlc"
)

// Repository groups the generated sqlc queries and shared DB handle.
type Repository struct {
	DB      *sql.DB
	Queries *sqlc.Queries
}

// NewRepository builds a sqlc-backed repository for an initialized DB.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		DB:      db,
		Queries: sqlc.New(db),
	}
}
