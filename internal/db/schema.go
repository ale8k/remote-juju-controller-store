package db

import (
	"database/sql"
	"strings"
)

// schemaVersion is incremented whenever tables are added or columns change in
// a way that requires a fresh database.
const schemaVersion = 2

// schemaVersionTable is always created first; it is never dropped.
const schemaVersionTable = `
CREATE TABLE IF NOT EXISTS schema_version (
	version INTEGER PRIMARY KEY
);
`

// alwaysCreate is applied on every startup. These tables existed since v1 and
// are version-independent (keys, users). They use IF NOT EXISTS so they are
// safe to re-run even when the schema is already at the current version.
const alwaysCreate = `
CREATE TABLE IF NOT EXISTS users (
	id         TEXT PRIMARY KEY,
	email      TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS signing_keys (
	id          TEXT PRIMARY KEY,
	key_pem     TEXT NOT NULL,
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	retired_at  DATETIME
);

CREATE TABLE IF NOT EXISTS controller_signing_keys (
	id          TEXT PRIMARY KEY,
	key_pem     TEXT NOT NULL,
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	retired_at  DATETIME
);
`

// v2Schema creates all namespace-aware tables. Called only when the stored
// schema version is less than 2.
const v2Schema = `
CREATE TABLE namespaces (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE,
	owner_id   TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE namespace_members (
	namespace_id TEXT NOT NULL,
	user_id      TEXT NOT NULL,
	PRIMARY KEY (namespace_id, user_id)
);

CREATE TABLE controllers (
	namespace_id TEXT NOT NULL DEFAULT '',
	name         TEXT NOT NULL,
	details_json TEXT NOT NULL,
	PRIMARY KEY (namespace_id, name)
);

CREATE TABLE controller_meta (
	namespace_id TEXT NOT NULL DEFAULT '',
	key          TEXT NOT NULL,
	value        TEXT NOT NULL,
	PRIMARY KEY (namespace_id, key)
);

CREATE TABLE models (
	namespace_id    TEXT NOT NULL DEFAULT '',
	controller_name TEXT NOT NULL,
	model_name      TEXT NOT NULL,
	details_json    TEXT NOT NULL,
	PRIMARY KEY (namespace_id, controller_name, model_name)
);

CREATE TABLE model_meta (
	namespace_id    TEXT NOT NULL DEFAULT '',
	controller_name TEXT NOT NULL,
	key             TEXT NOT NULL,
	value           TEXT NOT NULL,
	PRIMARY KEY (namespace_id, controller_name, key)
);

CREATE TABLE accounts (
	namespace_id    TEXT NOT NULL DEFAULT '',
	controller_name TEXT NOT NULL,
	details_json    TEXT NOT NULL,
	PRIMARY KEY (namespace_id, controller_name)
);

CREATE TABLE credentials (
	namespace_id TEXT NOT NULL DEFAULT '',
	cloud_name   TEXT NOT NULL,
	details_json TEXT NOT NULL,
	PRIMARY KEY (namespace_id, cloud_name)
);

CREATE TABLE bootstrap_config (
	namespace_id    TEXT NOT NULL DEFAULT '',
	controller_name TEXT NOT NULL,
	config_json     TEXT NOT NULL,
	PRIMARY KEY (namespace_id, controller_name)
);

CREATE TABLE cookie_jars (
	namespace_id    TEXT NOT NULL DEFAULT '',
	controller_name TEXT NOT NULL,
	cookies_json    TEXT NOT NULL,
	PRIMARY KEY (namespace_id, controller_name)
);

CREATE TABLE controller_access (
	namespace_id    TEXT NOT NULL,
	controller_name TEXT NOT NULL,
	user_id         TEXT NOT NULL,
	access          TEXT NOT NULL,
	PRIMARY KEY (namespace_id, controller_name, user_id)
);
`

// v2DropTables lists the tables that must be dropped before recreating under
// the v2 schema. Only resource tables change — users/keys are preserved.
var v2DropTables = []string{
	"controllers",
	"controller_meta",
	"models",
	"model_meta",
	"accounts",
	"credentials",
	"bootstrap_config",
	"cookie_jars",
}

// InitDB applies the schema and runs any required migrations.
func InitDB(db *sql.DB) error {
	// Ensure the version table always exists.
	if _, err := db.Exec(schemaVersionTable); err != nil {
		return err
	}

	// Read the stored version (0 if none yet).
	var version int
	_ = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)

	if version < schemaVersion {
		// Drop resource tables that need to be recreated (data loss is
		// acceptable — this is a dev-phase tool and data comes from bootstrap).
		for _, t := range v2DropTables {
			if _, err := db.Exec(`DROP TABLE IF EXISTS ` + t); err != nil {
				return err
			}
		}
		// Also drop old namespace tables in case of a partial migration.
		for _, t := range []string{"namespaces", "namespace_members", "controller_access"} {
			if _, err := db.Exec(`DROP TABLE IF EXISTS ` + t); err != nil {
				return err
			}
		}

		stmts := strings.Split(v2Schema, ";")
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}

		if _, err := db.Exec(`INSERT OR REPLACE INTO schema_version VALUES (?)`, schemaVersion); err != nil {
			return err
		}
	}

	// Always create version-independent tables.
	if _, err := db.Exec(alwaysCreate); err != nil {
		return err
	}

	return nil
}
