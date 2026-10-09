-- name: CreateEntry :one
INSERT OR IGNORE INTO entries (domain, username, encrypted_password, created_at, updated_at)
VALUES (
    ?1,
    ?2,
    ?3,
	?4,
    ?5
)
RETURNING id, domain, username, encrypted_password, created_at, updated_at;

-- name: GetEntry :one
SELECT *
FROM entries
WHERE domain = ?1 AND username = ?2;

-- name: DeleteEntry :one
DELETE FROM entries
WHERE domain = ?1 AND username = ?2
RETURNING *;

-- name: ListEntries :many
SELECT id, domain, username, encrypted_password, created_at, updated_at
FROM entries
ORDER BY domain ASC, username ASC;


-- name: RestoreEntry :one
INSERT INTO entries (id, domain, username, encrypted_password, created_at, updated_at)
VALUES (
    ?1,
    ?2,
    ?3,
	?4,
    ?5,
    ?6
)
RETURNING *;

-- name: UpdateEntry :one
UPDATE entries
SET encrypted_password = ?3, updated_at = ?4
WHERE domain = ?1 AND username = ?2
RETURNING *;
