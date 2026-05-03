-- name: ListControllersByNamespace :many
SELECT name, details_json
FROM controllers
WHERE namespace_id = ?
ORDER BY name ASC;

-- name: GetControllerByName :one
SELECT details_json
FROM controllers
WHERE namespace_id = ? AND name = ?;

-- name: CountControllerByName :one
SELECT COUNT(*)
FROM controllers
WHERE namespace_id = ? AND name = ?;

-- name: FindControllerByEndpoints :many
SELECT name, details_json
FROM controllers
WHERE namespace_id = ?;

-- name: CreateController :exec
INSERT INTO controllers(namespace_id, name, details_json)
VALUES (?, ?, ?);

-- name: UpdateController :exec
UPDATE controllers
SET details_json = ?
WHERE namespace_id = ? AND name = ?;

-- name: DeleteController :exec
DELETE FROM controllers
WHERE namespace_id = ? AND name = ?;

-- name: SetControllerMeta :exec
INSERT INTO controller_meta(namespace_id, key, value)
VALUES (?, ?, ?)
ON CONFLICT(namespace_id, key) DO UPDATE SET value = excluded.value;

-- name: GetControllerMeta :one
SELECT value
FROM controller_meta
WHERE namespace_id = ? AND key = ?;

-- name: DeleteControllerSelectionMeta :exec
DELETE FROM controller_meta
WHERE namespace_id = ? AND key IN ('current', 'previous');

-- name: DeleteControllerMetaByNamespace :exec
DELETE FROM controller_meta
WHERE namespace_id = ?;

-- name: SetControllerAccess :exec
INSERT INTO controller_access(namespace_id, controller_name, user_id, access)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace_id, controller_name, user_id) DO UPDATE SET access = excluded.access;

-- name: GetControllerAccess :one
SELECT access
FROM controller_access
WHERE namespace_id = ? AND controller_name = ? AND user_id = ?;

-- name: DeleteControllerAccessByController :exec
DELETE FROM controller_access
WHERE namespace_id = ? AND controller_name = ?;

-- name: DeleteControllerAccessByNamespace :exec
DELETE FROM controller_access
WHERE namespace_id = ?;

-- name: MigrateControllerAccessUserID :exec
INSERT INTO controller_access(namespace_id, controller_name, user_id, access)
SELECT ca.namespace_id, ca.controller_name, ?, ca.access
FROM controller_access AS ca
WHERE ca.user_id = ?
ON CONFLICT(namespace_id, controller_name, user_id) DO NOTHING;

-- name: DeleteControllerAccessByUserID :exec
DELETE FROM controller_access
WHERE user_id = ?;
