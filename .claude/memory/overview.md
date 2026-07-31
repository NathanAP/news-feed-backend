# Visão geral do projeto

> Resumo de entrada. Para detalhes, ver `.claude/PROJECT.md` (filosofia/fluxos),
> `.claude/rules/conventions.md` (convenções) e `.claude/ROADMAP.md` (estado das features).

## O que é

API REST em Go para um **feed de notícias hiper personalizado** baseado em RSS. O usuário
cria _feeds_ com palavras-chave; o sistema descobre notícias das fontes RSS, trata/tagueia
via IA e julga em quais feeds cada notícia entra. Entrega personalizada por usuário.

## Stack

- **Go** + **Fiber** (web) + **PostgreSQL 18** (driver `jackc/pgx/v5` via `database/sql`; SQLite saiu na 0.37)
- **goose** (migrações, embutidas via `embed`) · **sqlc** (queries tipadas, `engine: postgresql`)
- **JWT** (auth Bearer) · **OAuth2 Google** (login) · **gofeed** (RSS) · **cron** · **lingua-go** (detecção de idioma)
- **Gemini 2.5 Flash** (IA, futuro) · **testify** + **testcontainers-go** (testes) · **Bruno** (coleção de requisições)

## Arquitetura (camadas)

- `main.go` — bootstrap: carrega `.env`, põe o logger padrão em UTC (`log.LUTC`), conecta no Postgres
  (`DATABASE_URL`), roda migrações, instancia controllers e registra rotas.
- `services/utctime/` — `Time`/`NullTime`, os tipos das colunas `timestamptz` (plugados via
  `overrides` no `sqlc.yaml`). Existem porque o pgx materializa `timestamptz` no fuso **local do
  processo**: sem eles o formato de data da API dependeria do relógio do host (`-03:00` em vez de `Z`).
  O `Scan` normaliza na fronteira e o `MarshalJSON` sempre emite `Z`. Data **gerada em Go** (não vinda
  do banco) o tipo não alcança — precisa de `.UTC()` explícito (ver `conventions.md`, datas).
- `migrations/` — SQL goose (up/down). `schema.sql` — espelho do schema para o sqlc.
- `sqlc/` — código gerado (models, querier, `*.sql.go`) + `queries/*.sql`. Desde a 0.30, gerado
  rodando o `sqlc` real (`task sg` / `sqlc generate`, binário `v1.31.1`) — não há `sqlc generate`
  automático no CI ainda, mas o código deixou de ser escrito à mão. Gotcha do parser: evitar
  caracteres não-ASCII (em-dash etc.) em comentários dos `.sql` de `queries/` — corrompe a geração.
- `schemas/` — DTOs de request/response. Enums em `schemas/enums/` (fonte única, um arquivo por
  enum: `language.go`, `ai_personality.go`), referenciados como `enums.X`.
- `services/controllers/` — regras de negócio. **Stateless**, recebem `db.Querier`, nunca
  commitam (ver "Transações" abaixo).
- `services/endpoints/v1/<modelo>/` — handlers HTTP (um arquivo por rota).
- `services/rss/` — descoberta de URLs de RSS (a partir de uma URL principal).
- `services/discovery/` — descoberta de **notícias** no RSS de uma source (gofeed + retry). Seam
  `Processor`; `TreatmentProcessor` deduplica por `url_original`, trata via IA e persiste.
- `services/cron/` — scheduler (`robfig/cron/v3`) + `DiscoveryRunner` que varre as sources ativas. A
  varredura de RSS e o pipeline por artigo rodam num **worker pool limitado** por `DISCOVERY_CONCURRENCY`
  (default `1` = sequencial, dev/testes; sobe em staging/prod, limitado pelo rate limit da IA). O pipeline
  por artigo é `discovery.processOne` — self-contained e concorrente-safe (dedup + unique de `url_original`
  colapsam duplicatas do batch; cada escrita em sua própria transação curta). É o caso de 1 nó do modelo
  futuro fila+workers (ROADMAP "Escalabilidade futura", pós-Postgres): trocar dispatcher, não reescrever.
- `services/sanitize/` — **estágio primário** de limpeza do corpo (0.34): aplica a whitelist do
  bluemonday direto no HTML cru do RSS (determinístico, sem IA). Política permissiva-porém-segura:
  formatação + `<a href>` + `<img src>` (esquemas seguros, `rel=nofollow`) + `<iframe>` de embed **só** com
  `src` na allowlist (YouTube/Twitch, 0.36); bloqueia `script`/`style`/`on*`/`class` e qualquer iframe fora
  da allowlist. Função de pacote `Sanitize(html)`, usada no pipeline e no endpoint de tradução.
- `services/embedtreatment/` — passo determinístico **antes** do sanitize (0.36): (1) converte embeds via
  `<script>` (Instagram: `<blockquote class="instagram-media">`) em `<a href={permalink}>{permalink}</a>`,
  já que `<script>` nunca é liberado; (2) reescreve o `parent` do iframe do Twitch para o host do
  `CLIENT_URL` (0.36.1 — senão o player renderiza mas não reproduz). Puro (`x/net/html`), best-effort.
- `services/urltreatment/` — passo determinístico **antes** do sanitize (0.35): parseia o HTML (`x/net/html`),
  acha os `<a href>` e reescreve os que casam a `url_original` de uma notícia nossa para `CLIENT_URL/articles/{id}`.
  DB-agnóstico: recebe um `Resolve` injetado (1 transação de leitura por artigo, via `FindByURLOriginal`).
  Best-effort (erro → corpo original) e pulado sem `CLIENT_URL`.
- `services/ai/` — costura de IA **por capacidade**: `Keyworder` (keywords), `Judger` (julgar,
  `score` 0–100) e `Translator` (traduzir, LLM-only), com
  `ParseKeywords`/`ParseScore`/`ParseTranslation` compartilhadas. **A IA não toca no corpo** (a
  capacidade `Treater` foi removida na 0.34). Impls: `gemini/`
  (LLM) e `openaicompat/` (Ollama local, Groq, ou qualquer endpoint OpenAI-compatible; API key
  opcional). Keywords via `KEYWORDS_MODE`; julgamento via `JUDGEMENT_MODE`
    - `JUDGEMENT_THRESHOLD` — modos `local`/`groq`/`gemini` pré-montados no boot (`buildModeClients`,
      compartilhado) e trocáveis por chamada nos dry-runs. `services/prompts/` — `.yaml` (`go:embed`).
- `services/judgement/` — camada 2 do julgamento: `Evaluator` (um `ai.Judger` + threshold) pontua uma
  notícia contra feeds candidatos. Sem I/O além da IA; buscar candidatos (camada 1, SQL `json_each`
  via `FeedController.FindCandidatesByKeywords`) e gravar `articles_feeds` é do chamador, então serve
  CRON e o `POST /articles/judgement` (dry-run) igual.
- `services/langdetect/` — detecção do idioma original da notícia via `lingua-go` (offline,
  determinístico, **não é IA**), restrito aos idiomas suportados. Roda no tratamento e grava em
  `articles.language_original` (null quando não confiável). Capacidade `ai.Translator` (LLM apenas,
  `TRANSLATION_*`) faz a **tradução personalizada** on-demand (`GET /articles/:id/translate/:language`),
  read-only, respeitando `ai_personality`. Enum de idiomas único em `schemas/enums/language.go`.
- `services/pagination/` — paginação global (`ParseParams` → `Params.Limit/Offset` →
  `BuildResponse(docs, total, params)`). Toda rota de lista (`GET /v1/{model}`) responde
  `{ docs, pagination }` (`page`/`page_size` na query). Desde a 0.38 filtragem e paginação são **SQL**
  (`LIMIT/OFFSET` + query de contagem irmã com os mesmos filtros); nada é filtrado ou fatiado em Go.
  Listagens ordenam por `created_at DESC, id DESC` — o desempate pelo `id` (UUIDv7) é o que mantém a
  paginação consistente quando a CRON grava várias notícias no mesmo instante.
- `middlewares/` — `auth.go` (parse JWT + valida sessão); `admin.go` (0.40: `AdminResolver` +
  `RequireAdmin`, autorização de administrador lendo `users.admin` **no banco**, nunca o claim);
  `app_status.go` (guard de manutenção global: 503 quando `system.app_status` é falso, exceto
  `/health`, o toggle e todo o grupo `/auth` — e exceto requisições de administrador); `cors.go`
  (`CORS_ALLOWED_ORIGINS` + `CORS_ALLOWED_HEADERS`, ambos de env; origens vazias = fail-closed;
  headers vazios = default `Authorization,Content-Type`).
- `tests/` — `unit/`, `integration/api/`, `end-to-end/api/`, `fixtures/`, `mocks/`, `utils/`.

## Domínios já implementados

| Domínio        | Tabela(s)                 | Resumo                                                                             |
| -------------- | ------------------------- | ---------------------------------------------------------------------------------- |
| Usuários/Auth  | `users`, `refresh_tokens` | Login só via Google + JWT; flag `admin` (0.40)                                     |
| Preferências   | `user_preferences`        | 1:1 com usuário; idioma-alvo de tradução (anulável), personalidade IA              |
| Fontes         | `sources`                 | Globais; CRUD + descoberta de RSS                                                  |
| Notícias       | `articles`                | Globais; ligadas a uma `source`; CRUD (criação manual/admin por ora)               |
| Feeds          | `feeds`                   | Por usuário; palavras-chave; máx. 5 ativos                                         |
| Feed × Notícia | `articles_feeds`          | Junction com `is_read`                                                             |
| Sistema        | `system`                  | Singleton; `app_status` (chave de manutenção global) + `last_article_discovery_at` |

Descoberta via CRON (0.19) + **tratamento e persistência** (0.20/0.21) + **julgamento**
(0.22): a CRON descobre, deduplica por `url_original`, detecta o idioma (lingua-go), **reescreve links
internos** (url treatment → `CLIENT_URL/articles/{id}`, 0.35), **converte embeds via script** (Instagram
→ link, 0.36) e **sanitiza o corpo cru do RSS** (bluemonday, determinístico — desde a 0.34 a IA não
reescreve mais o corpo; iframes YouTube/Twitch permitidos por allowlist), nomeia
keywords com uma **SLM/LLM** por modo (`KEYWORDS_MODE` = local/groq/gemini; inglês minúsculo; **texto
puro** como input; mistura específicas + genéricas, máx. 30 — 0.36.2),
grava o `article` (com `language_original`) e por fim **julga** a quais
feeds ele pertence (camada 1 SQL por keywords **com overlap** — e, desde a 0.43, só de **usuários ativos**
(`last_active_at` dentro de `DAYS_UNTIL_USER_IS_INACTIVE`, `-1` desliga) + camada 2 **triagem**: auto-associa
overlap forte / descarta overlap 1 / IA só no borderline por `título + keywords` vs `JUDGEMENT_THRESHOLD`
— envs `JUDGEMENT_AUTOASSOCIATE_RATIO`/`JUDGEMENT_MIN_MATCHES`, 0.36.4), gravando as associações em `articles_feeds`. **Tradução personalizada** on-demand por usuário já existe (0.23,
LLM-only, read-only). **Modo administrador** (0.40): flag `users.admin`, rotas de escrita de
`articles`/`sources`, invalidação de sessões e o toggle de manutenção restritos a administradores
(403 para usuário comum), com administradores atravessando a manutenção. **Estrutura de staging/produção
+ CI/CD** (0.42): compose multi-ambiente, Taskfile, templates de env e workflows do GitHub Actions
prontos — falta só ter a AWS pra ligar o deploy. Ainda **não** implementado: **resumo** por IA (depende
de Redis), backup, exclusão de usuário.

## Transações (regra central)

Toda escrita passa por `WithTransaction` (`services/controllers/transaction.go`), o **único**
dono de commit/rollback. Controllers são passos intermediários que recebem `q db.Querier` e
nunca finalizam. Quem orquestra o caso de uso (rota/middleware) abre a transação no topo.

Com o Postgres (0.37) **caiu o teto de single-writer** do SQLite: escrita concorrente é real, e os
pragmas `busy_timeout`/`journal_mode(WAL)` que existiam no DSN deixaram de fazer sentido e saíram. Isso
destrava o `DISCOVERY_CONCURRENCY > 1` de verdade, múltiplas réplicas da API e a seção "Escalabilidade
futura" (fila + workers) do ROADMAP. Violação de UNIQUE continua detectada por erro **tipado** do
driver (`controllers.isUniqueViolation`), agora via `*pgconn.PgError` + SQLSTATE `23505` — nunca por
texto do erro.

## Como rodar

`task dbs` (sobe Postgres + pgAdmin4 — **pré-requisito**) · `task ls` (sobe a API local) ·
`task ta` (todos os testes) · `task tu`/`ti`/`te2e` (por camada) · `task va` (vet). Porta em `API_PORT`.

O banco **não é mais um arquivo**: é o servidor Postgres do compose, com os dados no volume
`postgres_data`. `task dd` (down) preserva o volume; `task dp` (prune) o apaga — é o "começar do zero".
Inspeção via pgAdmin4 em `localhost:5050` (servidor `news-feed` já pré-registrado; pede a senha).

**Testes exigem Docker ligado**: o Postgres não tem modo em memória, então o `SetupTestDB`
(`tests/utils/db.go`) sobe um container descartável via testcontainers, migra um database **template**
uma vez e cada teste **clona** o template para ter o seu (`CREATE DATABASE ... TEMPLATE`), dropado no
fim. Um container serve a execução inteira (o `go test` roda os pacotes em paralelo, em processos
separados; sem isso seriam 14 Postgres simultâneos) e o reaper do testcontainers o destrói ao fim —
nada sobra. Suíte completa a frio: ~40s.

**Seed de dev** (`ENVIRONMENT=development`): `task sdfull` popula o banco (usuário dev, sources,
feeds, artigos, associações) e imprime um token; `task sdl` só emite o token. `cmd/` é exclusivo de
dev (ver `.claude/memory/cmd.md`). Login do dev sem OAuth: CLI `task sdl` ou `POST /v1/users/dev-login`
(rota registrada só em `development`).

**Docker**: `task ds` (`docker compose up -d --build`) sobe **Postgres 18** (volume `postgres_data`,
healthcheck `pg_isready`) + a **API** (Dockerfile multi-stage, binário Go puro `CGO_ENABLED=0` — o pgx
também é Go puro; healthcheck em `/health`; só inicia depois do banco healthy, pois migra no boot) +
**pgAdmin4** (`:5050`). Só o banco é obrigatório em Docker; a API roda igual no host via `task ls`
(o compose sobrescreve o `DATABASE_URL` do `.env`, que aponta para `localhost`, com o host `postgres`
da rede interna). Em Docker use `KEYWORDS_MODE=groq|gemini` (Ollama local do host não é alcançável por
`localhost` no container; `host.docker.internal` disponível via `extra_hosts`).

**Ambientes e deploy (0.41/0.42)**: o compose é multi-ambiente. **Base** (`docker-compose.yaml`) = só a
`api`, com imagem parametrizada (`API_IMAGE`). **Dev** (`docker-compose.override.yaml`, auto-merge) =
postgres+pgadmin+`build`. **Staging/produção** = overlays com `-f` explícito (`task staging-up`/`prod-up`),
que desliga o auto-override — pgAdmin nunca sobe fora de dev. Produção usa **RDS por padrão** (postgres
container comentado, removível). Portas em `127.0.0.1` (0.41). **CI/CD** em `.github/workflows/`:
`ci.yml` (fmt+vet+test, sempre ativo) e `deploy.yml` (build→ECR→EC2 via SSH/OIDC, inerte até
`DEPLOY_ENABLED=true`). Modelo de deploy = **construir imagem no CI e enviar** (o servidor só dá `pull`,
nunca builda). **Instância única em produção**: a CRON é in-process, réplicas duplicariam a descoberta.
