-- name: UpsertUser :exec
INSERT INTO users(id, email)
VALUES (?, ?)
ON CONFLICT(id) DO UPDATE SET email = excluded.email;

-- name: GetUserEmailByID :one
SELECT email
FROM users
WHERE id = ?;

-- name: GetUserIDByEmail :one
SELECT id
FROM users
WHERE email = ?
ORDER BY created_at ASC
LIMIT 1;
