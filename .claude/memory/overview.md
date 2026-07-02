# Visão geral do projeto

> Resumo de entrada. Para detalhes, ver `.claude/PROJECT.md` (filosofia/fluxos),
> `.claude/rules/conventions.md` (convenções) e `.claude/ROADMAP.md` (estado das features).

## O que é

API REST em Go para um **feed de notícias hiper personalizado** baseado em RSS. O usuário
cria *feeds* com palavras-chave; o sistema descobre notícias das fontes RSS, trata/tagueia
via IA e julga em quais feeds cada notícia entra. Entrega personalizada por usuário.

## Stack

- **Go** + **Fiber** (web) + **SQLite** (`modernc.org/sqlite`, driver puro Go)
- **goose** (migrações, embutidas via `embed`) · **sqlc** (queries tipadas)
- **JWT** (auth Bearer) · **OAuth2 Google** (login) · **gofeed** (RSS) · **cron** (futuro)
- **Gemini 2.5 Flash** (IA, futuro) · **testify** (testes) · **Bruno** (coleção de requisições)

## Arquitetura (camadas)

- `main.go` — bootstrap: carrega `.env`, abre o SQLite, roda migrações, instancia
  controllers e registra rotas.
- `migrations/` — SQL goose (up/down). `schema.sql` — espelho do schema para o sqlc.
- `sqlc/` — código gerado (models, querier, `*.sql.go`) + `queries/*.sql`. Gerado à mão
  seguindo o padrão do sqlc (não há `sqlc generate` rodando no CI ainda).
- `schemas/` — DTOs de request/response e enums.
- `services/controllers/` — regras de negócio. **Stateless**, recebem `db.Querier`, nunca
  commitam (ver "Transações" abaixo).
- `services/endpoints/v1/<modelo>/` — handlers HTTP (um arquivo por rota).
- `services/rss/` — descoberta de URLs de RSS (a partir de uma URL principal).
- `services/discovery/` — descoberta de **notícias** no RSS de uma source (gofeed + retry). Seam
  `Processor`; `TreatmentProcessor` deduplica por `url_original`, trata via IA e persiste.
- `services/cron/` — scheduler (`robfig/cron/v3`) + `DiscoveryRunner` que varre as sources ativas.
- `services/sanitize/` — sanitiza a saída do tratamento pra HTML básico (bluemonday, política
  customizada: só tags básicas, sem `a`/`img`/`class`/`style`/URL). Decorator no `Treater`.
- `services/ai/` — costura de IA **por capacidade**: `Treater` (tratar), `Keyworder` (keywords) e
  `Judger` (julgar, `score` 0–100), com `ParseKeywords`/`ParseScore` compartilhadas. Impls: `gemini/`
  (LLM) e `openaicompat/` (Ollama local, Groq, ou qualquer endpoint OpenAI-compatible; API key
  opcional). Tratamento via `TREATMENT_*`; keywords via `KEYWORDS_MODE`; julgamento via `JUDGEMENT_MODE`
  + `JUDGEMENT_THRESHOLD` — modos `local`/`groq`/`gemini` pré-montados no boot (`buildModeClients`,
  compartilhado) e trocáveis por chamada nos dry-runs. `services/prompts/` — `.yaml` (`go:embed`).
- `services/judgment/` — camada 2 do julgamento: `Evaluator` (um `ai.Judger` + threshold) pontua uma
  notícia contra feeds candidatos. Sem I/O além da IA; buscar candidatos (camada 1, SQL `json_each`
  via `FeedController.FindCandidatesByKeywords`) e gravar `articles_feeds` é do chamador, então serve
  CRON e o `POST /articles/judgement` (dry-run) igual.
- `middlewares/` — `auth.go` (parse JWT + valida sessão); `app_status.go` (guard de manutenção
  global: 503 quando `system.app_status=0`, exceto `/health` e o toggle).
- `tests/` — `unit/`, `integration/api/`, `end-to-end/api/`, `fixtures/`, `mocks/`, `utils/`.

## Domínios já implementados

| Domínio | Tabela(s) | Resumo |
|---------|-----------|--------|
| Usuários/Auth | `users`, `refresh_tokens` | Login só via Google + JWT |
| Preferências | `user_preferences` | 1:1 com usuário; tema, idioma, tradução, personalidade IA |
| Fontes | `sources` | Globais; CRUD + descoberta de RSS |
| Notícias | `articles` | Globais; ligadas a uma `source`; CRUD (criação manual/admin por ora) |
| Feeds | `feeds` | Por usuário; palavras-chave; máx. 5 ativos |
| Feed × Notícia | `articles_feeds` | Junction com `is_read` |
| Sistema | `system` | Singleton; `app_status` (chave de manutenção global) + `last_article_discovery_at` |

Descoberta via CRON (0.19) + **tratamento por IA e persistência** (0.20/0.21) + **julgamento**
(0.22): a CRON descobre, deduplica por `url_original`, trata o conteúdo com uma **LLM** (`TREATMENT_*`,
sanitizado pra HTML básico), nomeia keywords com uma **SLM/LLM** por modo (`KEYWORDS_MODE` =
local/groq/gemini, em inglês minúsculo), grava o `article` e por fim **julga** a quais feeds ele
pertence (camada 1 SQL por keywords + camada 2 IA vs `JUDGEMENT_THRESHOLD`), gravando as associações
em `articles_feeds`. Ainda **não** implementado: tradução/resumo por IA (**0.23**), deploy, usuário
administrador, exclusão de usuário.

## Transações (regra central)

Toda escrita passa por `WithTransaction` (`services/controllers/transaction.go`), o **único**
dono de commit/rollback. Controllers são passos intermediários que recebem `q db.Querier` e
nunca finalizam. Quem orquestra o caso de uso (rota/middleware) abre a transação no topo.

## Como rodar

`task ls` (sobe local) · `task ta` (todos os testes) · `task tu`/`ti`/`te2e` (por camada) ·
`task va` (vet). Porta em `API_PORT`. Banco local em `db/news_feed.db` (descartável: se uma
migração `NOT NULL` quebrar o boot por dado antigo, basta apagar o arquivo).
