# Endpoints da API

> Este arquivo está sendo enviado em conjunto com o `api-integration.md` ao client.
> Resumo para consumo (inclusive por outras instâncias, ex: o client). Base: `/{API_VERSION}`
> (hoje `/v1`). Auth = `Authorization: Bearer <access_token>`. Datas sempre em UTC.
> Convenção de "não encontrado": busca em lista vazia → 200 `[]`; busca de item único
> inexistente → 404.

## Formatos de resposta (DTOs)

Shapes de **sucesso** reutilizados pelas rotas abaixo (campo com `?` = opcional/pode ser `null`).
Todo **erro** responde `{ error: string }` com o status apropriado. Datas em UTC (ISO-8601).

- **AuthResponse** — `{ access_token, refresh_token, expires_in }` (`refresh_token` = **id** do token; `expires_in` em segundos).
- **UserResponse** — `{ id, email, name, picture?, created_at, admin }` (`admin` sempre presente, mesmo
  `false`; é dica de client — a autorização real lê o banco).
- **UserPreferencesResponse** — `{ language_to_translate?, ai_personality }` (`language_to_translate` anulável: null = tradução off, só dica de client).
- **UpdatePreferencesResponse** — `{ access_token, expires_in, preferences: UserPreferencesResponse }`.
- **SystemResponse** — `{ id, app_status, last_article_discovery_at?, created_at, modified_at? }`.
- **SourceResponse** — `{ id, status, name, url, url_rss, created_at, modified_at? }`.
- **ArticleResponse** — `{ id, status, title, content, url_original, keywords[], source_id, language_original?, created_at, modified_at?, is_read?, source? }` (`is_read`: `null`|`false`|`true`; `source` é `SourceResponse` — só aparece quando a rota o preenche, ver `GET /feeds/:id/articles`).
- **FeedResponse** — `{ id, status, name, keywords[], user_id, created_at, modified_at? }`.
- **Envelope paginado** — `{ docs: [T], pagination }` (ver "Paginação"); usado por toda rota de lista.
- Respostas específicas de dry-run/discovery/tradução estão descritas na própria rota.

## Guard de manutenção (global)

- Middleware global lê `system.app_status` a cada request. Quando `false`, **toda rota retorna
  503** — exceto as isentas, registradas antes do guard: `GET /v1/health`,
  `PUT /v1/system/app-status` e **todo o grupo `/v1/auth`**. O guard roda **antes da auth**, então em
  manutenção uma rota protegida sem token responde 503 (não 401). Sem cache: o toggle reflete no
  próximo request.
- **Administradores atravessam a manutenção** (0.40). A identificação é feita só no branch em que a
  app já está desligada — no caminho normal não há query a mais — e é fail-closed: token ausente/ruim,
  sessão encerrada ou erro de banco viram 503.
- A isenção do `/auth` evita um travamento: o bypass identifica o admin pelo `access_token` (expira em
  ~1h) e o `refresh_token` não carrega identidade legível pelo bypass. Com `/auth` atrás do guard, o
  admin perderia o acesso ao próprio `refresh` e não conseguiria religar a aplicação.

## Autorização de administrador (0.40)

- Rotas exclusivas: `POST|PUT|DELETE /articles`, `POST|PUT|DELETE /sources` +
  `GET /sources/:id/article-discovery`, `DELETE /auth/invalidate`, `DELETE /auth/invalidate-all`,
  `PUT /system/app-status`. Leitura de artigos e sources continua aberta a qualquer autenticado.
- Sem token → 401; usuário comum → **403**; falha ao ler a flag → **500** (nunca 403). O guard roda
  antes da validação de body, então body inválido de usuário comum ainda dá 403.
- A flag vem **do banco** (`users.admin`), não do claim — o claim é dica de client.

## Health

- `GET /v1/health` — **sem auth**, isenta do guard. → 200 `{ status, version, app_status, server_time }`
  (`app_status` = estado global; `server_time` = hora do servidor em UTC).

## System (`/v1/system`)

- `PUT /system/app-status` — **auth + admin**, isenta do guard. Body
  `{ app_status: bool }` (obrigatório) → 200 **SystemResponse** / 400 (campo ausente) / 401 / 403 / 500.
  Liga/desliga a aplicação globalmente; isenta do guard para nunca trancar o religamento.

## Auth (`/v1/auth`)

- `GET /auth/google?redirect_uri=` — **sem auth**. Valida o `redirect_uri` contra a allowlist
  (`OAUTH_ALLOWED_REDIRECT_URIS`, match exato; 400 se ausente/fora dela) e **redireciona** (307) ao Google
  com um `state` assinado (CSRF + carrega o `redirect_uri`). Sem body.
- `GET /auth/google/callback` — **sem auth**. Valida o `state` (401 se inválido/expirado), troca o
  `code` e **redireciona** (307) ao `redirect_uri` com os tokens no **fragment**
  (`#access_token=...&refresh_token=...&expires_in=...`) — **não devolve JSON**. Cancelamento no Google →
  redirect com `?error=`. Troca de `code` falha → 401. (fluxo completo em `PROJECT.md`)
- `POST /auth/refresh` — **sem auth**. Body `{ refresh_token }`. → 200 **AuthResponse** (renova o
  `access_token` e estende o `refresh_token`) / 400 (body ausente) / 401 (inválido/expirado).
- `POST /auth/logout` — **auth**. Soft-remove do refresh token atual. → 204 (sem body) / 500.
- `DELETE /auth/invalidate` — **auth + admin**. Body `{ refresh_token_id }`. Derruba uma sessão.
  → 204 (sem body) / 400 (`refresh_token_id` ausente) / 401 / 403 / 500.
  (o valor é o mesmo que o `refresh_token` do login: o token **é** o id da linha, um UUID v7)
- `DELETE /auth/invalidate-all` — **auth + admin**. Query `user_id`. Derruba todas as sessões
  do usuário. → 204 (sem body) / 400 (`user_id` ausente) / 401 / 403 / 500. Um usuário comum leva 403
  mesmo passando o próprio `user_id` — para encerrar a própria sessão existe `POST /auth/logout`.

## Users (`/v1/users`) — com auth (exceto dev-login)

- `GET /users/me` — dados do usuário (lidos do JWT). → 200 **UserResponse**.
- `GET /users/me/preferences` — preferências (lidas do JWT). → 200 **UserPreferencesResponse**.
- `PUT /users/me/preferences` — atualiza preferências. → 200 **UpdatePreferencesResponse**
  (o `access_token` já vem regerado com as preferências novas) / 400 / 500.
- `POST /users/dev-login` — **exclusivo de desenvolvimento** (registrado só quando `ENVIRONMENT=development`;
  não existe em staging/prod). **Sem auth.** Loga o usuário dev semeado (`task sud`) sem Google OAuth.
  → 200 **AuthResponse** / 404 (usuário dev não semeado). Equivalente ao `task sdl` no CLI.

## Sources (`/v1/sources`) — auth; escrita exige admin (403 para usuário comum)

- `POST /sources/create` — **admin**. `{ name (≤120), url, url_rss }` → 201 **SourceResponse** / 400 / 403 / 409 (url duplicada entre ativas) / 500.
- `GET /sources/rss-discovery?url=` — descobre feeds RSS da URL → 200 `{ feeds: [string] }` (lista pode ser vazia) / 400.
- `GET /sources/:id/article-discovery` — **dry-run** da descoberta de notícias de 1 source (espelha a CRON: parsing RSS + dedup por `url_original`). Não grava nada. Query opcional `last_article_discovery_at` (RFC3339 UTC) adiciona limite inferior por data. → 200 `{ articles: [{ title, content, url_original, published_at?, source_id }] }` (pode ser vazia) / 400 (data inválida) / 403 / 404 (source inexistente). **Admin**.
- `GET /sources/:id` → 200 **SourceResponse** / 404.
- `GET /sources?url=&name=` — filtros por substring (case-insensitive) na url e/ou no nome; os dois se
  combinam com AND → 200 **envelope paginado de SourceResponse** (ver "Paginação").
- `PUT /sources/:id` — **admin**. `{ name (≤120), url, url_rss }` → 200 **SourceResponse** / 400 / 403 / 404 / 409.
- `DELETE /sources/:id` — **admin**. Soft delete → 204 (sem body) / 403 / 404. Cascata: soft-remove das `articles` da fonte.

## Articles (`/v1/articles`) — toda rota exige `Authorization: Bearer` válido (401 sem token)

Criação/edição/remoção são **exclusivas de admin** (403 para usuário comum); leitura e marcação de
leitura são de qualquer autenticado. As rotas de **dry-run de IA** (`treatment`, `judgement`) são
**exclusivas de desenvolvimento** — ver subseção no fim.

- `POST /articles/create` — **admin**. `{ title, content, url_original, keywords[5..20], source_id, language_original }` →
  201 **ArticleResponse** / 400 (inclui `source_id` ausente/fonte inativa e `language_original` ausente/inválido) / 403 / 409 (url_original duplicada) / 500.
  `language_original` é um código do enum de idiomas (`pt|en|es|fr|de|it`) e é obrigatório na criação manual (na CRON é detectado).
- `GET /articles/:id/translate` — **tradução personalizada** sob demanda (LLM apenas, `TRANSLATION_*`). O **idioma-alvo vem da preferência `language_to_translate` do usuário** (lida do JWT), **não da URL** (mudança 0.33.1). Traduz título+conteúdo para esse idioma, preservando o HTML e adaptando o tom à `ai_personality` (também do JWT); re-sanitiza a saída (bluemonday). **Keywords não são traduzidas** (ficam canônicas em inglês). **Read-only** (não grava; client cacheia). Aberta a qualquer usuário autenticado (a notícia é global; não exige que ela esteja num feed do usuário). → 200 `{ title, content, language, language_original }` / 400 (`language_to_translate` nulo/sem alvo, alvo == original, ou `language_original` null) / 404 / 500.
- `PUT /articles/:id/read` — marca como lida nos feeds do usuário. Sem body na resposta → **200**
  (em ≥1 feed, ou já lida) / **204** (não está em nenhum feed do usuário) / 404 (notícia inexistente). Idempotente.
- `GET /articles/:id` → 200 **ArticleResponse** / 404. **Rota principal de visualização de uma notícia.**
  Aberta a **qualquer usuário autenticado do sistema**, mesmo que a notícia não esteja em nenhum dos seus
  feeds — a notícia é entidade **global**, não por-usuário como `feeds`. O `is_read` vem enriquecido:
  `null` (não está em feed do usuário) | `false` (em ≥1 feed, ao menos um não lido) | `true` (todos lidos).
- `GET /articles?url=` — filtro por substring em url_original → 200 **envelope paginado de ArticleResponse** (ver "Paginação").
- `PUT /articles/:id` — **admin**. `{ title, content, url_original, keywords, language_original }` (sem `source_id`, imutável) → 200 **ArticleResponse** / 400 / 403 / 404 / 409.
- `DELETE /articles/:id` — **admin**. Soft delete → 204 (sem body) / 403 / 404.

### Dry-run de IA — **exclusivas de `ENVIRONMENT=development`** (nunca para clientes)

Registradas só em desenvolvimento (mesmo padrão do `dev-login`): a rota **não existe** em staging/
produção — resposta **404** (a rota não está montada), não 401. São ferramentas internas de simulação
do pipeline; **chamam a IA de verdade** (consomem quota), **não persistem** nada e expõem comportamento
interno — por isso jamais devem ser alcançáveis por um client.

- `POST /articles/treatment` — **dry-run** do tratamento. Body `{ article: { title, content, ... }, keywords_mode? }`. Roda detecção de idioma (lingua-go) + **tratamento de URLs** (reescreve links internos, leitura no banco) + **tratamento de embeds** (Instagram → link; `parent` do iframe do Twitch reescrito para o host do `CLIENT_URL`) + **sanitização do corpo cru** (bluemonday, iframes YouTube/Twitch por allowlist; todos determinísticos — a IA não toca no corpo desde a 0.34) → keywords (única etapa de IA). → 200 `{ content, keywords, keywords_mode, language_original, treatment_ms, keywords_ms }` (`content` é o corpo sanitizado; `treatment_ms` mede a sanitização) / 400 / 500 (falha da IA de keywords). O `keywords_mode` opcional (`local`|`groq`|`gemini`) troca o backend das keywords só nesta chamada (benchmark sem reiniciar; 400 se o modo não existe).
- `POST /articles/judgement` — **dry-run** do julgamento. Body `{ article: { title, keywords }, judgement_mode? }` (sem `id`; **sem `content`** — o corpo não é usado). Camada 1: feeds candidatos por overlap de keywords **com a contagem** (SQL `json_each` + `COUNT`, feeds ativos de qualquer usuário). Camada 2 = **triagem** (0.36.4): `overlap/nº-keywords-do-feed ≥ JUDGEMENT_AUTOASSOCIATE_RATIO` (0.30) → `auto_associated` sem IA; overlap `< JUDGEMENT_MIN_MATCHES` (2) → `discarded` sem IA; resto → `judged` pela IA (`score` 0–100 de `título + keywords` vs `JUDGEMENT_THRESHOLD`). → 200 `{ judgement_mode, threshold, candidate_count, judgements: [{ feed_id, feed_name, overlap, decision, score, passed }], judgement_ms }` / 400 / 500. `judgement_mode` opcional (`local`|`groq`|`gemini`). Ótimo para calibrar (mostra overlap+decisão de cada candidato).

## Feeds (`/v1/feeds`) — auth, **recurso por-usuário**

- Acesso restrito ao dono: feed de outro usuário responde **404** (não 403), sem vazar existência.
- `POST /feeds/create` — `{ name (≤120), keywords[5..20] }` → 201 **FeedResponse** / 400 / 409 (limite de 5 feeds ativos) / 500.
- `GET /feeds/check-for-new-articles` — **poll leve** de "quais feeds do usuário têm notícias não lidas".
  Rota estática (registrada **antes** de `/:id` para não ser capturada como id). **Não paginada** (é indicador,
  não listagem; o usuário tem no máx. 5 feeds). Contagem feita **em SQL** (`CountUnreadArticlesByFeedForUser`:
  junta feed+artigo ativos, conta `is_read=0`, agrupa por feed). Resposta é um **objeto plano** `{ "<feed_id>": <int>, ... }`
  só com feeds que têm ≥1 não lida; nenhuma → `{}`. → 200 / 500. Futuro: candidata a virar SSE/WebSocket.
- `GET /feeds/keyword-suggestions?keywords=&limit=` — sugere keywords para montar feed, do acervo
  **global** de artigos (usa `articleCtrl`, não `feedCtrl`; rota estática antes de `/:id`). **Não
  paginada** (indicador ranqueado; `limit` padrão 10, máx 50). `keywords` = já escolhidas (CSV,
  normalizadas, máx 20 → 400). Duas estratégias, ecoadas na resposta: `related` (co-ocorrência com as
  escolhidas, via `keywords ?|` no índice GIN `idx_articles_keywords`) e `popular` (mais frequentes
  numa janela `KEYWORD_SUGGESTIONS_WINDOW_DAYS`, padrão 30d, `-1` desliga). Fallback: related vazio →
  popular; nunca fica sem sugestão. Nunca sugere de volta uma keyword já escolhida. Ranking por
  contagem crua (empurrar genéricos é o objetivo). Resposta `{ strategy, suggestions: [{ keyword,
  count }] }` (array, nunca null). → 200 / 400 (>20 keywords) / 401.
- `GET /feeds/:id` → 200 **FeedResponse** / 404 (inexistente ou de outro usuário).
- `GET /feeds/:id/articles` — as notícias que caíram no feed (do dono), cada uma **ArticleResponse** com
  `is_read` sempre definido (a notícia está no feed). Filtros opcionais: `is_read` (`true`|`false`), janela
  `period_starting_at`/`period_ending_at` (RFC3339 UTC, sobre `created_at`, inclusivos), e `with_sources=true`
  (preenche `source` em cada item; qualquer outro valor/ausência é ignorado silenciosamente — convenção
  `with_{tabela_relacionada}=true` em `conventions.md`). → 200
  **envelope paginado de ArticleResponse** (ver "Paginação") / 404 (feed inexistente ou de outro usuário) / 400 (filtro inválido).
- `GET /feeds?name=` — só os próprios feeds, filtro por substring no nome → 200 **envelope paginado de FeedResponse** (ver "Paginação").
- `PUT /feeds/:id` — `{ name, keywords }` (sem `user_id`, imutável) → 200 **FeedResponse** / 400 / 404.
- `DELETE /feeds/:id` — soft delete (permanente, sem reativação) → 204 (sem body) / 404.

## Descoberta automática (CRON, sem rota)

- CRON interna (`services/cron`, `robfig/cron/v3`) varre as sources ativas em `RSS_FEED_CRON_SCHEDULE`,
  ativa por `RSS_FEED_CRON_ACTIVE`. Lê o RSS de cada source (gofeed), **deduplica por `url_original`**,
  **trata** as novas (detecta o idioma com lingua-go + **reescreve links internos** (url treatment →
  `CLIENT_URL/articles/{id}`, 0.35) + **trata embeds** (Instagram → link; `parent` do Twitch → host do `CLIENT_URL`, 0.36) +
  **sanitiza o corpo cru do RSS** com bluemonday (iframes YouTube/Twitch por allowlist) — todos
  determinísticos, a IA não toca no corpo desde a 0.34 — + SLM/LLM **nomeia keywords** (sobre o texto
  puro; mistura termos específicos + genéricos desde a 0.36.2),
  **persiste** o `article` (com `language_original`) e por fim **julga** (camada 1 SQL por keywords **com overlap** +
  camada 2 **triagem**: auto-associa overlap forte, descarta overlap 1, IA só no borderline por `título + keywords`
  vs `JUDGEMENT_THRESHOLD`), gravando as associações aprovadas em `articles_feeds`; ao
  final grava `system.last_article_discovery_at` (informativo). Falha de IA no tratamento → não
  persiste, re-tenta na próxima run; falha no julgamento é best-effort (a notícia já está persistida,
  não é re-julgada — não-retroativo). Pula a run quando `app_status` está off. Logs gated por
  `RSS_FEED_CRON_VERBOSE_MODE`. `DISCOVERY_MAX_ARTICLES` (default `-1` = sem cap) corta o lote da
  varredura — freio bruto de rate limit para testes (produção fica `-1`).

## Paginação

- Toda rota de busca de coleção (`GET /v1/articles`, `/v1/sources`, `/v1/feeds`, `/v1/feeds/:id/articles`) é **paginada**.
  Query: `?page=` (mín/padrão 1) e `?page_size=` (mín 1, máx 100, padrão 20). Resposta:
  `{ docs: [...], pagination: { actual_page, total_pages, actual_count, total_count, has_next_page,
has_previous_page } }`. Página fora do range → `docs` vazio (sem erro), `actual_page` fica no valor
  pedido.
- **Filtragem e paginação acontecem em SQL** (desde a 0.38). Cada listagem é um par de queries: a
  página (filtros + `LIMIT/OFFSET`) e um `Count*` irmão com os **mesmos filtros**, que alimenta o
  `total_count`. Ao mexer numa das duas, mexa na outra. Filtro opcional = `sqlc.narg` (`NULL` = não
  aplicado); substring = `strpos(lower(a), lower(b)) > 0` (não `ILIKE`, para `%`/`_` do usuário não
  virarem curinga). Helper global em `services/pagination`: `ParseParams` → `Params.Limit/Offset` →
  `BuildResponse(docs, totalCount, params)`.
- **Ordenação**: `created_at DESC, id DESC` em todas as listagens. O desempate por `id` (UUIDv7) não é
  cosmético: notícias do mesmo lote da CRON empatam no `created_at` e sem ordenação total o
  `LIMIT/OFFSET` repete/pula linha entre páginas.
- Consumidores **batch** (CRON, `cmd/seed`) não usam essas rotas nem o `List` dos controllers: usam
  `ListAll*` (sem `LIMIT`), porque precisam do conjunto inteiro. Handler HTTP sempre pagina.

## Notas

- Todo endpoint dispara log de início/fim quando `VERBOSE_MODE=true`.
- **CORS**: origens liberadas via `CORS_ALLOWED_ORIGINS` (lista separada por vírgula, origins
  completas com esquema). Sem cookie/credentials (auth é Bearer). Vazio = nenhuma origem cross-origin
  liberada (fail-closed).
- A coleção Bruno em `bruno/` espelha todas essas rotas.
