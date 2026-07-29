-- +goose Up
-- 0.40: administrator mode. The `admin` flag is what separates a regular user from an administrator;
-- PROJECT.md already described the behaviour, this is the column that finally backs it.
--
-- Two deliberate choices:
--   * NOT NULL DEFAULT FALSE: fail-closed. Every row that already exists becomes a regular user, and
--     so does every user created from here on (CreateUser does not write this column at all, so the
--     Google login flow is structurally incapable of minting an administrator).
--   * No index. The flag is only ever read by primary key (the middleware looks the requester up by
--     id), never filtered on, so an index would be dead weight — the same mistake the 0.37 GIN index
--     made before 0.37.3 caught it.
--
-- Promoting someone is a manual database change for now (PROJECT.md, "Administradores"); in
-- development `task sud` marks the seeded dev user.
ALTER TABLE users ADD COLUMN admin BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE users DROP COLUMN admin;
