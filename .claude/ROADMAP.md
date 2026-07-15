# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.36.2.0

## Versão 0.1.0.0

- [x] Setup inicial
    - [x] Criar o projeto para o Claude
    - [x] Definir arquitetura e padrões técnicos

## Versão 0.2.0.0

- [x] Criar o projeto utilizando a stack principal com suas dependências e estrutura de pastas básicas (Exemplos: mod, .gitignore, Dockerfile, .env)

## Versão 0.3.0.0

- [x] Preparar banco de dados para receber migrações utilizando o `goose`
- [x] Criar uma migração para a tabela de usuários
- [x] Cadastro de usuários (lembrete: apenas através do Google)
    - Sem endpoints na API, apenas tabela
    - Armazenar apenas dados da conta Google e:
        - id (`UUID` v7)
        - status (`true` ou `false`)
        - last_login_at (`timestamp`)
        - created_at (`timestamp`)
        - modified_at (`timestamp` opcional)
        - removed_at (`timestamp` opcional)

## Versão 0.4.0.0

- [x] Criar uma migração para a tabela de `refresh_tokens`
    - Armazenar os seguintes dados:
        - id (`UUID` v7)
        - status (`true` ou `false`)
        - user_id (`UUID` do usuário)
        - expires_at (`timestamp`)
        - created_at (`timestamp`)
        - modified_at (`timestamp` opcional)
        - removed_at (`timestamp` opcional)
- [x] Criar rota para visualizar os dados do usuário (GET `base_url/v1/users/me`)
- [x] Autenticação Google + JWT (Bearer)
    - Lembrete: fluxo de login está em ´PROJECT.md`
- [x] Criar rota para renovação do `access_token` (POST `base_url/v1/auth/refresh/` requirindo o `refresh_token` no body)
    - Retorna o novo token renovado
    - `refresh_token` sofre alteração em `expires_at` para também ser renovado
    - Caso o `refresh_token` esteja expirado, o fluxo de login deve ser refeito
    - Lembrete: esse fluxo está presente em `PROJECT.md`
- [x] Criar rota para logout
- [x] Criar um invalidador de `refresh_token`, possibilitando derrubar a sessão de um usuário específico
    - Vamos adicionar o usuário administrador depois, então essa rota por enquanto pode ficar aberta
- [x] Criar um invalidador de todos os `refresh_token`, possibilitando derrubar todas as sessões de uma vez
    - Vamos adicionar o usuário administrador depois, então essa rota por enquanto pode ficar aberta

## Versão 0.5.0.0

- [x] Preparar testes criando `mocks`
    - `google_oauth` (na pasta `external`)
    - `users` (na pasta `repositories`)
    - `refresh_tokens` (na pasta `repositories`)
    - `jwt`(na pasta services)
- [x] Preparar testes criando `fixtures`
    - `user`
    - `refresh_token`
- [x] Criar testes na pasta `unit` para as rotas `auth` e `users`

## Versão 0.6.0.0

- [x] Criar um Taskfile.yaml para rodar os comandos básicos do projeto
    - Versão mais atualizada possível caso necessite instalação local
    - Descrições coerentes
    - Subir aplicação toda localmente, sem Docker(`task local-start` e `task ls`)
    - Reiniciar aplicação toda localmente (`task local-restart` e `task lr`)
    - Parar aplicação toca localmente (`task local-down` e `task ld`)
    - Subir aplicação toda via Docker (`task docker-start` e `task ds`)
    - Reiniciar aplicação via Docker (`task docker-restart` e `task dr`)
    - Parar aplicação toca localmente (`task docker-down` e `task dd`)
    - Prune do Docker (`task docker-prune` e `task dp`)
    - Build (`task build` e `task b`)
    - Rodar `sqlc generate` (`task sqlc-generate` e `task sg`)
    - Lint do Go em ./... (`task vet-all` e `task va`)
    - Rodar todos os testes (`task test-all` e `task ta`)
    - Rodar testes unitários (`task test-units` e `task tu`)
    - Rodar testes de integração (`task test-integration` e `task ti`)
    - Rodar testes end-to-end (`task test-end-to-end` e `task te2e`)
    - Criar helper (`task` ou `task help`)
    - Observação: não precisa rodar o up do Docker nesse momento, mas garanta que é possível rodar ele localmente
    - Observação 2: se tiver algum comando que você recomenda muito ser feito, pode me falar durante a etapa de planejamento
- [x] Implementar os testes de integração
    - Para `users` e `refresh_tokens`
    - Mock para o OAuth2
- [x] Implementar os testes end-to-end de autenticação
- [x] Criar os primeiros arquivos em `.claude/versions` para especificar o que foi feito até agora
    - Não tem problema você não saber o timestamp das outras versões, coloque de forma que fique em ordem e tá tudo certo

## Versão 0.7.0.0

- [x] Criar o sistema de log de acordo com as convenções e o project.md
    - Observação: inclua a rota `/health`

## Versão 0.8.0.0

- [x] Criar a pasta relacionada ao aplicativo Bruno

## Versão 0.9.0.0

- [x] Criar uma documentação da versão 0.8.0.0 na qual adicionamos o Bruno ao projeto
- [x] Conferir se as especificações em relação ao Bruno estão boas o suficiente para fazer o básico
- [x] Garantir que, no teste E2E do cadastro de usuários, além de simular que o usuário fez o processo da Google, também:
    - grave um `user no banco de dados com o propósito de testar que esse processo está ocorrendo corretamente.
    - grave um `refresh_token`no banco de dados com o propósito de testar que esse processo está ocorrendo corretamente.

## Versão 0.10.0.0

- [x] Criar uma migração para a tabela de preferências do usuário
    - Armazene os seguintes dados:
        - id (`UUID` v7)
        - status (`true` ou `false`)
        - user_id (`UUID` do usuário - 1 preferência de usuário pertence a 1 usuário, 1 usuário só pode ter 1 preferência de usuário)
        - Modo light ou dark (`enum`)
        - Idioma preferido (Um `enum` contendo alguns idiomas como Português, Inglês, Espanhol, etc - pode colocar várias, não tem problema)
        - Traduzir conteúdo de notícias para o idioma preferido (`true` ou `false`)
        - Personalidade de inteligência artificial preferida (mais divertido, mais informativo ou misto)
        - created_at (`timestamp`)
        - modified_at (`timestamp` opcional)
        - removed_at (`timestamp` opcional)
    - Na mesma migração passar em cada usuário do banco de dados atual e criar uma preferência para cada um deles utilizando os valores padrão de preferências
- [x] Criar rotas para as preferências do usuário
    - Edição, que pode ser PUT em `base_url/v1/users/me/preferences`
        - Lembrete: o fluxo desta rota exige que um novo `access_token` seja gerado contendo as novas informações de preferências do usuário, então além das preferences, esta rota já retorna também esse `access_token` atualizado
    - Buscar, que pode ser GET em `base_url/v1/users/me/preferences`
        - Lembrete: os dados já estão no JWT, você pode fazer ele ler diretamente no `access_token` sem precisar ir ao banco de dados
- [x] Atualizar soft-remove do usuário para também dar soft-remove em suas preferências
- [x] Atualizar todas as rotas criadas ou alteradas ao Bruno
- [x] Atualizar testes unitários para todas as rotas criadas ou alteradas
- [x] Atualizar testes de integração para todas as rotas criadas ou alteradas
- [x] Atualizar testes E2E para todas as rotas criadas ou alteradas

## Versão 0.10.1.0

- [x] Realizar uma revisão geral até agora
    - Detalhes ficaram no arquivo do versions

## Versão 0.11.0.0

- [x] Criar uma migração para a tabela de fonte de notícias (`sources`)
    - Nomenclatura: `sources`
    - Campos: id, status, url, url_rss, created_at, modified_at, removed_at
    - url e url_rss com constraint UNIQUE global
    - Sem user_id (fontes são globais, gerenciadas por admins)
- [x] Criar rotas para as fontes de notícias
    - Lembrete: no futuro essas rotas serão acessadas apenas pelos usuários administradores
    - [x] Criação (`POST /v1/sources/create`)
    - [x] Edição (`PUT /v1/sources/{id}`)
    - [x] Remoção (`DELETE /v1/sources/{id}`) — soft delete
    - [x] Buscar por id (`GET /v1/sources/{id}`)
    - [x] Buscar por filtro (`GET /v1/sources?url=...`)
    - [x] Descobrir automaticamente URL de RSS (`GET /v1/sources/rss_discovery?url=...`)
        - Estratégia: HTML parsing → common paths → validação com gofeed
        - Retorna 200 com lista vazia quando não encontra nada
- [x] Atualizar todas as rotas criadas ao Bruno
- [x] Criar testes unitários para todas as rotas
- [x] Criar testes de integração para todas as rotas
- [x] Criar testes E2E para todas as rotas

## Versão 0.12.0.0

- [x] Garantir que a documentação sobre o `status` está coerente
- [x] Garantir que a aplicação está de acordo com a documentação sobre `status`
- [x] Corrigir locais inconsistentes com a documentação sobre `status`

## Versão 0.12.1.0

- [x] Corrigir bug onde uma fonte soft-deleted bloqueava a criação de uma nova fonte com a mesma `url`/`url_rss` (constraint `UNIQUE` ignorava `removed_at`)
    - Trocado por índices únicos parciais (`WHERE removed_at IS NULL`)

## Versão 0.13.0.0

- [x] Criar uma migração para a tabela de notícias
    - Nomenclatura da tabela: `articles`
    - Armazene os seguintes dados:
        - id (`UUID` v7)
        - status (`true` ou `false`)
        - title (`string`)
        - content (`string` em formato `md`)
        - url_original (`string`, único entre os ativos na tabela)
        - keywords (uma lista de `string` - formato `JSON array (TEXT)`)
        - created_at (`timestamp`)
        - modified_at (`timestamp` opcional)
        - removed_at (`timestamp` opcional)
- [x] Criar rotas para as notícias
    - Lembrete: no futuro essas rotas serão acessadas apenas pelos usuários administradores
    - [x] Criação (`POST /v1/articles/create`)
    - [x] Edição (`PUT /v1/articles/{id}`)
    - [x] Remoção (`DELETE /v1/articles/{id}`) — soft delete
    - [x] Buscar por id (`GET /v1/articles/{id}`)
    - [x] Buscar por filtro (`GET /v1/articles?url=...`)

## Versão 0.14.0.0

- [x] Criar uma migração para relacionar as fontes de notícias com as notícias
    - Na tabela de notícias já existente
        - Adicionar a coluna `source_id` sendo um `UUID` v7
        - As notícias cadastradas devem receber de agora em diante um `source_id` também
        - Não precisa fazer processo retroativo para notícias já existentes
    - Alterar rota de criação para receber também o `source_id`

## Versão 0.14.1.0

- [x] Implementar o cascade de exclusão entre fonte de notícias e notícias
    - Ao dar soft remove em uma `source`, todos os `articles` daquela fonte também sofrem soft remove

## Versão 0.15.0.0

- [x] Confirmar que o soft remove de usuários existe no controller
- [x] Criar uma migração para a tabela de feeds
    - Tabela de feeds
        - Nomenclatura: `feeds`
        - Armazene os seguintes dados:
            - id (`UUID` v7)
            - status (`true` ou `false`)
            - name (`string`)
            - keywords (uma lista de `string` - formato `JSON array (TEXT)`)
            - user_id (`UUID` v7)
            - created_at (`timestamp`)
            - modified_at (`timestamp` opcional)
            - removed_at (`timestamp` opcional)
- [x] Criar rotas para os feeds
    - [x] Criação (`POST /v1/feeds/create`)
    - [x] Edição (`PUT /v1/feeds/{id}`)
    - [x] Remoção (`DELETE /v1/feeds/{id}`) — soft delete
    - [x] Buscar por id (`GET /v1/feeds/{id}`)
    - [x] Buscar por filtro (`GET /v1/feeds?name=...`)
    - Lembrete: lembre-se que apenas o próprio usuário da requisição pode ver o feed requisitado na url/query
    - Lembrete: se um usuário requisitar feed de outro usuário, deve-se retornar 404 e não 403
    - Lembrete: fazer cascateamento para quando um usuário for removido (soft ou hard)

## Versão 0.16.0.0

- [x] Criar uma migração para a tabela relacional (junction table) entre feeds e notícias
    - Tabela relacional:
        - Nomenclatura: `articles_feeds`
        - Armazene os seguintes dados:
            - id (`UUID` v7)
            - article_id (`UUID` v7)
            - feed_id (`UUID` v7)
            - is_read (`bool`)
            - created_at (`timestamp`)
            - modified_at (`timestamp` opcional)
- [x] Criar uma rota em notícias para marcar ela como lida (`PUT /v1/articles/{id}/read`)
    - Lembrete: essa marcação vai afetar o registro em `articles_feeds`
    - Lembrete: se a notícia estiver em um ou mais feeds do usuário, todas são marcadas como lida
    - Lembrete: se a notícia não existir em nenhum feed do usuário, nada acontece
    - Lembrete: se a notícia já estiver lida, nada acontece (o retorno pode continuar sendo 200)

## Versão 0.17.0.0

- [x] Fazer revisão
- [x] Falar sobre memory

## Versão 0.18.0.0

- [x] Criar uma migração para a tabela `system`
    - Tabela de sistema:
        - id (`UUID` v7)
        - app_status (`true` ou `false`)
        - last_article_discovery_at (`timestamp`)
        - created_at (`timestamp`)
        - modified_at (`timestamp` opcional)
    - Lembrete: a migração deve criar um novo registro nessa tabela durante a migração
    - Lembrete: essa tabela nunca aceita novos registros ou exclusões, apenas atualizações
    - Lembrete: todas as rotas devem levar em conta se `app_status` está ativo ou não
- [x] Criar uma rota para alterar o status do app
    - Pode estar debaixo de `PUT base_url/v1/system/app-status`
    - Lembrete: no futuro essas rotas serão acessadas apenas pelos usuários administradores
- [x] Adicionar à rota `health`:
    - A hora atual do servidor (em UTC)
    - O status do servidor (`app_status` da tabela `system`)

## Versão 0.19.0.0

- [x] Descobrir notícias de todas as fontes cadastradas
    - O processo ocorre através de CRON interna que é executada periodicamente
    - Utilize as variáveis de ambiente para controlar a atividade da CRON interna (`RSS_FEED_CRON_ACTIVE` e `RSS_FEED_CRON_SCHEDULE`)
    - Crie um bom log para mostrar a CRON funcionando, principalmente porque nesse ponto do dev nada será gravado no banco de dados, então para o teste visual é importante ver algo como "a CRON rodou e trouxe isso aqui" (variável de ambiente `RSS_FEED_CRON_VERBOSE_MODE`)
    - Lembrete: ao final desse processo, deve escrever em `system.last_article_discovery_at` a hora atual para armazenar a última descoberta de notícias
    - Lembrete: o próximo passo após a descoberta é o tratamento e julgamento que será feito a seguir, deixe isso preparado para evoluir
    - Lembrete: aplicar o tratamento de `now()` quando `last_article_discovery_at` estiver vazio (primeira run da cron)
- [x] Criar uma rota de teste para descobrir notícias a partir de uma source, simulando a exata execução da CRON através do Bruno
    - Detalhes presentes em `PROJECT.md`
    - Vamos adicionar o usuário administrador depois, então essa rota por enquanto pode ficar aberta
- [x] Criar os prompts que serão utilizados a seguir

## Versão 0.20.0.0

- [x] Alterar a rota de `baseUrl/v1/sources/rss_discovery` para `rss-discovery`
- [x] Alterar a rota de `baseUrl/v1/sources/discovery` para `article-discovery`
- [x] Aplicar nova lógica de descoberta de notícias (através da URL original ao invés da hora)
- [x] Tratamento de notícias
    - Realizar o tratamento de notícias de acordo com o `PROJECT.md`
    - Ao final do tratamento a notícia é salva no banco de dados normalmente
    - Lembrete: esse método precisa fazer parte da CRON da descoberta de notícias, porém ela deve ser independente dos outros métodos
    - Lembrete: vamos utilizar Google Gemini 2.5 Flash para os primeiros testes
- [x] Criar uma rota de teste para tratar notícias a partir dos dados crus de uma notícia, simulando a exata execução da CRON através do Bruno
    - Detalhes presentes em `PROJECT.md`
    - Pode estar debaixo de `POST /v1/articles/treatment`
    - Vamos adicionar o usuário administrador depois, então essa rota por enquanto pode ficar aberta
    - Lembrete: essa rota é dry-run

## Versão 0.21.0.0

- [x] Alterar o processo de atribuição de keywords da etapa de tratamento para uma SLM ao invés de LLM
    - Provider por tarefa via env (`TREATMENT_*` LLM / `KEYWORDS_*` SLM); Ollama local (`OLLAMA_BASE_URL`), sem Docker por ora; testes via mock.

## Versão 0.21.3.0

- [x] Melhorias da etapa de tratamento
    - Mais detalhes no arquivo de `versions`

## Versão 0.22.0.0

- [x] Julgamento de notícias aos feeds
    - Camada 1 (comparação de keywords) totalmente em SQL via `json_each` (`FindCandidateFeedsByKeywords`)
    - Camada 2 (IA + `score`) através da nova capacidade `ai.Judger`, com modos `JUDGEMENT_MODE` (local/groq/gemini) e `JUDGEMENT_THRESHOLD`
    - Integrado na CRON de descoberta: após persistir a notícia, julga e grava as associações em `articles_feeds`
- [x] Criar uma rota de teste para julgamento de notícias a partir dos dados crus de uma notícia, simulando a exata execução da CRON através do Bruno
    - Detalhes presentes em `PROJECT.md`
    - Pode estar debaixo de `POST /v1/articles/judgement`
    - Vamos adicionar o usuário administrador depois, então essa rota por enquanto pode ficar aberta
    - Lembrete: essa rota é dry-run

## Versão 0.23.0.0

- [x] Garantir que o `enum` de idiomas (atualmente apenas utilizado nas preferências de usuário) seja globalmente visível para a aplicação sempre usar a mesma fonte
    - Movido para `schemas/language.go` como fonte única, usado por preferências, artigos e tradução
- [x] Criar uma migração que identifica qual é o idioma original da notícia
    - Adicionar o campo `language_original` à tabela de notícias
    - Tipo `enum` utilizando o mesmo enum alterado no passo anterior
    - Não precisa fazer backfill, eu excluo meu banco antes de começar a usar novamente
    - Utilizar o `lingua-go` para detectar o idioma
- [x] Adicionar ao tratamento de notícias o passo que detecta o idioma da notícia sendo tratada
    - Detecção via `services/langdetect` (lingua-go), no conteúdo cru; `null` quando não confiável
- [x] Criar métodos para fazer traduções personalizadas
    - Capacidade `ai.Translator` (LLM apenas via `TRANSLATION_*`) + prompt `article_translation`
- [x] Criar rota para chamada das traduções personalizadas
    - `GET /v1/articles/{id}/translate/{language}` — read-only, re-sanitiza a saída, gate por `translate_content` (403)
- [x] Alterar a rota de criação a alteração de notícias para aceitar também o idioma da notícia
    - É obrigatório e deve estar na lista de idiomas disponíveis
- [x] Remover idioma japonês e chinês da lista de idiomas disponíveis

## Versão 0.23.1.0

- [x] Remover a tradução das keywords da notícia
    - Keywords são canônicas em inglês; traduzi-las geraria termos fora de sincronia. Removido de `ai.Translator`, do prompt e da resposta do endpoint.

## Versão 0.24.0.0

- [x] Corrigir a nomenclatura da pasta `raiz/services/judgment` e `raiz/services/ai/judgment` para `judgement`
- [x] Criar lógica de paginação para uso global
    - Seguir a lógica presente em `PROJECT.md`
    - Pacote `services/pagination` (`ParseParams` + `Paginate[T]` genérico); paginação em memória sobre a lista já filtrada
- [x] Adicionar paginação em todas as rotas de busca múltipla (`GET base_url/v1/{model}/`)
    - `GET /v1/articles`, `/v1/sources`, `/v1/feeds` agora retornam `{ docs, pagination }`
- [x] Mover `enums` para um local comum
    - Ficaram em `schemas/enums/` (um arquivo por enum: `theme.go`, `language.go`, `ai_personality.go`)
    - Sobre `langdetect`/`sanitize` terem `_test.go`: são pacotes de lógica pura (sem banco/rota), então teste co-localizado é o idiomático — mesmo padrão do `services/ai`. Documentado no `conventions.md`.

## Versão 0.25.0.0

- [x] Criar um comando para criar registro do usuário dev
    - Comando deve ser `task seed-user-dev` ou `task sud`
    - A execução do comando roda um script em Go presente na pasta `raiz/cmd/seed/dev_user.go`
    - Utilizar arquivos na pasta `raiz/cmd/seed/examples.json`
    - Não permitir criar o usuário duas vezes
- [x] Criar um comando para criar um `access_token` ao usuário dev
    - Comando deve ser `task seed-dev-login` ou `task sdl`
    - A execução do comando roda um script em Go presente na pasta `raiz/cmd/seed/dev_login.go`
    - Usuário dev deve existir
    - Emitir automaticamente um `refresh_token` e um `access_token` como retorno final do comando
- [x] Criar um comando para criar registros de fontes de notícias
    - Comando deve ser `task seed-dev-sources` ou `task sds`
    - A execução do comando roda um script em Go presente na pasta `raiz/cmd/seed/dev_sources.go`
    - Utilizar arquivos na pasta `raiz/cmd/seed/examples.json`
    - Não permitir criar o fontes de notícias duas vezes
- [x] Criar um comando para criar registros de notícias sem precisar depender da CRON
    - Comando deve ser `task seed-dev-articles` ou `task sda`
    - A execução do comando roda um script em Go presente na pasta `raiz/cmd/seed/dev_articles.go`
    - Utilizar arquivos na pasta `raiz/cmd/seed/examples.json`
    - Permitir chamar o comando mais de uma vez pra criar as mesmas notícias mais de uma vez
    - Pra evitar o problema da `url_original` duplicada, precisamos que o script executado crie uma url aleatória inexistente para cada notícia
        - Pode fazer um random numérico mesmo, tipo `https://www.article-{uuid_aleatorio}.com.br/feed/`
- [x] Criar um comando para criar registros de feeds
    - Comando deve ser `task seed-dev-feeds` ou `task sdf`
    - A execução do comando roda um script em Go presente na pasta `raiz/cmd/seed/dev_feeds.go`
    - Utilizar arquivos na pasta `raiz/cmd/seed/examples.json`
    - Criar apenas para o usuário dev
    - Não permitir criar o feeds duas vezes
- [x] Criar um comando para criar registros de `articles_feeds` sem precisar depender da CRON
    - Comando deve ser `task seed-dev-articles-feeds` ou `task sdaf`
    - A execução do comando roda um script em Go presente na pasta `raiz/cmd/seed/dev_article_feeds.go`
    - Procurar cada notícia e ligar elas em cada feed existente
    - Criar apenas para o usuário dev
- [x] Criar um comando para executar todos de uma vez
    - Comando deve ser `task seed-dev-full` ou `task sdfull`
    - A execução do comando roda um script em Go presente na pasta `raiz/cmd/seed/dev_full.go`
- [x] Criar endpoint `login-dev` conforme descritp em `PROJECT.md`
- [x] Todos esses scripts só podem ser executados enquanto a variável de ambiente `ENVIRONMENT` estiver em `development`
- [x] Documentar sobre esses comandos (os de seed) em `raiz/cmd/seed/instructions.md`
- [x] Documentar todos os comportamentos sobre a pasta `cmd` detalhadamente em `raiz/.claude/memory/cmd.md`

## Versão 0.26.0.0

- [x] Fazer com que a aplicação possa subir através do Docker
    - API (Dockerfile multi-stage, binário estático Go puro; healthcheck em `/health`)
    - SQLite + interface gráfica (`sqlite-web`, espera a API ficar healthy antes de abrir o banco)

## Versão 0.27.0.0

- [x] Tornar o processo de descoberta, tratamento e julgamento de notícias poder ocorrer em paralelo ao invés de sequencial
    - Escolha: **goroutines internas** (worker pool limitado), não container efêmero. O gargalo é latência de I/O de rede (2-3 round-trips de IA por notícia), que se resolve com concorrência, não com mais containers.
    - `DISCOVERY_CONCURRENCY` (default `1`) governa a varredura de sources (RSS) e o pipeline por artigo. Dev/testes ficam em `1` (sequencial, determinístico); staging/produção sobem o valor, limitado pelo rate limit do provedor de IA.
    - A fronteira ficou desenhada de forma que o pipeline por artigo (`processOne`) não conhece quem o despacha — o pool in-process é o caso de 1 nó do modelo de fila distribuída (ver seção futura), então a migração pós-Postgres é troca de dispatcher, não reescrita.
- [x] Melhorar o prompt de keywords para o Groq acertar o idioma (few-shot multilíngue + regra de nomes próprios + reforço no user)
    - Motivo bundle: o ganho de throughput da versão depende do Groq elencar keywords bem e nunca errar o idioma delas.

## Versão 0.28.0.0

- [x] Atualização de pacotes e dependências
    - Bump conservador das diretas desatualizadas: `fiber` v2.52.14, `goose` v3.27.2, `sqlite` v1.53.0 (+ `go mod tidy`)
- [x] Revisão
    - Achados corrigidos no patch `0.28.1.0` (banco de produção sem `busy_timeout`; detecção de UNIQUE por substring)
- [x] Organizar o exemplo de arquivos em structure.md

## Versão 0.28.1.0

- [x] Corrigir banco de produção sem `busy_timeout`/`WAL` (achado grave da revisão da 0.28)
    - `main.go` abria o SQLite sem PRAGMA nem config de pool; com a concorrência da 0.27 (`DISCOVERY_CONCURRENCY > 1`) ou mesmo a cron sobreposta às requisições, escritas paralelas davam `SQLITE_BUSY` e o artigo era descartado
    - DSN passou a carregar `_pragma=busy_timeout(5000)` (writer espera o lock em vez de falhar) e `_pragma=journal_mode(WAL)` (leitores não bloqueiam o writer)
- [x] Trocar a detecção de violação de UNIQUE por substring por erro tipado do driver
    - Novo helper `controllers.isUniqueViolation` (checa `SQLITE_CONSTRAINT_UNIQUE` via `*sqlite.Error`); aplicado nos 5 call sites (users, sources×2, articles×2)
    - Você é melhor que eu nisso, não dá pra negar
    - Desisti da ideia de ter "arquivo.extensão" e "...", é melhor só mostrar a organização das pastas e uma simples descrição em parênteses mesmo
        - Algo parecido com `├── bruno (tudo relacionado ao Bruno)`

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

## Versão 0.36.2.0

Redução de tokens de IA + correção de recall no julgamento (a camada 1 deixava de trazer feeds que encaixariam).

- [x] Camada 1 (recall): o keyworder passa a emitir também termos **genéricos** (gênero/categoria) junto dos específicos, para que feeds genéricos casem notícias de entidades específicas no overlap exato de keywords
    - Limite de keywords da notícia subiu de 20 para **30** (`ArticleKeywordsMax`) para dar espaço aos genéricos; prompt `article_keywords` reescrito com a regra específico+genérico e exemplos
- [x] Keywords recebem **texto puro** (HTML removido via `sanitize.PlainText`) em vez do HTML — corta tokens de URL de imagem/iframe/href sem perder sinal. Conteúdo gravado continua HTML
- [x] Camada 2 (precisão + tokens): o julgamento **deixa de enviar o corpo** da notícia; julga por `título + keywords` (`ai.Judger` sem o param `content`, prompt reescrito). Isso mata o multiplicador (corpo × nº de candidatos) que estourava o rate limit do Groq
    - O `JUDGEMENT_THRESHOLD` precisa ser **re-calibrado** manualmente no dry-run (a escala dos scores muda sem o corpo)
    - Dry-run `POST /articles/judgement` não aceita mais `article.content` (só `title` + `keywords`)

## Futuro

Planos que não serão aplicados agora. Use para entender evolução futura do código:

- Aumentar o verbose mode (+++++++++++++++++++++ logs)
- Trocar para Postgres
- Filtragens estão acontecendo via Go (ao invés de SQL)
- Paginações estão acontecendo via Go (ao invés de SQL)
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
