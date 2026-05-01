package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed sql/schema.sql
var schemaSQL string

// Open creates a sqlite connection at dsn and ensures the current schema
// exists before returning the DB handle.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := InitSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// InitSchema executes schema.sql statements with IF NOT EXISTS semantics.
func InitSchema(db *sql.DB) error {
	for _, stmt := range strings.Split(schemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("apply schema statement %q: %w", firstLine(stmt), err)
		}
	}
	return nil
}

func firstLine(sql string) string {
	parts := strings.Split(sql, "\n")
	if len(parts) == 0 {
		return sql
	}
	return strings.TrimSpace(parts[0])
}
