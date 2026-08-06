# Banco de dados

> Resumo. Regras completas em `.claude/rules/conventions.md` (seção banco de dados) e
> `.claude/PROJECT.md` (status, exclusão, cascades). Schema-fonte em `schema.sql`.

## Motor: PostgreSQL 18

Desde a **0.37** o banco é PostgreSQL 18 (antes era SQLite). É cliente-servidor: os dados vivem no
servidor, não num arquivo do repo. A conexão inteira vem de `DATABASE_URL` (DSN libpq), o driver é
**pgx** (`jackc/pgx/v5/stdlib`) usado através de `database/sql` — o sqlc gera com
`sql_package: "database/sql"`, então models e controllers seguem em `sql.NullString`/`sql.NullTime`.

O `lib/pq` também aparece no `go.mod`, mas **não é um segundo driver ativo**: entra só como helper de
encoding, porque o sqlc emite `pq.Array` para o `ANY(...::text[])` de
`ListArticleOutboundLinksByArticleIDs`. Não tente trocar por `sqlc.slice()` para removê-lo — foi
tentado na 0.46 e é quebrado para o engine postgresql (perde o marcador `/*SLICE:*/` e gera placeholder
`?` de MySQL). O sintoma é traiçoeiro: compila, passa no vet e passa em qualquer teste que envie **um**
id; só quebra com dois ou mais.

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
`last_login_at` (nullable), `created_at`, `modified_at`, `removed_at`, `admin`.

`admin` (0.40): `BOOLEAN NOT NULL DEFAULT FALSE`, sem índice (só é lido por PK, nunca filtrado).
É a fonte de verdade da autorização de administrador — o claim homônimo no JWT é só dica de client.
A coluna **não** é escrita pelo `CreateUser`: quem promove é a query `SetUserAdmin`, então nenhum
fluxo de login consegue criar um administrador. Fica no fim da tabela, onde o `ALTER TABLE` a colocou;
como as queries usam `SELECT *`, a ordem de colunas do `schema.sql` precisa bater com a real.

`last_active_at` (0.43): `TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`, sem índice. Sinal de
**atividade** (não de login): escrito no `/auth/refresh`, no login e no `dev-login` (query
`UpdateUserLastActive`). É lido pela camada 1 do julgamento (`FindCandidateFeedsByKeywords` faz um
`JOIN users` e descarta feeds de usuários não vistos dentro de `DAYS_UNTIL_USER_IS_INACTIVE` dias; `-1`
desliga). Backfill automático no `ADD COLUMN`; usuário novo nasce com `now()`. Também no fim da tabela.

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

> **0.45 — formato do `content`:** desde a 0.45 o corpo gravado **não guarda URLs** nas âncoras: cada
> `<a href>` carrega o **id** de uma linha em `article_outbound_links`. A URL real é resolvida a cada
> leitura (swap). Notícias gravadas antes da 0.45 mantêm URLs reais e passam intactas (o swap só troca
> href que casa com um id de outbound). Ver `article_outbound_links` abaixo.

### article_outbound_links (0.45 — ponteiros de link do corpo)

`id`, `article_id` (FK articles), `href`, `created_at`, `modified_at`. **Exceção de convenção:** sem
`status`/`removed_at`. Índices em `article_id` (swap de leitura em batch) e `href` (retarget/limpeza por
match exato). Serve para corrigir links entre notícias de forma **retroativa**: o corpo de uma notícia é
imutável, então quando a notícia B chega representando uma URL que A já linkava, só se altera a linha
desta tabela (não o corpo de A). Uma linha por **href distinto** do corpo. `href` guarda a URL real:
link **interno** como o token literal `{CLIENT_URL}/articles/{id}` (desacopla de mudança de domínio; o
swap expande na leitura), **externo** literal. **Invariante:** o id e o token não são URLs válidas, então
nunca podem passar pelo bluemonday (o corpo em forma-id só existe pós-sanitize; o swap roda antes de
qualquer re-sanitização). Cascade hard-delete por `article_id` (dormente — notícias só sofrem
soft-remove). Serviço `services/outboundlinks` (`Assign` grava, `Resolve` troca de volta). Populada na
etapa "operações no banco de dados" do tratamento (`discovery/processor.go`, numa transação só).

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
