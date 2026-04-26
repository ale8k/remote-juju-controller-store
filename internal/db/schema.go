package db

import "database/sql"

const Schema = `
CREATE TABLE IF NOT EXISTS controllers (
	name         TEXT PRIMARY KEY,
	details_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS controller_meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS models (
	controller_name TEXT NOT NULL,
	model_name      TEXT NOT NULL,
	details_json    TEXT NOT NULL,
	PRIMARY KEY (controller_name, model_name)
);

CREATE TABLE IF NOT EXISTS model_meta (
	controller_name TEXT NOT NULL,
	key             TEXT NOT NULL,
	value           TEXT NOT NULL,
	PRIMARY KEY (controller_name, key)
);

CREATE TABLE IF NOT EXISTS accounts (
	controller_name TEXT PRIMARY KEY,
	details_json    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS credentials (
	cloud_name   TEXT PRIMARY KEY,
	details_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bootstrap_config (
	controller_name TEXT PRIMARY KEY,
	config_json     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cookie_jars (
	controller_name TEXT PRIMARY KEY,
	cookies_json    TEXT NOT NULL
);

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

// InitDB applies the schema to the database.
func InitDB(db *sql.DB) error {
	_, err := db.Exec(Schema)
	return err
}
