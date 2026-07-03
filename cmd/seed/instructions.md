# Seed commands (development only)

These commands populate the **development** database with example data (a dev user, sources,
articles, feeds and their associations) without waiting for the discovery CRON. They are meant for
local development and are **refused unless `ENVIRONMENT=development`**. The whole `cmd/` folder is
removed by CI/CD in staging/production.

## How it works

- A single binary (`cmd/seed`) with a subcommand. The Taskfile in this folder (included/flattened by
  the root Taskfile) wraps each subcommand as a `task` target — run them from the project root.
- Data comes from `cmd/seed/examples.json` (`user`, `user_preferences`, `sources`, `feeds`,
  `articles`).
- Writes go through the same controllers the API uses, so the seed follows the exact same rules
  (keyword encoding, per-user feed limit, UUID v7, cascades, ...).
- The seed finds the project root (via `go.mod`), opens `db/news_feed.db` and runs migrations, so it
  works on a fresh database too.

## Commands

| Task (alias)                         | Subcommand        | What it does |
|--------------------------------------|-------------------|--------------|
| `task seed-user-dev` (`sud`)         | `user`            | Creates the dev user + preferences. **Idempotent.** |
| `task seed-dev-login` (`sdl`)        | `login`           | Prints a fresh `access_token` + `refresh_token` for the dev user. |
| `task seed-dev-sources` (`sds`)      | `sources`         | Creates the example sources. **Idempotent.** |
| `task seed-dev-articles` (`sda`)     | `articles`        | Creates the example articles. **Re-runnable** (random `url_original` each run). Requires sources. |
| `task seed-dev-feeds` (`sdf`)        | `feeds`           | Creates the example feeds for the dev user. **Idempotent.** Requires the dev user. |
| `task seed-dev-articles-feeds` (`sdaf`) | `articles-feeds` | Links every article to every dev-user feed (bypasses judgement). **Idempotent.** |
| `task seed-dev-full` (`sdfull`)      | `full`            | Runs all of the above in order, then a dev login. |

Idempotent commands skip records that already exist and report exactly what was created vs. skipped.

## Typical flow

```bash
# one shot: user → sources → feeds → articles → articles-feeds → login (prints a token)
task sdfull

# or step by step
task sud     # dev user
task sds     # sources
task sdf     # feeds
task sda     # articles (re-run to add more)
task sdaf    # link articles to feeds
task sdl     # grab a fresh access_token
```

## Logging in the dev user

Two equivalent ways to obtain a token for the dev user (no Google OAuth):
- **CLI:** `task sdl` — prints the `access_token` and `refresh_token`.
- **HTTP:** `POST /v1/users/dev-login` — same result, for a front-end "dev login" button. This route
  is registered **only when `ENVIRONMENT=development`**, so it does not exist in staging/production.

## Notes

- Requires the JWT env vars (`JWT_SECRET_KEY`, `JWT_ACCESS_TOKEN_EXPIRY_MINUTES`,
  `JWT_REFRESH_TOKEN_EXPIRY_DAYS`) for the login step — the same ones the API uses (loaded from
  `.env`).
- `examples.json` preference values are validated against the enums; a typo (e.g. an unknown
  `ai_personality`) fails loudly instead of persisting garbage.
- Editing anything under `cmd/` should keep these commands and this file in sync (see CLAUDE.md).
