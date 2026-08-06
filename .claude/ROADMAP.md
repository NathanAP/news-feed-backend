# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.48.0.0

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

- [x] Criar o modo administrador
    - Migração: `users.admin BOOLEAN NOT NULL DEFAULT FALSE` (fail-closed, sem índice — a flag só é
      lida por PK). A promoção tem query própria (`SetUserAdmin`) em vez de virar parâmetro do
      `CreateUser`: assim o fluxo de login fica **estruturalmente incapaz** de criar um administrador.
    - **Autorização lê o banco, não o token.** O claim `admin` existe (e o `/users/me` o devolve), mas
      só como dica de client — igual ao `language_to_translate`. Motivo: se a decisão viesse do token,
      revogar acesso só valeria quando ele expirasse. Custo: 1 query, e só nas rotas admin.
      Token antigo decodifica `admin` como `false`, então nenhuma sessão precisou ser invalidada.
    - `middlewares/admin.go`: `AdminResolver` (uma definição de "é admin" para a app inteira) +
      `RequireAdmin`. Não-admin → 403; falha de banco → 500, nunca 403.
    - Bypass de manutenção contido no branch de indisponibilidade: com a app no ar o caminho é idêntico
      ao anterior (nenhuma query a mais). Fail-closed — token ruim, sessão encerrada ou erro de banco
      viram 503.
    - Usuário dev nasce administrador; um usuário de seed anterior é promovido no lugar pelo `task sud`.

## Versão 0.41.0.0

- [x] Correção docker-compose e pgadmin
    - **Split do compose**: `docker-compose.yaml` (base) + `docker-compose.override.yaml` (dev),
      mesclados automaticamente pelo Compose. O pgAdmin saiu pro override — produção nunca o sobe por
      **ausência** (o `-f` explícito de prod desliga o auto-override), não por alguém lembrar de removê-lo.
    - **Bind em `127.0.0.1`** em todas as portas publicadas (api, postgres, pgadmin): deixam de ser
      alcançáveis pela LAN. Túnel (ngrok/SSH) continua funcionando porque roda no host e alcança o loopback.
    - Motivo original: o pgAdmin abre direto no dashboard (`SERVER_MODE=False`, sem login) — ok em dev
      amarrado ao loopback, inaceitável exposto. A correção garante que ele não vaze.

## Versão 0.42.0.0

- [x] Preparações production / staging
    - **Modelo de deploy invertido**: em vez de "pull do código + build no servidor" (o que estava no
      `PROJECT.md`), o CI constrói a imagem **uma vez**, empurra pro ECR, e o servidor só dá `pull` da tag
      e sobe. O servidor nunca vê código nem builda. Isso torna "apagar arquivos de dev" desnecessário: a
      imagem final (multi-stage) já é só o binário, e as rotas de dev são barradas por `ENVIRONMENT` em
      runtime.
    - **Compose multi-ambiente**: o `postgres` saiu da base pra cada overlay (o Compose não remove um
      serviço da base num override, e prod pode usar RDS em vez de container). Base = só `api` com imagem
      parametrizada (`API_IMAGE`). Dev = postgres+pgadmin+build. `docker-compose.staging.yaml` = postgres
      container. `docker-compose.production.yaml` = RDS por padrão (postgres comentado, removível).
    - **Taskfile**: `staging-up/down/logs` e `prod-up/down/logs` com `-f` explícito + `--env-file`.
    - **Templates de env**: `.env.staging.example` e `.env.production.example` (só nomes, segredos
      marcados). `.gitignore` versiona os `.example`, ignora os reais.
    - **CI/CD (GitHub Actions)**: `ci.yml` (fmt+vet+test, sempre ativo) e `deploy.yml`
      (build→ECR→deploy EC2 via SSH+OIDC nos branches `staging`/`production`), inerte até
      `DEPLOY_ENABLED=true`. Secrets por OIDC/Environment, nunca no YAML. Doc em `.github/workflows/README.md`.
    - **Postgres de produção (RDS vs container)**: decisão adiada de propósito — o app só enxerga uma
      connection string, então trocar é config, não rebuild. Backup fica na 0.43.
- [x] Resolver problema de ter vários workers + CRON
    - Resolvido **por disciplina operacional**: produção roda **instância única** (documentado no
      `docker-compose.production.yaml`, nos templates de env e no README do CI). O fix de código
      (lock distribuído / fila) continua arquivado na seção de escalabilidade futura — só vale a pena
      quando precisar de mais de uma réplica.

## Versão 0.43.0.0

- [x] Na CRON, barrar usuários inativos de receber notícias
    - Migração `users.last_active_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`. O `ADD COLUMN`
      já faz o backfill (toda linha existente recebe a hora da migração), então ninguém vira inativo
      no deploy. Novo usuário nasce com `now()` (acabou de logar).
    - **Sinal de atividade, não de login**: `last_active_at` é escrito no `/auth/refresh` (o batimento
      ~horário), no login e no `dev-login`. Diferente do `last_login_at`, que só muda no login completo
      do Google (~30 dias) e marcaria um usuário diário como inativo.
    - Filtro na **camada 1** (`FindCandidateFeedsByKeywords`): um `JOIN users` com predicado de
      atividade dentro da MESMA query — o custo de IA cai porque feeds de inativos nem viram candidatos.
      `DAYS_UNTIL_USER_IS_INACTIVE` controla a janela; `-1` desliga (todo mundo ativo). Threaded do
      `main.go` até o processor da CRON e o dry-run de julgamento.
    - Aceito de propósito: julgamento não é retroativo, então quem volta perdeu as notícias do período
      inativo (documentado no `PROJECT.md`).
    - Testes: filtro pega inativo / mantém ativo / `-1` ignora (integração contra o Postgres); e o
      refresh realmente carimba `last_active_at` (o elo sem o qual o filtro se auto-sabota).

## Versão 0.44.0.0

- [x] Entender como a CRON pode ser organizada — e corrigir dois furos de robustez que a análise achou
    - **"Uma CRON por worker?"** Não, nunca. A CRON é o **gatilho/produtor** (um tick = uma varredura);
      workers são **consumidores** (hoje o pool de `DISCOVERY_CONCURRENCY` num processo; amanhã, réplicas).
      Uma CRON por worker = varredura duplicada = descoberta e IA duplicadas. Ao escalar pra M réplicas o
      alvo é **uma varredura só**: scheduler dedicado (uma réplica com `RSS_FEED_CRON_ACTIVE=true`) agora,
      advisory lock do Postgres quando precisar de réplicas idênticas. A fila distribuída segue arquivada.
    - **"E se demorar demais?"** Toda chamada externa já tem timeout (RSS 30s, Gemini 60s, Ollama/Groq
      120s), então um sweep é longo mas **finito** — timeout por sweep foi descartado (só cortaria sweep
      legítimo). O que faltava eram dois guards no scheduler:
        - `SkipIfStillRunning`: sweep que passa do intervalo não gera um segundo concorrente. **Não é
          correção de integridade** (o índice único de `url_original` já colapsa duplicata em uma linha e
          o perdedor sai antes do julgamento) — é **economia de IA** (dois sweeps nomeando keywords do
          mesmo artigo).
        - `Recover`: panic numa varredura vira log, não derruba o processo. A CRON é in-process, então sem
          isso um sweep ruim levava a API junto — o `recover.New()` do Fiber (0.37.3) só cobre HTTP.

## Versão 0.45.0.0

- [x] Criar a tabela de URLs externas de notícias
    - Tabela `article_outbound_links` (id, article_id FK, href indexado; sem status/removed_at). Cascade
      hard-delete por article_id (dormente hoje, pronta no código). Migração sem backfill.
- [x] Alterações nas notícias para condizer com as novas regras
    - Serviço `services/outboundlinks`: `Assign` (grava ids no corpo na descoberta) + `Resolve` (troca de
      volta na leitura). Links internos guardados como token `{CLIENT_URL}/articles/{id}`, externos literais.
    - Swap centralizado nos endpoints de leitura (`GET /articles/{id}`, `GET /articles`,
      `GET /feeds/{id}/articles`, `/translate`) — batch, sem N+1. Invariante: id/token nunca passam pelo
      bluemonday (Assign pós-sanitize, Resolve pré-qualquer-re-sanitize).
- [x] Alterar o fluxo de tratamento de notícias para integrar os novos passos de banco de dados
    - `persist` do processor virou uma transação: gravação → associação+reorganização → alteração
      (retarget retroativo). Seed (`cmd/seed/dev_articles.go`) espelha o novo formato.
    - Decisão aceita: custo de leitura maior (parse/rewrite por notícia) em troca da retroatividade; sem
      cache até o Redis. Detalhes no version file `20260804120000_0.45.0.0.md`.

## Versão 0.46.0.0

- [x] Atualização de dependências / bibliotecas
    - Motivada por **segurança**, não por atraso: o `govulncheck` apontava 3 vulnerabilidades com
      símbolo alcançável; depois desta versão são **zero**. A que importava era o loop infinito do
      `golang.org/x/text` (GO-2026-5970), alcançável via corpo de RSS remoto em `services/rss`.
    - Toolchain `go 1.26.4` → `1.26.5` no `go.mod` e só: o Dockerfile (`golang:1.26-alpine`) e o CI
      (`go-version-file: go.mod`) herdam por construção.
    - Diretas: `gofeed` 1.4.0, `goose` 3.27.3, `go-retry` 0.4.0, `x/net` 0.57.0, `genai` 1.66.0,
      `lib/pq` 1.12.3. Indiretas de segurança: `x/text` 0.40.0, `grpc` 1.83.0.
    - **Remoção do `lib/pq` tentada e revertida**: o `sqlc.slice()` é quebrado para o engine
      postgresql no sqlc v1.31.1 (perde o marcador `/*SLICE:*/` e gera placeholder `?` de MySQL).
      Compila, passa no vet e passa em qualquer teste com **um** id — só quebra com dois. Detalhes e
      proibição explícita de repetir a tentativa no comentário da query e no version file.
- [x] Revisão (concluída em 6 levas: 0.46.1.0 a 0.46.7.0)
    - **0.46.1.0 — leva 1 (0.45, outboundlinks)**: `toStored` casava domínio sósia (`HasPrefix` sem
      fronteira de URL: `https://algo.com.br/x` virava token interno — dormente até o `CLIENT_URL`
      mudar); e o "pre-remove cleanup" que o comentário da 0.45 prometia nunca tinha sido ligado,
      deixando links para uma página que responde 404. Batch de leitura verificado sem N+1.
    - **0.46.2.0 — leva 2 (infra de teste)**: `task test-all` eram três invocações de `go test`
      encadeadas, e o reaper destrói o container quando a invocação que o criou termina. A fase
      seguinte se anexava a um container já sentenciado e morria no meio com `unexpected EOF`. Agora é
      uma invocação só, igual ao CI — a divergência entre local e CI *era* o bug. Priorizado à frente
      da 0.38 porque teste instável corrói a confiança na revisão inteira.
    - **0.46.3.0 — leva 3 (0.38, filtros/paginação em SQL)**: `Offset()` estourava int32 e virava
      OFFSET negativo, que o Postgres rejeita — `?page=200000000` devolvia 500 em toda listagem
      paginada. Agora satura em `MaxInt32`, preservando a semântica de página fora do intervalo.
      Verificado sem ressalva: os 4 pares `List*`/`Count*` têm filtros idênticos, e os filtros de
      data normalizam para UTC.
    - **0.46.4.0 — leva 4 (0.39 + alcance na 0.37.3)**: os índices GIN de keywords existiam, estavam
      corretos e **não eram escolhidos pelo planner**. O `?|` estava certo, mas o lado direito era um
      `ARRAY(SELECT ...)` — um `InitPlan` opaco, sem estimativa de seletividade, então o planner
      precificava o índice acima do seq scan. Alcançar índice exige duas coisas: operador certo **e**
      operando estimável. Corrigido para `text[]` nas duas queries (sugestão de keywords e camada 1 do
      julgamento). Teste novo `tests/integration/queryplans/` afirma sobre o `EXPLAIN`, porque essa
      falha é invisível a teste de resultado e já ocorreu duas vezes.
    - **0.46.5.0 — leva 5 (0.40 + 0.43)**: a superfície de autorização passou limpa (banco e não token,
      falha de banco vira 500 e não 403, `adminRoute` sem aliasing de slice, dry-runs de IA só existem
      em development). O achado veio de um vizinho: `GET /v1/sources/rss-discovery` recebia um
      `&http.Client{}` **sem timeout** para buscar uma URL do usuário, e o `rss.Discover` faz ~15
      saídas externas por chamada (13 delas concorrentes). Corrigido com timeout por requisição (30s,
      o mesmo do irmão) e orçamento total de 45s no handler.
        - **Em aberto, aguardando decisão**: (1) bloquear destinos privados nessa rota (SSRF —
          loopback, link-local, RFC1918); (2) se ela deve virar admin-only, já que o irmão
          `article-discovery` é, ou permanecer `[AUTH]` por causa da futura rota de sugestão de fontes.
    - **0.46.6.0 — as duas decisões acima, tomadas**: (1) pacote novo `services/safehttp`, cujo dialer
      recusa destinos não públicos. O controle fica no **dialer** e não na validação da URL, porque um
      hostname pode resolver para IP privado e um redirect escapa de qualquer checagem feita antes da
      primeira requisição; o IP aprovado é discado direto, para não reabrir janela de DNS rebinding.
      (2) `rss-discovery` virou administrator-only, como o irmão — estava aberta por conveniência do
      Bruno, não por intenção de produto. Coberto por 403 em e2e.
    - **0.46.7.0 — leva 6 (0.41/0.42), última do escopo**: o container rodava como **root** (sem
      diretiva `USER` no Dockerfile). Nada ali precisa de root — a app escuta porta não privilegiada e
      não escreve em disco. Corrigido com `appuser` uid 10001, verificado na imagem real. Também: dois
      comentários de compose mandavam usar `docker-compose.prod.yaml`, arquivo que não existe.
      Confirmados como corretos: deploy realmente inerte, portas todas em loopback, pgAdmin ausente
      fora de dev por construção, segredos só como `CHANGE_ME`, e o CI não cai na armadilha do
      `gofmt -l` (que retorna 0 mesmo com arquivo desformatado).
    - Escopo: 0.38 → 0.45 (a última revisão foi a 0.37.3, em 17/07 — ~8.800 linhas em 146 arquivos
      desde então). Ordem por risco: 0.45 (outboundlinks, está no caminho de leitura de toda notícia)
      → 0.38 (filtros/paginação em SQL, sincronia entre query de dados e query de `Count`) → 0.39
      (agregação de tabela inteira) → 0.40+0.43 (autorização admin e filtro de inativo na camada 1)
      → 0.41/0.42 (compose e CI, que nenhum teste cobre).
    - Dependências vêm **antes** da revisão de propósito: a revisão deve ler o código como ele
      vai rodar de fato, e uma quebra de dependência não pode se misturar a uma correção de revisão.

## Versão 0.47.0.0

- [x] Migração Fiber v2 → v3
    - Estamos no v2.52.14; o v3 já está em v3.4.0. É a única dependência uma major inteira atrás, e o
      v2 entra em manutenção com o v3 estável. **Sem CVE aberto hoje** — é dívida crescente, não incêndio.
    - Superfície real medida no nosso código (conferida no fonte do v3.4.0, não na doc): ~58 pontos de
      edição, dos quais 47 são a troca de assinatura `*fiber.Ctx` → `fiber.Ctx` (o `Ctx` virou interface).
      Os outros: 3 `c.Redirect(url, status)` → `c.Redirect().Status(s).To(url)` (fluxo de login OAuth),
      1 config de cors (`AllowMethods` string → `[]string`) e 7 `app.Test(req, timeout)` →
      `fiber.TestConfig{Timeout:}`.
    - **O que parecia quebrar e não quebra**: a doc oficial diz que `Context()` foi removido, mas no
      fonte ele continua existindo — só estreitou o retorno de `*fasthttp.RequestCtx` para
      `context.Context` (o antigo virou `RequestCtx()`). Como nossos 88 usos são todos
      `runTx(c.Context(), ...)`, tratando o valor como contexto e nunca como fasthttp cru, compilam sem
      alteração. Idem `app.Test(req)` sem timeout (314 sites — o config é variádico) e `recover.New()`.
    - Fazer **antes** de qualquer coisa de SSE/Websocket: o v3 traz `middleware/sse`, e escrever SSE na
      mão no v2 primeiro seria trabalho jogado fora. Também traz `timeout`, `healthcheck` e `paginate`,
      que hoje temos em versão própria.
    - Viável porque a suíte é grande (321 `app.Test`, integração e e2e). Troca de framework web sem essa
      cobertura seria temerária; com ela é verificável.
    - **Feita.** A estimativa acertou os ~58 pontos e os 90 `Context()` passaram intactos (auditada a
      forma de uso, não só a compilação). A cauda apareceu, como avisado: `DisableStartupMessage` saiu
      do `Config` (32 sites), o registro de rota mudou para `(path, handler, ...handlers)` (125 sites)
      e `BodyParser` virou `Bind().Body()` (12). O ponto perigoso é o registro: `Add` preserva a ordem,
      mas assumir o contrário rodaria a autenticação **depois** do handler, compilando. Centralizado em
      `addRoute`/`testutils.AddRoute` com a explicação escrita, e auditado por forma. Verificado também
      com boot real: `/health` 200 e rotas autenticadas 401.
    - Fora do escopo, corrigido junto: `PROJECT_VERSION` estava três versões atrás nos `.env.*.example`
      (o `/health` publicava versão errada) e o `services/safehttp` não fora registrado no `structure.md`.
    - **Propostas, não feitas**: injetar a versão em build time (`-ldflags -X`) em vez de manter
      `PROJECT_VERSION` em quatro arquivos — a sincronização manual é o motivo de ter derivado; e adotar
      `group.Use(...)` no lugar dos helpers de cadeia, que é o idioma do v3 (redesenho de wiring, não
      cabia dentro da migração).

## Versão 0.48.0.0

- [x] Auditoria de coerência dos comentários
    - Ataca o padrão que a 0.46 mais encontrou: documentação afirmando garantia que o código não dava
      (4 casos). Sem mudança de comportamento.
    - Método: classificar por risco em vez de varrer 1.142 comentários. Explicação de decisão ("por
      quê") envelhece bem e não foi tocada; afirmação sobre comportamento (~166) e marcador temporal
      (17) foram verificados **contra o código**, não pelo texto.
    - A maioria se sustentou (o `gemini` é mesmo o único importador do SDK; o `rss-discovery` é mesmo a
      única rota que busca URL do chamador; `SetAdmin` só é chamado pelo seed). Corrigidos três:
      `update_app_status` dizia "open for now" sendo admin desde a 0.40; os dry-runs de IA omitiam que
      só existem em development, que é o que de fato protege; e a query da camada 1 afirmava garantia
      de índice no primeiro parágrafo com a ressalva 12 linhas abaixo.
    - **Parte durável**: seção "Convenções de comentários" no `conventions.md`. Afirmação sobre
      comportamento exige a versão em que foi verificada ou um teste que a sustente; garantia de escopo
      global não pode ser declarada de dentro de um arquivo; afirmação cara de descobrir errada deve
      virar teste; e ao corrigir divergência, verificar qual lado está errado — senão a auditoria vira
      máquina de cimentar bug como intenção.

Planos que não serão aplicados agora. Use para entender evolução futura do código:

- Aumentar o verbose mode (+++++++++++++++++++++ logs)
- TlDraw do banco de dados
- Denunciar conteúdo
- Multi feed
- Editar informações básicas do usuário
- Cascade de tabelas (parcial: a 0.45 fez o de `article_outbound_links`; falta o restante)
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
