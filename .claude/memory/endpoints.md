# Endpoints da API

> Resumo para consumo (inclusive por outras instâncias, ex: o client). Base: `/{API_VERSION}`
> (hoje `/v1`). Auth = `Authorization: Bearer <access_token>`. Datas sempre em UTC.
> Convenção de "não encontrado": busca em lista vazia → 200 `[]`; busca de item único
> inexistente → 404.

## Guard de manutenção (global)
- Middleware global lê `system.app_status` a cada request. Quando `false`, **toda rota retorna
  503** — exceto as duas isentas: `GET /v1/health` e `PUT /v1/system/app-status` (registradas
  antes do guard). O guard roda **antes da auth**, então em manutenção uma rota protegida sem
  token responde 503 (não 401). Sem cache: o toggle reflete no próximo request.

## Health
- `GET /v1/health` — **sem auth**, isenta do guard. Retorna `{ status, version, app_status,
  server_time }` (`app_status` = estado global; `server_time` = hora do servidor em UTC).

## System (`/v1/system`)
- `PUT /system/app-status` — **aberta** (admin-futuro), isenta do guard. Body
  `{ app_status: bool }` (obrigatório) → 200 `SystemResponse` / 400 (campo ausente) / 500.
  Liga/desliga a aplicação globalmente; isenta do guard para nunca trancar o religamento.

## Auth (`/v1/auth`)
- `GET /auth/google?redirect_uri=` — **sem auth**. Valida o `redirect_uri` contra a allowlist
  (`OAUTH_ALLOWED_REDIRECT_URIS`, match exato; 400 se ausente/fora dela) e redireciona ao Google com um
  `state` assinado (CSRF + carrega o `redirect_uri` pela ida-e-volta).
- `GET /auth/google/callback` — **sem auth**. Valida o `state` (401 se inválido/expirado), troca o
  `code` e **redireciona ao `redirect_uri`** com os tokens no **fragment**
  (`#access_token=...&refresh_token=...&expires_in=...`) — não devolve JSON. Cancelamento no Google →
  redirect com `?error=`. Troca de `code` falha → 401. (fluxo completo em `PROJECT.md`)
- `POST /auth/refresh` — **sem auth**. Body `{ refresh_token }`. Renova o `access_token` e
  estende o `refresh_token`. 401 se inválido/expirado.
- `POST /auth/logout` — **auth**. Soft-remove do refresh token atual. 204.
- `DELETE /auth/invalidate` — **aberta** (admin-futuro). Body `{ refresh_token }`. Derruba uma sessão.
- `DELETE /auth/invalidate-all` — **aberta** (admin-futuro). Query `user_id`. Derruba todas as sessões.

## Users (`/v1/users`) — com auth (exceto dev-login)
- `GET /users/me` — dados do usuário (lidos do JWT).
- `GET /users/me/preferences` — preferências (lidas do JWT).
- `PUT /users/me/preferences` — atualiza preferências. Retorna `{ access_token, expires_in, preferences }`
  (novo token já com as preferências atualizadas).
- `POST /users/dev-login` — **exclusivo de desenvolvimento** (registrado só quando `ENVIRONMENT=development`;
  não existe em staging/prod). **Sem auth.** Loga o usuário dev semeado (`task sud`) sem Google OAuth,
  emitindo `{ access_token, refresh_token, expires_in }`. 404 se o usuário dev não foi semeado. Equivalente
  ao `task sdl` no CLI.

## Sources (`/v1/sources`) — auth (semântica admin; hoje aberta a qualquer autenticado)
- `POST /sources/create` — `{ url, url_rss }` → 201 / 400 / 409 (url duplicada entre ativas) / 500.
- `GET /sources/rss-discovery?url=` — descobre feeds RSS da URL → 200 `{ feeds: [...] }` (lista pode ser vazia) / 400.
- `GET /sources/:id/article-discovery` — **dry-run** da descoberta de notícias de 1 source (espelha a CRON: parsing RSS + dedup por `url_original`). Não grava nada. Query opcional `last_article_discovery_at` (RFC3339 UTC) adiciona limite inferior por data. → 200 `{ articles: [...] }` (pode ser vazia) / 400 (data inválida) / 404 (source inexistente). Aberta (admin-futuro).
- `GET /sources/:id` → 200 / 404.
- `GET /sources?url=` — filtro por substring na url → 200 (paginada, ver "Paginação").
- `PUT /sources/:id` — `{ url, url_rss }` → 200 / 400 / 404 / 409.
- `DELETE /sources/:id` — soft delete → 204 / 404. Cascata: soft-remove das `articles` da fonte.

## Articles (`/v1/articles`) — auth (criação/edição/remoção = admin-futuro; hoje abertas)
- `POST /articles/create` — `{ title, content, url_original, keywords[5..20], source_id, language_original }` →
  201 / 400 (inclui `source_id` ausente/fonte inativa e `language_original` ausente/inválido) / 409 (url_original duplicada) / 500.
  `language_original` é um código do enum de idiomas (`pt|en|es|fr|de|it`) e é obrigatório na criação manual (na CRON é detectado).
- `POST /articles/treatment` — **dry-run** do tratamento por IA. Body `{ article: { title, content, ... }, keywords_mode? }`. Roda detecção de idioma (lingua-go) + tratamento (LLM) → keywords, e retorna `{ content, keywords, keywords_mode, language_original, treatment_ms, keywords_ms }`. **Não persiste**, mas **chama a IA de verdade** (consome quota). O `keywords_mode` opcional (`local`|`groq`|`gemini`) troca o backend das keywords só nesta chamada (benchmark sem reiniciar; 400 se o modo não existe). → 200 / 400 / 500. Aberta (admin-futuro).
- `GET /articles/:id/translate/:language` — **tradução personalizada** sob demanda (LLM apenas, `TRANSLATION_*`). Traduz título+conteúdo para `:language` (`pt|en|es|fr|de|it`), preservando o HTML e adaptando o tom à `ai_personality` (lida do JWT); re-sanitiza a saída (bluemonday). **Keywords não são traduzidas** (ficam canônicas em inglês). **Read-only** (não grava; client cacheia). → 200 `{ title, content, language, language_original }` / 403 (`translate_content` off) / 400 (idioma inválido, igual ao original, ou `language_original` null) / 404 / 500. Aberta.
- `POST /articles/judgement` — **dry-run** do julgamento por IA. Body `{ article: { title, content, keywords }, judgement_mode? }` (notícia já tratada; sem `id`). Camada 1: feeds candidatos por sobreposição de keywords (SQL `json_each`, feeds ativos de qualquer usuário); camada 2: `score` 0–100 da IA por candidato vs `JUDGEMENT_THRESHOLD`. Retorna `{ judgement_mode, threshold, candidate_count, judgements: [{ feed_id, feed_name, score, passed }], judgement_ms }`. **Não grava** (para na penúltima etapa), mas **chama a IA de verdade**. `judgement_mode` opcional (`local`|`groq`|`gemini`) troca o backend só nesta chamada (400 se inexistente). → 200 / 400 / 500. Aberta (admin-futuro).
- `PUT /articles/:id/read` — marca como lida nos feeds do usuário → **200** (em ≥1 feed, ou já
  lida) / **204** (não está em nenhum feed do usuário) / 404 (notícia inexistente). Idempotente.
- `GET /articles/:id` → 200 / 404. Resposta enriquecida com `is_read`: `null` (não está em
  feed do usuário) | `false` (em ≥1 feed, ao menos um não lido) | `true` (todos lidos).
- `GET /articles?url=` — filtro por substring em url_original → 200 (paginada, ver "Paginação").
- `PUT /articles/:id` — `{ title, content, url_original, keywords, language_original }` (sem `source_id`, imutável) → 200 / 400 / 404 / 409.
- `DELETE /articles/:id` — soft delete → 204 / 404.

## Feeds (`/v1/feeds`) — auth, **recurso por-usuário**
- Acesso restrito ao dono: feed de outro usuário responde **404** (não 403), sem vazar existência.
- `POST /feeds/create` — `{ name (≤120), keywords[5..20] }` → 201 / 400 / 409 (limite de 5 feeds ativos) / 500.
- `GET /feeds/:id` → 200 / 404 (inexistente ou de outro usuário).
- `GET /feeds/:id/articles` — as notícias que caíram no feed (do dono), cada uma com `is_read` (aqui
  sempre definido — a notícia está no feed). Filtros opcionais: `is_read` (`true`|`false`) e janela
  `period_starting_at`/`period_ending_at` (RFC3339 UTC, sobre `created_at`, inclusivos). → 200
  (paginada, ver "Paginação") / 404 (feed inexistente ou de outro usuário) / 400 (filtro inválido).
- `GET /feeds?name=` — só os próprios feeds, filtro por substring no nome → 200 (paginada, ver "Paginação").
- `PUT /feeds/:id` — `{ name, keywords }` (sem `user_id`, imutável) → 200 / 400 / 404.
- `DELETE /feeds/:id` — soft delete (permanente, sem reativação) → 204 / 404.

## Descoberta automática (CRON, sem rota)
- CRON interna (`services/cron`, `robfig/cron/v3`) varre as sources ativas em `RSS_FEED_CRON_SCHEDULE`,
  ativa por `RSS_FEED_CRON_ACTIVE`. Lê o RSS de cada source (gofeed), **deduplica por `url_original`**,
  **trata** as novas (detecta o idioma com lingua-go + LLM limpa o conteúdo + SLM nomeia keywords),
  **persiste** o `article` (com `language_original`) e por fim **julga** (camada 1 SQL por keywords + camada 2 IA vs
  `JUDGEMENT_THRESHOLD`), gravando as associações aprovadas em `articles_feeds`; ao
  final grava `system.last_article_discovery_at` (informativo). Falha de IA no tratamento → não
  persiste, re-tenta na próxima run; falha no julgamento é best-effort (a notícia já está persistida,
  não é re-julgada — não-retroativo). Pula a run quando `app_status` está off. Logs gated por
  `RSS_FEED_CRON_VERBOSE_MODE`.

## Paginação
- Toda rota de busca de coleção (`GET /v1/articles`, `/v1/sources`, `/v1/feeds`, `/v1/feeds/:id/articles`) é **paginada**.
  Query: `?page=` (mín/padrão 1) e `?page_size=` (mín 1, máx 100, padrão 20). Resposta:
  `{ docs: [...], pagination: { actual_page, total_pages, actual_count, total_count, has_next_page,
  has_previous_page } }`. Página fora do range → `docs` vazio (sem erro), `actual_page` fica no valor
  pedido. Helper global em `services/pagination` (paginação em memória sobre a lista já filtrada).

## Notas
- Todo endpoint dispara log de início/fim quando `VERBOSE_MODE=true`.
- **CORS**: origens liberadas via `CORS_ALLOWED_ORIGINS` (lista separada por vírgula, origins
  completas com esquema). Sem cookie/credentials (auth é Bearer). Vazio = nenhuma origem cross-origin
  liberada (fail-closed).
- A coleção Bruno em `bruno/` espelha todas essas rotas.
