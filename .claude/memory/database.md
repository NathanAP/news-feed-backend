# Banco de dados

> Resumo. Regras completas em `.claude/rules/conventions.md` (seção banco de dados) e
> `.claude/PROJECT.md` (status, exclusão, cascades). Schema-fonte em `schema.sql`.

## Convenções transversais

- `id` é **UUID v7** (imutável). Exceção histórica: o backfill de `user_preferences` usa
  UUID v4 (sem função nativa no SQLite), comentado na migração.
- Tabelas comuns têm `id`, `status` (INTEGER 0/1), `created_at`, `modified_at`, `removed_at`.
- **Soft delete**: `status = 0` + `removed_at` preenchido. Junction tables são exceção:
  só têm `id` (+ FKs e campos extras), e sofrem **hard delete**.
- **Predicado de leitura ativo** (em toda query de busca/listagem/update):
  `status = 1 AND removed_at IS NULL`.
- **Unicidade só entre ativos**: implementada como **índice único parcial**
  `WHERE removed_at IS NULL` (nunca constraint UNIQUE total). Exceção atual: o domínio de
  usuário (`users.google_id`, `users.email`, `user_preferences.user_id`) ainda usa UNIQUE
  total — ver nota em PROJECT.md ("Usuário"); revisitar com a futura exclusão de usuário.
- **Junction válida** só quando **todos** os lados estão ativos → as queries fazem JOIN e
  validam `status`/`removed_at` de cada relacionado.

## Tabelas

### users
`id`, `google_id` (UNIQUE), `email` (UNIQUE), `name`, `picture` (nullable), `status`,
`last_login_at` (nullable), `created_at`, `modified_at`, `removed_at`.

### user_preferences (1:1 com users)
`id`, `user_id` (UNIQUE, FK users), `status`, `theme` (`light`|`dark`),
`language` (`pt`|`en`|`es`|...), `translate_content` (0/1), `ai_personality`
(`fun`|`informative`|`mixed`), timestamps. Defaults: dark, pt, translate=1, mixed.

### refresh_tokens (N:1 com users)
`id`, `user_id` (FK users), `status`, `expires_at`, timestamps. O `id` do token É o valor
usado no fluxo de refresh.

### sources (globais)
`id`, `status`, `url`, `url_rss`, timestamps. Índices únicos parciais em `url` e `url_rss`.

### articles (globais)
`id`, `status`, `title`, `content` (markdown), `url_original`, `keywords` (JSON array TEXT),
`source_id` (**NOT NULL**, FK sources, **imutável**), timestamps. Índice único parcial em
`url_original`. Keywords: 5–20 itens.

### feeds (por usuário)
`id`, `status`, `name`, `keywords` (JSON array TEXT, 5–20), `user_id` (FK users), timestamps.
Sem unicidade (nomes duplicados permitidos). Máx. **5 feeds ativos** por usuário.

### articles_feeds (junction feed × notícia)
`id`, `article_id` (FK articles), `feed_id` (FK feeds), `is_read` (0/1), `created_at`,
`modified_at`. **Sem** `status`/`removed_at`. `user_id` é obtido via JOIN com `feeds` (sem
desnormalização). Hard delete quando um pai sofre hard remove; no soft remove de um pai o
registro **permanece** mas fica invisível (filtrado pelo JOIN de ativos).

## Cascades (executados nos controllers)

- `users` soft remove → soft remove de `user_preferences`, `refresh_tokens`, `feeds`.
- `sources` soft remove → soft remove de `articles` (da fonte).
- Hard remove dos pais → hard remove dos registros de junction relacionados (futuro; sem
  gatilho hoje, pois não há rotas de hard delete).
