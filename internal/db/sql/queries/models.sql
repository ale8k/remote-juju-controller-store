-- name: ListModelsByController :many
SELECT model_name, details_json
FROM models
WHERE namespace_id = ? AND controller_name = ?
ORDER BY model_name ASC;

-- name: ReplaceModelsByController :exec
DELETE FROM models
WHERE namespace_id = ? AND controller_name = ?;

-- name: CreateModel :exec
INSERT INTO models(namespace_id, controller_name, model_name, details_json)
VALUES (?, ?, ?, ?);

-- name: UpsertModel :exec
INSERT INTO models(namespace_id, controller_name, model_name, details_json)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace_id, controller_name, model_name) DO UPDATE
SET details_json = excluded.details_json;

-- name: GetModelByName :one
SELECT details_json
FROM models
WHERE namespace_id = ? AND controller_name = ? AND model_name = ?;

-- name: GetModelNameBySuffix :one
SELECT model_name
FROM models
WHERE namespace_id = ? AND controller_name = ? AND model_name LIKE ?;

-- name: GetModelBySuffix :one
SELECT details_json
FROM models
WHERE namespace_id = ? AND controller_name = ? AND model_name LIKE ?;

-- name: CountModelByName :one
SELECT COUNT(*)
FROM models
WHERE namespace_id = ? AND controller_name = ? AND model_name = ?;

-- name: DeleteModelByName :exec
DELETE FROM models
WHERE namespace_id = ? AND controller_name = ? AND model_name = ?;

-- name: DeleteModelsByController :exec
DELETE FROM models
WHERE namespace_id = ? AND controller_name = ?;

-- name: DeleteModelsByNamespace :exec
DELETE FROM models
WHERE namespace_id = ?;

-- name: SetModelMeta :exec
INSERT INTO model_meta(namespace_id, controller_name, key, value)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace_id, controller_name, key) DO UPDATE SET value = excluded.value;

-- name: GetModelMeta :one
SELECT value
FROM model_meta
WHERE namespace_id = ? AND controller_name = ? AND key = ?;

-- name: DeleteModelMetaByController :exec
DELETE FROM model_meta
WHERE namespace_id = ? AND controller_name = ?;

-- name: DeleteModelMetaByNamespace :exec
DELETE FROM model_meta
WHERE namespace_id = ?;
