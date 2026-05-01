-- name: GetAccountByController :one
SELECT details_json
FROM accounts
WHERE namespace_id = ? AND controller_name = ?;

-- name: UpsertAccountByController :exec
INSERT INTO accounts(namespace_id, controller_name, details_json)
VALUES (?, ?, ?)
ON CONFLICT(namespace_id, controller_name) DO UPDATE
SET details_json = excluded.details_json;

-- name: DeleteAccountByController :exec
DELETE FROM accounts
WHERE namespace_id = ? AND controller_name = ?;

-- name: DeleteAccountsByNamespace :exec
DELETE FROM accounts
WHERE namespace_id = ?;

-- name: ListCredentialsByNamespace :many
SELECT cloud_name, details_json
FROM credentials
WHERE namespace_id = ?
ORDER BY cloud_name ASC;

-- name: GetCredentialByCloud :one
SELECT details_json
FROM credentials
WHERE namespace_id = ? AND cloud_name = ?;

-- name: UpsertCredentialByCloud :exec
INSERT INTO credentials(namespace_id, cloud_name, details_json)
VALUES (?, ?, ?)
ON CONFLICT(namespace_id, cloud_name) DO UPDATE
SET details_json = excluded.details_json;

-- name: DeleteCredentialsByNamespace :exec
DELETE FROM credentials
WHERE namespace_id = ?;

-- name: GetBootstrapConfigByController :one
SELECT config_json
FROM bootstrap_config
WHERE namespace_id = ? AND controller_name = ?;

-- name: UpsertBootstrapConfigByController :exec
INSERT INTO bootstrap_config(namespace_id, controller_name, config_json)
VALUES (?, ?, ?)
ON CONFLICT(namespace_id, controller_name) DO UPDATE
SET config_json = excluded.config_json;

-- name: DeleteBootstrapConfigByController :exec
DELETE FROM bootstrap_config
WHERE namespace_id = ? AND controller_name = ?;

-- name: DeleteBootstrapConfigByNamespace :exec
DELETE FROM bootstrap_config
WHERE namespace_id = ?;

-- name: GetCookiesByController :one
SELECT cookies_json
FROM cookie_jars
WHERE namespace_id = ? AND controller_name = ?;

-- name: UpsertCookiesByController :exec
INSERT INTO cookie_jars(namespace_id, controller_name, cookies_json)
VALUES (?, ?, ?)
ON CONFLICT(namespace_id, controller_name) DO UPDATE
SET cookies_json = excluded.cookies_json;

-- name: DeleteCookiesByController :exec
DELETE FROM cookie_jars
WHERE namespace_id = ? AND controller_name = ?;

-- name: DeleteCookiesByNamespace :exec
DELETE FROM cookie_jars
WHERE namespace_id = ?;

-- name: DeleteControllersByNamespace :exec
DELETE FROM controllers
WHERE namespace_id = ?;
