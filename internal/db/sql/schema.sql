CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS signing_keys (
    id TEXT PRIMARY KEY,
    key_pem TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    retired_at DATETIME
);

CREATE TABLE IF NOT EXISTS controller_signing_keys (
    id TEXT PRIMARY KEY,
    key_pem TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    retired_at DATETIME
);

CREATE TABLE IF NOT EXISTS namespaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    owner_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS namespace_members (
    namespace_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    PRIMARY KEY (namespace_id, user_id)
);

CREATE TABLE IF NOT EXISTS controllers (
    namespace_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    details_json TEXT NOT NULL,
    PRIMARY KEY (namespace_id, name)
);

CREATE TABLE IF NOT EXISTS controller_meta (
    namespace_id TEXT NOT NULL DEFAULT '',
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (namespace_id, key)
);

CREATE TABLE IF NOT EXISTS models (
    namespace_id TEXT NOT NULL DEFAULT '',
    controller_name TEXT NOT NULL,
    model_name TEXT NOT NULL,
    details_json TEXT NOT NULL,
    PRIMARY KEY (namespace_id, controller_name, model_name)
);

CREATE TABLE IF NOT EXISTS model_meta (
    namespace_id TEXT NOT NULL DEFAULT '',
    controller_name TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (namespace_id, controller_name, key)
);

CREATE TABLE IF NOT EXISTS accounts (
    namespace_id TEXT NOT NULL DEFAULT '',
    controller_name TEXT NOT NULL,
    details_json TEXT NOT NULL,
    PRIMARY KEY (namespace_id, controller_name)
);

CREATE TABLE IF NOT EXISTS credentials (
    namespace_id TEXT NOT NULL DEFAULT '',
    cloud_name TEXT NOT NULL,
    details_json TEXT NOT NULL,
    PRIMARY KEY (namespace_id, cloud_name)
);

CREATE TABLE IF NOT EXISTS bootstrap_config (
    namespace_id TEXT NOT NULL DEFAULT '',
    controller_name TEXT NOT NULL,
    config_json TEXT NOT NULL,
    PRIMARY KEY (namespace_id, controller_name)
);

CREATE TABLE IF NOT EXISTS cookie_jars (
    namespace_id TEXT NOT NULL DEFAULT '',
    controller_name TEXT NOT NULL,
    cookies_json TEXT NOT NULL,
    PRIMARY KEY (namespace_id, controller_name)
);

CREATE TABLE IF NOT EXISTS controller_access (
    namespace_id TEXT NOT NULL,
    controller_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    access TEXT NOT NULL,
    PRIMARY KEY (namespace_id, controller_name, user_id)
);
