# Estrutura

Este arquivo descreve a organização de pastas atual do projeto. Ele é um guia para entender onde cada parte do código deve ser colocada.

Mostramos apenas a hierarquia de pastas com uma descrição em parênteses. Arquivos individuais e arquivos ignorados pelo git (temporários, configuração local, etc.) não são listados aqui.

O banco não aparece nesta árvore: desde a 0.37 é PostgreSQL (cliente-servidor), com os dados num volume do Docker em vez de um arquivo dentro do repositório.

```
/                              (raiz: main.go, go.mod/go.sum, schema.sql, sqlc.yaml,
│                               Taskfile.yaml, Dockerfile, docker-compose.yaml, .env.example)
├── bruno                      (coleção de requisições do app externo Bruno)
├── cmd                        (executáveis auxiliares fora da API)
│   └── seed                   (scripts de seed de dev: usuário, sources, feeds, artigos, associações)
├── logger                     (sistema de log global, controlado por VERBOSE_MODE)
├── middlewares                (middlewares HTTP: auth JWT, guard de manutenção por app_status)
├── migrations                 (migrações goose up/down, embutidas via embed)
├── pgadmin4                   (config do pgAdmin4: servers.json pré-registra o servidor de dev, sem a senha)
├── schemas                    (DTOs de request/response da API)
│   └── enums                  (enums de fonte única: language, ai_personality)
├── services                   (regras de negócio e integrações)
│   ├── ai                     (costura de IA por capacidade: Keyworder, Judger, Translator — a IA não toca no corpo)
│   │   ├── gemini             (implementação LLM — Google Gemini)
│   │   └── openaicompat       (implementação OpenAI-compatible — Ollama local, Groq)
│   ├── controllers            (regras de negócio stateless; recebem db.Querier e nunca commitam)
│   ├── cron                   (scheduler robfig/cron + DiscoveryRunner que varre as sources ativas)
│   ├── discovery              (descoberta de notícias no RSS de uma source + pipeline por artigo)
│   ├── embedtreatment         (antes do sanitize: Instagram(script) → link e reescreve o parent do iframe do Twitch para o host do CLIENT_URL)
│   ├── endpoints              (handlers HTTP)
│   │   └── v1                 (versão 1 da API: uma pasta por modelo, um arquivo por rota)
│   ├── judgement              (camada 2 do julgamento: pontua uma notícia contra feeds candidatos)
│   ├── langdetect             (detecção do idioma original via lingua-go, offline/determinístico)
│   ├── oauthstate             (assina/valida o `state` do login OAuth: CSRF + carrega o redirect_uri)
│   ├── pagination             (paginação global genérica: ParseParams + Paginate[T])
│   ├── prompts                (prompts de IA em .yaml, embutidos via go:embed)
│   ├── rss                    (descoberta de URLs de RSS a partir de uma URL principal)
│   ├── sanitize               (estágio primário de limpeza do corpo cru do RSS via bluemonday, determinístico)
│   └── urltreatment           (reescreve links internos do corpo para CLIENT_URL/articles/{id} antes do sanitize; parsing HTML, DB injetado)
├── sqlc                       (código gerado pelo sqlc: models, querier, *.sql.go)
│   └── queries                (queries SQL de origem consumidas pelo sqlc)
└── tests                      (testes automatizados)
    ├── end-to-end             (testes E2E)
    │   └── api                (uma pasta por modelo: users, auth, feeds, sources, ...)
    ├── integration            (testes de integração)
    │   ├── api                (uma pasta por modelo)
    │   ├── cron               (testes do scheduler/descoberta)
    │   └── discovery          (testes do pipeline de descoberta/tratamento)
    ├── unit                   (testes unitários; uma pasta por modelo)
    ├── fixtures               (helpers reaproveitáveis: criar usuário, feed, source, etc.)
    ├── mocks                  (mocks de integrações e dependências)
    │   ├── external           (Google OAuth2, Gemini, RSS)
    │   ├── repositories       (dados do banco de dados)
    │   └── services           (serviços internos, como geração de JWT)
    └── utils                  (utilitários gerais de teste; db.go sobe o Postgres descartável e dá um database por teste)
```
