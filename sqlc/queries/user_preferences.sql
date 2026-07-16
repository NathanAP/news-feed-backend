-- name: CreateUserPreferences :one
INSERT INTO user_preferences (id, user_id, language_to_translate, ai_personality)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: FindUserPreferencesByUserID :one
SELECT * FROM user_preferences
WHERE user_id = $1 AND status = TRUE AND removed_at IS NULL
LIMIT 1;

-- name: UpdateUserPreferences :one
UPDATE user_preferences
SET language_to_translate = $1, ai_personality = $2,
    modified_at = CURRENT_TIMESTAMP
WHERE user_id = $3 AND status = TRUE AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteUserPreferences :exec
UPDATE user_preferences
SET status = FALSE, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND removed_at IS NULL;
