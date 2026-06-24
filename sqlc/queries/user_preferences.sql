-- name: CreateUserPreferences :one
INSERT INTO user_preferences (id, user_id, theme, language, translate_content, ai_personality)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: FindUserPreferencesByUserID :one
SELECT * FROM user_preferences
WHERE user_id = ? AND status = 1 AND removed_at IS NULL
LIMIT 1;

-- name: UpdateUserPreferences :one
UPDATE user_preferences
SET theme = ?, language = ?, translate_content = ?, ai_personality = ?,
    modified_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND status = 1 AND removed_at IS NULL
RETURNING *;

-- name: SoftDeleteUserPreferences :exec
UPDATE user_preferences
SET status = 0, removed_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND removed_at IS NULL;
