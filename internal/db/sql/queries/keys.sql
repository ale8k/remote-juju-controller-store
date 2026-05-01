-- name: CreateSigningKey :exec
INSERT INTO signing_keys(id, key_pem)
VALUES (?, ?);

-- name: RetireSigningKey :exec
UPDATE signing_keys
SET retired_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListActiveSigningKeys :many
SELECT id, key_pem, created_at, retired_at
FROM signing_keys
WHERE retired_at IS NULL
ORDER BY created_at DESC;

-- name: ListSigningKeys :many
SELECT id, key_pem, created_at, retired_at
FROM signing_keys
ORDER BY created_at DESC;

-- name: CreateControllerSigningKey :exec
INSERT INTO controller_signing_keys(id, key_pem)
VALUES (?, ?);

-- name: RetireControllerSigningKey :exec
UPDATE controller_signing_keys
SET retired_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListActiveControllerSigningKeys :many
SELECT id, key_pem, created_at, retired_at
FROM controller_signing_keys
WHERE retired_at IS NULL
ORDER BY created_at DESC;

-- name: ListControllerSigningKeys :many
SELECT id, key_pem, created_at, retired_at
FROM controller_signing_keys
ORDER BY created_at DESC;
