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
- `services/rss/` — descoberta de RSS.
- `middlewares/` — `auth.go` (parse JWT + valida sessão).
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

Ainda **não** implementado: descoberta automática via cron, tratamento/tradução/resumo por
IA, julgamento notícia→feed, deploy, usuário administrador, exclusão de usuário.

## Transações (regra central)

Toda escrita passa por `WithTransaction` (`services/controllers/transaction.go`), o **único**
dono de commit/rollback. Controllers são passos intermediários que recebem `q db.Querier` e
nunca finalizam. Quem orquestra o caso de uso (rota/middleware) abre a transação no topo.

## Como rodar

`task ls` (sobe local) · `task ta` (todos os testes) · `task tu`/`ti`/`te2e` (por camada) ·
`task va` (vet). Porta em `API_PORT`. Banco local em `db/news_feed.db` (descartável: se uma
migração `NOT NULL` quebrar o boot por dado antigo, basta apagar o arquivo).
