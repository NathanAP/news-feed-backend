# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.39.0.1

## Versão 0.37.3.0

- [x] Revisão
    - Achado principal: o índice GIN de `feeds.keywords` criado na 0.37 **nunca era usado** —
      a camada 1 do julgamento não tinha operador que o índice servisse (428ms → 22ms em 60k feeds).
    - Também: `recover` ausente (panic derrubava a app) e falha de banco virando 401 (deslogava todos).
    - Deliberadamente **fora** do escopo: filtragem/paginação em Go, que é a 0.39 abaixo.

## Versão 0.38.0.0

Dívida deixada em aberto pela 0.37: as listagens carregavam a tabela ativa inteira para o Go, que
filtrava com `strings.Contains` e fatiava a página em memória. Contrato da API **inalterado** — o
client não precisa de nada.

- [x] Fazer com que as filtragens sejam feitas através de SQL ao invés de Go
    - `GET /v1/articles` (`url`), `GET /v1/sources` (`url` + `name`), `GET /v1/feeds` (`name`),
      `GET /v1/feeds/{id}/articles` (`is_read`, `period_starting_at`, `period_ending_at`)
    - Filtro opcional = `sqlc.narg` (`NULL` = não aplicado). Substring via `strpos(lower(a), lower(b)) > 0`
      e **não** `ILIKE '%...%'`: preserva a semântica do `strings.Contains` anterior e impede que `%`/`_`
      do usuário virem curinga (tem teste de integração pra isso)
- [x] Fazer com que as paginações sejam feitas através de SQL ao invés de Go
    - `LIMIT/OFFSET` na query + uma query `Count*` irmã por listagem (mesmos filtros). O `total_count`
      passa a vir do banco em vez de `len()`
    - Duas queries em vez de `COUNT(*) OVER()`: numa página fora do range não há linha pra carregar o
      total, e a regra do `PROJECT.md` ("página 5 de 4" devolve `actual_page: 5`, `total_pages: 4`) exige o total
    - `pagination.Paginate` (fatiava em memória) morreu; entrou `BuildResponse` + `Params.Limit/Offset`

Decisões tomadas durante a execução:

- **Desempate de ordenação** (bug latente que o `LIMIT/OFFSET` expôs): todas as listagens passaram a
  ordenar por `created_at DESC, id DESC`. Sem isso, notícias do mesmo lote da CRON empatam no
  `created_at` e uma linha pode repetir numa página e sumir de outra. O `id` é UUIDv7, então o
  desempate é determinístico e ainda cronológico. Verificado: removendo o `id` do `ORDER BY`, o teste
  `PaginationIsStableAcrossTiedTimestamps` falha
- **`ListAll` separado do `List`**: a CRON (varredura de sources) e o `cmd/seed` precisam de **todas**
  as linhas, não de uma página. Ganharam queries próprias sem `LIMIT` em vez de um `page_size` grande
  o suficiente "pra caber tudo", que é um bug esperando o volume crescer
- **Filtro `name` em sources**: o exemplo do `conventions.md` prometia um filtro que nunca existiu
  (o campo existe desde a 0.30). Implementado junto, já que a query estava sendo reescrita
- Papel dos testes mudou: o unitário agora prova que o handler **repassa** o filtro certo (e que
  `is_read=false` não vira "sem filtro"); a filtragem real é provada em integração contra o Postgres

## Versão 0.39.0.0

- [x] Sugestão de keywords
    - Endpoint novo: `GET /v1/feeds/keyword-suggestions?keywords=metallica,rock&limit=10` (sob `feeds`
      porque o uso é montar feed; o dado vem do acervo global de artigos, então usa `articleCtrl`).
      Resposta `{ strategy, suggestions: [{ keyword, count }] }` — a `strategy` (`related`|`popular`)
      é ecoada pro client rotular e pro fallback ser observável.
    -   1. `popular`: se nada selecionado (ou se o passo 2 voltou vazio), as keywords mais frequentes
           nas notícias, dentro de uma janela de tempo (`KEYWORD_SUGGESTIONS_WINDOW_DAYS`, padrão 30,
           `-1` desliga). Janela por produto ("popular agora") **e** por custo (é agregação de tabela
           inteira). Query `SuggestPopularKeywords`.
    -   2. `related`: se há keywords selecionadas, as que mais co-ocorrem com elas (mesmas notícias).
           Não é janelada (relatedness é topical, não temporal). Query `SuggestRelatedKeywords`, cujo
           `?|` alcança o novo índice GIN de `articles.keywords`.
    - Nunca fica sem sugestão: passo 2 vazio → cai no passo 1. A keyword já escolhida nunca é sugerida
      de volta (nas duas estratégias).
    - Não paginado (indicador ranqueado) → `limit` simples (padrão 10, máx 50). Isenção registrada no
      `conventions.md`.
    - Ranking = contagem crua **de propósito**: o objetivo é empurrar genéricos (o que faz a notícia
      bater na camada 1), não filtrá-los. Ranking mais fino (lift/especificidade) fica pra quando
      houver volume real pra calibrar.
    - Migração: índice GIN `idx_articles_keywords` (espelha o de `feeds.keywords`). Verificado por
      `EXPLAIN` que o operador `?|` **alcança** o índice (Bitmap Index Scan) — não é peso morto como o
      caso da 0.37.3; o planner usa quando a seletividade compensa.
    - **Ponto em aberto deixado explícito**: `FeedKeywordsMax` (20) e os knobs de triagem do julgamento
      são as alavancas reais da qualidade do match — recalibrar com dado real quando houver volume.
      Mantido 20 nesta versão por não ter dado pra justificar um número menor.

## Versão 0.39.0.1

- [x] Keywords nos artigos de exemplo do seed (`cmd/seed/examples.json`)
    - Os 20 artigos de exemplo não tinham `keywords`, então `task sda` populava tudo com `[]` e a
      sugestão de keywords respondia vazio em dev (não era bug, era pool sem keywords).
    - Cada artigo ganhou keywords em inglês minúsculo (específicos + genéricos, mín. 5), com overlap
      com os feeds de exemplo — habilita também o julgamento em dev. Registros já no banco foram
      atualizados no lugar. Tier `docs` (dado de seed, sem mudança de comportamento da app).

## Versão 0.40.0.0

- [ ] Criar o modo administrador
    - Escrevi no PROJECT.md como isso vai funcionar
    - Vamos precisar de uma migração
    - O usuário dev pode ser marcado diretamente como um administrador ao ser criado.
    - Lembrete: requisições do administrador não são afetadas pelo `system.app_status` quando estiver em `false`

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
