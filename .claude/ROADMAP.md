# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.37.3.0

## Versão 0.29.0.0

- [x] Criar uma rota para buscar notícias dos feeds
    - `GET /v1/feeds/{id}/articles`; filtros `is_read` + `period_starting_at`/`period_ending_at` (UTC) em Go, paginação padrão; `is_read` na resposta (reusa `ArticleResponse`); 404 vs 200-vazio
    - Pode ser debaixo de `GET base_url/v1/feeds/{id}/articles`
    - Possibilitar filtragem através da query:
        - `is_read`: apenas lidos ou não lidos
        - `period_starting_at` e `period_ending_at`: filtro direto em `articles.created_at` a partir de uma data inicial, até uma data final ou entre duas datas
    - Lembrete: 404 caso o feed com o `id` for inexistente. 200 e lista vazia caso não haja notícias para aquele filtro (docs vazio).
    - Lembrete: apenas do usuário da requisição (relacionado ao `access_token`)
    - Lembrete: trazer também o estado da notícia (por enquanto é o campo da tabela relacional `is_read`)
    - Lembrete: paginação necessária
- [x] Configurar CORS
    - Middleware `middlewares/cors.go` lendo `CORS_ALLOWED_ORIGINS`; sem `AllowCredentials` (Bearer); vazio = fail-closed
    - Utilizaremos React + Vite pro nosso client web
    - Preparar tanto para dev quanto para homologação e produção
- [x] Alterar o callback do `OAuth2` para ir de volta ao frontend
    - `redirect_uri` validado por allowlist (`OAUTH_ALLOWED_REDIRECT_URIS`), `state` assinado (pacote `services/oauthstate`), callback redireciona ao client com tokens no `fragment`

## Versão 0.30.0.0

- [x] Adicionar o campo `name` à fonte de notícias
    - Não precisa se preocupar com bancos existentes, vou excluir o meu localmente aqui antes de fazer a migração
    - Lembre-se de alterar todas as rotas, testes e memory também
- [x] Permitir que a fonte da notícia venha também na rota de notícias do feed
    - No endpoint `GET /v1/feeds/{id}/articles` precisamos de um filtro que traga também a fonte daquela notícia, assim podemos preencher no client
    - Vamos fazer a opção via query para vir ou não com as sources populadas, a opção pode ser `with_sources=` e se for diferente de `true`, volta sem o preenchimento

## Versão 0.31.0.0

- [x] Fechar as rotas de dry-run `POST /v1/articles/treatment` e `POST /v1/articles/judgement` ao modo de desenvolvimento
    - Estavam registradas incondicionalmente (só atrás de auth), acessíveis em produção por qualquer usuário autenticado
    - Agora registradas só quando `ENVIRONMENT=development` (mesmo padrão do `dev-login`); fora de dev a rota não existe (404)
    - Motivo: são ferramentas internas que chamam a IA de verdade (consomem quota) e expõem o pipeline — jamais para clientes
    - Doc alinhada em `endpoints.md`, `PROJECT.md` e nos `.bru`; a nota de "notícia global, sem checagem de posse" foi movida do cabeçalho da seção Articles para a rota principal de visualização (`GET /articles/:id`) e mencionada na tradução

## Versão 0.32.0.0

- [x] Criar um endpoint para saber quais feeds possuem notícias não lidas
    - Pensei em fazer uma rota só pra isso para que a gente pudesse pesquisar nela a cada pouco
    - Acredito que futuramente daria pra fazer um websocket ou um SSE disso?
    - Pode estar em `GET base_url/v1/feeds/check-for-new-articles`
    - A reposta pode ser `{ "{feed_id}": quantidade_de_noticias_nao_lidas_deste_feed_em_int, ... }`
    - Neste caso não precisa paginação
    - Lembrando que apenas os feeds ativos devem participar desta chamada
- [x] Criar uma funcionalidade que dá skip na LLM durante o tratamento de notícias
    - Usar a variável `TREATMENT_AI_ACTIVE` da forma descrita no `PROJECT.md`

## Versão 0.33.0.0

- [x] Alterar preferências do usuário
    - [x] Remover tema light/dark, isso vai ser gravado via localStorage no client mesmo. Esse controle não é nosso
    - [x] Ao invés de se chamar apenas "Language", vamos trocar o campo para "TranslateToLanguage" com a possibilidade dele ser vazio/nulo
        - Ficou como `language_to_translate` (nullable). Quando nulo a gente não mostra a opção de tradução no client
    - [x] A personalidade por enquanto pode deixar como está
    - Decisão de modelo: `translate_content` foi **colapsado** no `language_to_translate` nulo/preenchido (Opção A), removendo o campo redundante. A tradução virou capacidade read-only sem gate de preferência no servidor; a preferência é só dica de client e não afeta a API

## Versão 0.33.1.0

- [x] A rota de tradução não deveria requerer o idioma a ser traduzido, o ideal era pegar direto do que está nas preferências do usuário...
    - Rota virou `GET /v1/articles/{id}/translate` (sem o param de idioma). O alvo agora vem da preferência `language_to_translate` do JWT; nula = sem alvo → 400
    - Corrige a premissa errada da 0.33: `language_to_translate` **afeta** a resposta da API (é ele que decide o alvo). O client só altera o valor; quem decide o comportamento é a API

## Versão 0.34.0.0

Reforma do tratamento de notícias (PROJECT.md) quebrada em 3 fases.

**Fase 1** — a IA deixa de reescrever o corpo; o `bluemonday` vira o estágio primário sobre o RSS cru.

- [x] Remover a capacidade `ai.Treater` (LLM) e o código morto associado: `passthrough`, decorator `sanitize.NewTreater`, `NewVerboseTreater`, métodos `Treat` dos providers (gemini/openaicompat) e do mock, e as envs `TREATMENT_*` (incluindo o `TREATMENT_AI_ACTIVE` da 0.32, agora superseded)
- [x] Sanitizador novo (política permissiva-porém-segura): tags básicas de formatação + `<a href>` + `<img src>` (só esquemas seguros; sem `script`/`on*`/`style`/`iframe`). Embeds ainda são **removidos** nesta fase (resultado seguro; vídeo/post visual se perde até a Fase 3)
- [x] Reordenar o pipeline de tratamento: detecção de idioma → sanitização → nomeação de keywords (a IA nunca mais toca no corpo)
- [x] Atualizar o dry-run `POST /v1/articles/treatment` (sem passo de LLM no corpo), testes (unit/integração/e2e), Bruno e docs/memory

## Versão 0.35.0.0

**Fase 2** — tratamento de URLs (linking entre notícias).

- [x] Novo passo determinístico antes da sanitização: identifica `<a href>` no conteúdo, busca a `url_original` exata nos registros de notícias; se a notícia existe, reescreve o `href` para `CLIENT_URL/articles/{id}`; senão nada acontece
    - Roda **antes** da sanitização (o link interno precisa existir antes do bluemonday; o bluemonday já permite links, então não muda de política)
    - Novo pacote `services/urltreatment` (parse via `golang.org/x/net/html`, lookup por `url_original`). Usa `CLIENT_URL` e `URLS_TREATMENT_VERBOSE_MODE`
    - Escopo só `<a href>` (não texto solto/`<img>`); sem pré-filtro por source (lookup direto por `url_original`, que é índice único)
    - Best-effort (erro de banco → corpo original); pulado sem `CLIENT_URL`. Resolver DB-agnóstico injetado (1 transação de leitura por artigo). Integrado na CRON e no dry-run `POST /articles/treatment`

## Versão 0.35.1.0

- [x] Enriquecer os logs do tratamento de URLs (`URLS_TREATMENT_VERBOSE_MODE`) para dar pra acompanhar o passo pelo terminal
    - Antes: só um contador, e nada quando 0 links casavam. Agora: marca START com nº de `<a>` achados, lista cada href encontrado, cada reescrita (`de -> para`) e END com o total (inclusive o caso "0 são nossos")
    - Vale para a CRON e para o dry-run `POST /articles/treatment` (ambos passam o flag ao `urltreatment.Treat`)

## Versão 0.36.0.0

**Fase 3** — embeds conhecidos (com foco em segurança).

- [x] `<iframe>` de YouTube/Twitch liberados via allowlist apertada de `src`; `<script>` nunca é permitido
    - Política do `bluemonday` (`services/sanitize`): `AllowAttrs("src").Matching(embedSrc).OnElements("iframe")` + atributos de exibição (`width`/`height`/`allowfullscreen`/`allow`/`title`/`loading`). Iframe fora da allowlist é desembrulhado. Verificado empiricamente
- [x] Instagram (e similares baseados em `<script>`) convertidos para `<a href={url_da_postagem}>{url_reduzida}</a>` (PROJECT.md, "Whitelist de sanatização")
    - Novo pacote `services/embedtreatment` (puro, `x/net/html`), roda **antes** do sanitize; acha `<blockquote class="instagram-media">`, lê o `data-instgrm-permalink` (ou o `<a>` interno) e substitui por um link com a URL limpa (sem query). Best-effort. Integrado na CRON e no dry-run

## Versão 0.36.0.1

- [x] Criar `.claude/memory/api-integration.md` (doc de contratos/filosofias da API para o projeto do web client, para enviar junto com o `endpoints.md`)

## Versão 0.36.1.0

- [x] Reescrever o `parent` do iframe do Twitch para o host do `CLIENT_URL` no tratamento de embeds
    - O player do Twitch só reproduz quando o `parent` bate com o domínio que renderiza; o RSS traz o domínio da fonte (ou nada), então sem isso o embed renderizava mas não tocava (apontado pelo Claude do client)
    - `services/embedtreatment` passou a receber o `CLIENT_URL`, deriva o host (sem porta) e força `parent=<host>` no `src` do Twitch (`player.twitch.tv`/`clips.twitch.tv`), preservando os demais params. Pulado sem `CLIENT_URL`. YouTube não precisa

## Versão 0.36.4.0

- [x] Triagem da camada 2 do julgamento por overlap de keywords, pra desacoplar o custo de IA do total de feeds (300 users × 5 feeds estouraria a IA)
    - Camada 1 (`FindCandidateFeedsByKeywords`) passou a devolver o **overlap** (COUNT + GROUP BY). O controller retorna `FeedCandidate{Feed, OverlapCount}`
    - Triagem no `Evaluator`: `ratio ≥ JUDGEMENT_AUTOASSOCIATE_RATIO` (0.30) → auto-associa sem IA; overlap `< JUDGEMENT_MIN_MATCHES` (2, ou seja 1) → descarta sem IA; resto → IA (borderline)
    - Filosofia de risco: falso positivo (auto-associar) é barato, falso negativo (descartar) é grave → descarte conservador (só 1 keyword). Ajuste fino de verdade = pesar keyword por especificidade/embeddings (futuro)
    - Dry-run `POST /articles/judgement` mostra `overlap` + `decision` (auto_associated|judged|discarded) por candidato — ótimo pra calibrar

## Versão 0.36.3.0

- [x] `DISCOVERY_MAX_ARTICLES`: cap de quantas notícias uma varredura da CRON entrega ao pipeline (`-1` = sem cap, padrão/produção)
    - Freio bruto de rate limit para testar com Groq em dev sem estourar o TPM quando uma fonte traz muitas notícias de uma vez (ex.: Wikimetal com ~20). Corta o lote combinado antes do dedup, no `DiscoveryRunner`

## Versão 0.36.2.0

Redução de tokens de IA + correção de recall no julgamento (a camada 1 deixava de trazer feeds que encaixariam).

- [x] Camada 1 (recall): o keyworder passa a emitir também termos **genéricos** (gênero/categoria) junto dos específicos, para que feeds genéricos casem notícias de entidades específicas no overlap exato de keywords
    - Limite de keywords da notícia subiu de 20 para **30** (`ArticleKeywordsMax`) para dar espaço aos genéricos; prompt `article_keywords` reescrito com a regra específico+genérico e exemplos
- [x] Keywords recebem **texto puro** (HTML removido via `sanitize.PlainText`) em vez do HTML — corta tokens de URL de imagem/iframe/href sem perder sinal. Conteúdo gravado continua HTML
- [x] Camada 2 (precisão + tokens): o julgamento **deixa de enviar o corpo** da notícia; julga por `título + keywords` (`ai.Judger` sem o param `content`, prompt reescrito). Isso mata o multiplicador (corpo × nº de candidatos) que estourava o rate limit do Groq
    - O `JUDGEMENT_THRESHOLD` precisa ser **re-calibrado** manualmente no dry-run (a escala dos scores muda sem o corpo)
    - Dry-run `POST /articles/judgement` não aceita mais `article.content` (só `title` + `keywords`)

## Versão 0.37.0.0

- [x] Postgres 18 ao invés de SQLite

O SQLite sempre foi o desvio: a stack (`CLAUDE.md`) já declarava PostgreSQL + pgadmin4, e a pasta
`pgadmin4/` existe vazia desde então. O contrato da API **não muda** (`status` nunca foi filtro exposto e
o `int` já era convertido para bool nos schemas de resposta), então client e Bruno seguem intactos.

Decisões fechadas:

- Dados atuais são **descartáveis**: banco novo do zero, sem ETL. `task sdfull` repovoa.
- Migrações **recomeçam do zero**: os 13 arquivos SQLite saem e entra uma migração inicial única,
  PG-native. Some a dança de recriar tabela do `sources_partial_unique_index` (índice parcial é nativo).
- Driver **`pgx/v5/stdlib` via `database/sql`** (`sql_package: "database/sql"` no sqlc): mantém
  `*sql.DB`, `sql.NullString`/`sql.NullTime`, e com isso `transaction.go`, a interface `Querier` e os
  mocks ficam intactos. O pgx nativo (`pgtype.*`) fica como opção futura: é uma linha no `sqlc.yaml`.
- **sqlc continua** — Postgres é o engine mais forte dele; sair para um ORM contrariaria a convenção de
  tipagem forte e SQL explícito.
- Tipos: `status`/`is_read`/`app_status` → `BOOLEAN`; datas → `TIMESTAMPTZ`; `keywords` → `JSONB` (+ GIN).
  `id` **continua `TEXT`** — UUID nativo fica para versão própria (ripple em todo model/teste).
- Docker vira obrigatório para o **banco**; a API segue rodando local (`task ls`) ou no compose
  (`task ds`). O que morre é o banco local sem Docker.

Executada em 3 etapas (mesma versão; só está concluída no fim da Etapa 3):

- [x] **Etapa 1 — a aplicação roda.** Compose com `postgres:18-alpine` + pgadmin4 (volume + healthcheck),
      migração inicial única, `schema.sql`, `sqlc.yaml` (engine postgresql), queries reescritas (`?` → `$N`,
      `json_each` → `jsonb_array_elements_text`), regeneração do sqlc, `main.go` (driver, DSN, fora os
      pragmas WAL/busy_timeout, goose dialeto `postgres`), `sqlite_errors.go` → `postgres_errors.go`
      (`*pgconn.PgError`, SQLSTATE `23505`), ripple `int` → `bool` (19 sites em 11 arquivos).
      Ao fim: API sobe e é testável pelo Bruno. `task ta` fica vermelho — planejado.
- [x] **Etapa 2 — os testes voltam.** `tests/utils/db.go` sobe PG via `testcontainers-go`; migração roda
      uma vez num database template e cada teste clona via `CREATE DATABASE ... TEMPLATE` (isolamento
      total sem pagar a migração por teste); container morre no fim da suíte. Fixtures e mocks acompanham
      o `bool`. Reescrever a regra de teste do `conventions.md`: "instância em memória" → database
      temporário por teste, descartado ao fim. `task ta` passa a exigir Docker rodando. `test_manager` roda aqui.
- [x] **Etapa 3 — seed e documentação.** `cmd/seed`, CLAUDE.md, `rules/structure.md`, `versions/`, `memory/`.
      (O grosso do `cmd/seed` — driver e dialeto — foi puxado para a Etapa 1: sem ele o `task sdfull`
      não rodava e não havia como verificar a Etapa 1.)
- [x] **Etapa 4 — UTC no sistema de tipos** (entregue na **0.37.1.0**, ver `versions/`). Fecha a dívida
      aberta na Etapa 1: o `time.Local = time.UTC` foi **removido** e a garantia passou para o tipo
      `utctime.Time`/`NullTime` (`services/utctime`), plugado via `overrides` do `sqlc.yaml` nas
      colunas `timestamptz`. Decidido fazer porque a convenção passa a depender do compilador em vez
      da disciplina de lembrar a linha em cada novo entrypoint (o `cmd/seed` já tinha precisado dela).

Fora do escopo desta versão (candidatos naturais logo depois, viabilizados por ela): filtragens e
paginações que hoje acontecem em Go em vez de SQL.

## Versão 0.37.3.0

- [x] Revisão
    - Achado principal: o índice GIN de `feeds.keywords` criado na 0.37 **nunca era usado** —
      a camada 1 do julgamento não tinha operador que o índice servisse (428ms → 22ms em 60k feeds).
    - Também: `recover` ausente (panic derrubava a app) e falha de banco virando 401 (deslogava todos).
    - Deliberadamente **fora** do escopo: filtragem/paginação em Go, que é a 0.39 abaixo.

## Versão 0.38.0.0

- [ ] Fazer com que as filtragens sejam feitas através de SQL ao invés de Go
- [ ] Fazer com que as paginações sejam feitas através de SQL ao invés de Go

## Versão 0.39.0.0

- [ ] Sugestão de keywords
    -   1. se não houver nenhuma selecionada ou se o passo 2 retornou vazio, sempre trazer as mais "populares" dentre as notícias (quais keywords aparece mais vezes nas notícias)
    -   2. se houver alguma já selecionada, trazer as mais próximas de acordo com o que a pessoa selecionou (quais keywords aparecem junto com as que o usuário já selecionou)
    - Nunca estar sem sugestões (se a saída do passo 2 for vazio, voltar ao passo 1)

## Futuro

Planos que não serão aplicados agora. Use para entender evolução futura do código:

- Aumentar o verbose mode (+++++++++++++++++++++ logs)
- TlDraw do banco de dados
- Preparar ambiente staging + production
- Denunciar conteúdo
- Multi feed
- Editar informações básicas do usuário
- Cascade de tabelas
- Limpar o main
- Transportar a iniciação de endpoints para outro lugar, health para /api
- Criar uma rota para aceitar sugestões de fontes de notícias
- Tratar notícias que se auto-atualizam
    - Precisa ser feito depois do resumo de notícias pois o início do cache mora lá, e isso depende de trazer o Redis pro stack
    - O Claude falou que gofeed consegue fazer essa detecção também que pode servir como uma outra solução para esses casos
        - Provavelmente já serve como primeira camada confiável o suficiente. Podemos testar e verificar se precisa de uma segunda camada.
    - A segunda camada seria:
        - Talvez dá pra fazer a IA descobrir se a notícia está marcada como "em atualização" para flaggear no banco de dados
        - A partir daquele momento, outra tarefa da CRON vai atrás apenas das notícias em atualização para ir atualizando ela de tempos em tempos, passando por todo o tratamento toda vez até que sua flag se torne "false"
- Logger e observabilidade
- Swagger
- Associação de notícias (como uma notícia se liga à outra?)
- Aceitar URLs de embedding contidas na notícia para embeddar na nossa própria
- Melhorar o tratamento para também atribuir classes e elementos HTML mais personalizados
- Usuário administrador (lembrete: requisições do admin não são afetadas pelo `system.app_status` estando em `false`)
- Opções de usuário administrador (desativar X, habilitar Y)
- Endpoint para soft remove de usuário + testes de cascade (métodos já estão ok)
- Compartilhamento de notícias
- Linkar uma notícia à outra (através de keywords?)
- Múltiplos modelos de IA (Claude, GPT)
- LangChain para orquestração complexa
- Resumo de notícias longas
- Resumo diário de notícias (newsletter)
- Notificações em tempo real
- Sistema de feedback de notícias
- Descoberta automática de novas RSS feeds
- Regras de administradores
- SSE ou Websocket
    - Novas notícias chegaram
    - Rota `GET base_url/v1/feeds/check-for-new-articles`
    - Notificações
- Machine Learning: aprender com leitura/descarte do usuário
- CI/CD
- Backup
- Resumo de notícias
    - Acrescentar cache de resumo via redis
    - Tradução já existe mas precisa de cache também

## Escalabilidade futura: fila distribuída + workers (pós-Postgres)

Sessão própria para amadurecer quando a hora chegar. **Depende do Postgres entrar no stack** — é ele que
remove o teto de single-writer do SQLite e habilita escrita concorrente de verdade e múltiplas réplicas.

Contexto: a 0.27 já paralelizou descoberta/tratamento/julgamento com um **worker pool in-process**
(`DISCOVERY_CONCURRENCY`). Isso é o **caso de 1 nó** de um modelo maior: **produtor + fila + pool de
workers**. O pipeline por artigo (`discovery.processOne`) foi mantido sem conhecer quem o despacha,
justamente para essa evolução ser uma **troca de dispatcher, não uma reescrita**.

Ideia a avaliar no futuro:

- A descoberta _produz_ jobs (1 artigo = 1 job) numa fila persistente (tabela no Postgres via
  `river`/`asynq`, ou Redis/NATS).
- **M réplicas de container**, cada uma com N workers, _consomem_ da mesma fila → escala horizontal.
- A fila absorve picos de volume (ex.: evento de grande repercussão); os workers drenam no ritmo que
  o provedor de IA aguenta.
- O teto real nunca é o Go — é o **rate limit do provedor de IA**. A fila + workers dá a capacidade de
  adicionar throughput horizontalmente até esse limite (ou mais chaves/quota em paralelo).

Não programar agora: sem Postgres/Redis no stack, seria complexidade sem retorno. Só está **arquivado**
aqui para não perdermos o desenho quando o Postgres chegar.
