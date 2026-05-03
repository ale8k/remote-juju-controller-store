-- name: CountNamespacesByName :one
SELECT COUNT(*)
FROM namespaces
WHERE name = ?;

-- name: CreateNamespace :exec
INSERT INTO namespaces(id, name, owner_id)
VALUES (?, ?, ?);

-- name: GetNamespaceByName :one
SELECT id, name, owner_id, created_at
FROM namespaces
WHERE name = ?;

-- name: DeleteNamespaceByID :exec
DELETE FROM namespaces
WHERE id = ?;

-- name: ListNamespacesForUser :many
SELECT n.id, n.name, n.owner_id, n.created_at
FROM namespaces n
JOIN namespace_members m ON m.namespace_id = n.id
WHERE m.user_id = ?
ORDER BY n.name ASC;

-- name: GetNamespaceMembershipID :one
SELECT n.id
FROM namespaces n
JOIN namespace_members m ON m.namespace_id = n.id
WHERE n.name = ? AND m.user_id = ?;

-- name: AddNamespaceMember :exec
INSERT INTO namespace_members(namespace_id, user_id)
VALUES (?, ?)
ON CONFLICT(namespace_id, user_id) DO NOTHING;

-- name: RemoveNamespaceMember :exec
DELETE FROM namespace_members
WHERE namespace_id = ? AND user_id = ?;

-- name: ListNamespaceMemberEmails :many
SELECT u.email
FROM users u
JOIN namespace_members m ON m.user_id = u.id
WHERE m.namespace_id = ?
ORDER BY u.email ASC;

-- name: DeleteNamespaceMembersByNamespaceID :exec
DELETE FROM namespace_members
WHERE namespace_id = ?;

-- name: MigrateNamespaceMembersUserID :exec
INSERT INTO namespace_members(namespace_id, user_id)
SELECT nm.namespace_id, ?
FROM namespace_members AS nm
WHERE nm.user_id = ?
ON CONFLICT(namespace_id, user_id) DO NOTHING;

-- name: DeleteNamespaceMembersByUserID :exec
DELETE FROM namespace_members
WHERE user_id = ?;

-- name: ReassignNamespaceOwnership :exec
UPDATE namespaces
SET owner_id = ?
WHERE owner_id = ?;
