-- name: GetSystem :one
SELECT * FROM system
LIMIT 1;

-- name: UpdateSystemAppStatus :one
UPDATE system
SET app_status = ?, modified_at = CURRENT_TIMESTAMP
RETURNING *;
