# Banco de dados

> Resumo. Regras completas em `.claude/rules/conventions.md` (seção banco de dados) e
> `.claude/PROJECT.md` (status, exclusão, cascades). Schema-fonte em `schema.sql`.

## Motor: PostgreSQL 18

Desde a **0.37** o banco é PostgreSQL 18 (antes era SQLite). É cliente-servidor: os dados vivem no
servidor, não num arquivo do repo. A conexão inteira vem de `DATABASE_URL` (DSN libpq), o driver é
**pgx** (`jackc/pgx/v5/stdlib`) usado através de `database/sql` — o sqlc gera com
`sql_package: "database/sql"`, então models e controllers seguem em `sql.NullString`/`sql.NullTime`.

Em dev o servidor sobe no docker-compose (`postgres:18-alpine`, volume `postgres_data`, pgAdmin4 na
`:5050`). A API roda no host (`task ls`) ou no compose (`task ds`) — só o **banco** exige Docker.

**Migrações**: uma única inicial (`20260716120000_initial_schema.sql`). As 13 migrações SQLite foram
descartadas, não portadas — nenhum ambiente havia sido publicado e os dados eram descartáveis.

## Convenções transversais

- `id` é **UUID v7** (imutável), gerado em Go e armazenado como `TEXT`. O tipo nativo `uuid` fica para
  uma versão futura (ripple em todo model/teste). O PG 18 tem `uuidv7()` nativo, hoje não usado.
- Tabelas comuns têm `id`, `status` (**BOOLEAN**), `created_at`, `modified_at`, `removed_at`
  (todos **TIMESTAMPTZ** — o banco passa a garantir o UTC que a convenção exige).
- **Soft delete**: `status = FALSE` + `removed_at` preenchido. Junction tables são exceção:
  só têm `id` (+ FKs e campos extras), e sofrem **hard delete**.
- **Predicado de leitura ativo** (em toda query de busca/listagem/update):
  `status = TRUE AND removed_at IS NULL`.
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

`id`, `user_id` (UNIQUE, FK users), `status`, `language_to_translate`
(**nullable**, `pt`|`en`|`es`|...; null = tradução off, só dica de client), `ai_personality`
(`fun`|`informative`|`mixed`), timestamps. Defaults: language_to_translate=pt, mixed.
`theme` e `translate_content` foram removidos na 0.33 (tema é do client; opt-in de tradução
virou o próprio `language_to_translate` nulo/preenchido).

### refresh_tokens (N:1 com users)

`id`, `user_id` (FK users), `status`, `expires_at`, timestamps. O `id` do token É o valor
usado no fluxo de refresh.

### sources (globais)

`id`, `status`, `name` (**NOT NULL**, nome de exibição, ex. "G1"), `url`, `url_rss`, timestamps.
Índices únicos parciais em `url` e `url_rss` (não em `name` — sem unicidade).

### articles (globais)

`id`, `status`, `title`, `content` (HTML básico, sanitizado por bluemonday), `url_original`, `keywords` (**JSONB**),
`source_id` (**NOT NULL**, FK sources, **imutável**), `language_original` (**nullable**, código do enum
de idiomas), timestamps. Índice único parcial em `url_original`. Keywords: 5–20 itens, geradas **em
inglês** (canônico, pra matching entre fontes de qualquer idioma no julgamento). `language_original`
é o idioma detectado pelo `lingua-go` no tratamento (null quando a detecção falha); na criação/edição
manual é obrigatório no payload. Usado pela tradução para saber a origem.

### feeds (por usuário)

`id`, `status`, `name`, `keywords` (**JSONB**, 5–20), `user_id` (FK users), timestamps.
Sem unicidade (nomes duplicados permitidos). Máx. **5 feeds ativos** por usuário.
Índice **GIN** em `keywords` (0.37): a camada 1 do julgamento procura overlap contra os feeds ativos,
e o GIN evita varrer a tabela inteira. Não existia no SQLite, onde `keywords` era TEXT opaco.

> **Um índice GIN só é alcançado por um dos seus operadores** (`@>`, `?`, `?|`, `?&`). Expandir a
> coluna com `jsonb_array_elements_text` — que é o que a contagem de overlap precisa fazer — é
> chamada de função linha a linha e **nunca** usa o índice. Por isso a `FindCandidateFeedsByKeywords`
> carrega um predicado explícito `keywords ?|` junto da expansão. Ele é redundante com o join (não
> muda o resultado), mas é o que deixa o planner descartar não-candidatos antes da parte cara.
> Removê-lo faz o índice parar de ser usado **em silêncio** e a camada 1 volta a varrer tudo — foi
> exatamente o bug corrigido na **0.37.3.0** (medido em 60k feeds: 428ms → 22ms).

> Atenção ao ler/gravar `keywords`: JSONB guarda a **estrutura**, não o texto. O PG re-renderiza na
> leitura (notadamente com espaço após a vírgula), então comparar os bytes crus do que foi gravado
> não funciona — compare o JSON decodificado (nos testes, `assert.JSONEq`).

### articles_feeds (junction feed × notícia)

`id`, `article_id` (FK articles), `feed_id` (FK feeds), `is_read` (**BOOLEAN**), `created_at`,
`modified_at`. **Sem** `status`/`removed_at`. `user_id` é obtido via JOIN com `feeds` (sem
desnormalização). Hard delete quando um pai sofre hard remove; no soft remove de um pai o
registro **permanece** mas fica invisível (filtrado pelo JOIN de ativos). Populado pelo
**julgamento** (0.22): cada feed aprovado vira um registro. O `score` do julgamento **não** é
gravado (artefato transitório; julgamento não-retroativo). Camada 1 do julgamento:
`FindCandidateFeedsByKeywords` compara keywords notícia↔feed inteiramente em SQL via
`jsonb_array_elements_text` (feeds ativos de qualquer usuário, `DISTINCT`). O lado do feed é expandido
com `CROSS JOIN LATERAL` — é o `LATERAL` que permite a expansão referenciar a linha do feed sendo
varrida; o lado da notícia não depende da linha e é um join comum. Agrupa por `f.id` (válido: é PK, as
demais colunas são funcionalmente dependentes). Era `json_each` no SQLite.
`CountUnreadArticlesByFeedForUser` (0.32) conta as não lidas (`is_read = FALSE`) por feed ativo do
usuário, agrupando em SQL — alimenta a rota de poll `GET /feeds/check-for-new-articles`.

### system (singleton — painel de controle)

`id` (UUID v7), `app_status` (**BOOLEAN**), `last_article_discovery_at` (TIMESTAMPTZ nullable),
`created_at`, `modified_at`. **Exceção de convenção:** sem `status`/`removed_at` — é um painel
de controle, não um registro soft-deletável. **Linha única**: semeada na migração e **só sofre
update** (nunca insert/delete por código). O `id` é um literal v7 fixo
(`01900000-0000-7000-8000-000000000001`) **de propósito**: todo ambiente precisa concordar com o id
desta linha, então ele tem que ser determinístico. (Antes era literal por falta de v7 no SQLite; o
PG 18 tem `uuidv7()`, mas gerar aleatório aqui perderia o ponto.)
Queries: `GetSystem` (`LIMIT 1`) e `UpdateSystemAppStatus`. `app_status` é a chave de
manutenção global (ver guard em `endpoints.md`/`auth`). `last_article_discovery_at` será usado
pela descoberta via CRON (0.19).

## Cascades (executados nos controllers)

- `users` soft remove → soft remove de `user_preferences`, `refresh_tokens`, `feeds`.
- `sources` soft remove → soft remove de `articles` (da fonte).
- Hard remove dos pais → hard remove dos registros de junction relacionados (futuro; sem
  gatilho hoje, pois não há rotas de hard delete).
