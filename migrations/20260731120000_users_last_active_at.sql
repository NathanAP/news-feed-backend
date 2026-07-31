-- +goose Up
-- 0.43: activity signal for the discovery filter. `last_active_at` is what tells the judgement layer 1
-- whether a user is still around, so the CRON can stop routing news to abandoned accounts.
--
-- It is NOT the same as `last_login_at`: last_login_at only changes on a full Google login (which
-- happens at most every refresh-token window, ~30 days), so it would flag a daily-active user as
-- inactive. last_active_at is written on every /auth/refresh (the hourly heartbeat) and on login, so a
-- user active within the last few days always has a fresh value.
--
-- NOT NULL DEFAULT CURRENT_TIMESTAMP: adding the column fills every existing row with the migration
-- time, so nobody is treated as inactive right after the deploy (that would freeze their feed until
-- they next refreshed). New rows default to now() as well, because a just-created user has just logged
-- in — they are active by definition.
ALTER TABLE users ADD COLUMN last_active_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- +goose Down
ALTER TABLE users DROP COLUMN last_active_at;
