# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.10.1.0

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

- [ ] Criar uma migração para a tabela de fonte de notícias
    - Acredito que o melhor nomenclatura aqui seja 'sources', mas aceito melhores sugestões
    - O arquivo `PROJECT.md` explica melhor como essa tabela funciona
    - Armazene os seguintes dados:
        - id (`UUID` v7)
        - status (`true` ou `false`)
        - url (Exemplo: https://g1.globo.com/politica/)
        - url_rss (Exemplo: https://g1.globo.com/rss/g1/politica/)
        - created_at (`timestamp`)
        - modified_at (`timestamp` opcional)
        - removed_at (`timestamp` opcional)
    - Esta tabela possui um relacionamento muito para muitos (N to N) com categorias
    - Devemos ser capazes de encontrar automaticamente a URL de RSS
    - Valida a URL antes de salvar
- [ ] Criar rotas para as fontes de notícias
    - Criação
    - Edição
    - Remoção
    - Buscar por id
    - Buscar por filtro
    - Descobrir automaticamente URL de RSS através da URL base
- [ ] Atualizar todas as rotas criadas ou alteradas ao Bruno
- [ ] Atualizar testes unitários para todas as rotas criadas ou alteradas
- [ ] Atualizar testes de integração para todas as rotas criadas ou alteradas
- [ ] Atualizar testes E2E para todas as rotas criadas ou alteradas

## Versão 0.12.0.0

- [ ] Cadastro de notícias
    - Acho que a melhor nomenclatura para essa tabela é 'Article'
    - Título da notícia
    - Conteúdo da notícia, já personalizado e traduzido conforme o usuário preferiu
    - URL da notícia original
    - Notícia foi lida pelo usuário (true ou false)
    - Associação com usuário (1 notícia pertence a 1 usuário)

## Versão 0.13.0.0

- [ ] Criar uma migração para a tabela de categorias de notícias
    - Acredito que o melhor nomenclatura aqui seja 'article categories', mas aceito melhores sugestões
    - O arquivo `PROJECT.md` explica melhor como essa tabela funciona
    - Armazene os seguintes dados:
        - id (`UUID` v7)
        - status (`true` ou `false`)
        - user_id (`UUID` do usuário - 1 categoria pertence a 1 usuário, 1 usuário cria N categorias)
        - name (`string`)
        - keywords (uma lista de `string` - formato `JSON array (TEXT)`)
        - created_at (`timestamp`)
        - modified_at (`timestamp` opcional)
        - removed_at (`timestamp` opcional)
        - lista de fontes de notícias ligadas ao registro
            - significa que essa tabela possui uma relação muito para muitos (N to N) com a tabela de fonte de notícias
    - Exemplos: categoria "Metallica" com palavras-chaves "Metallica, Rock"; categoria "Anime" com palavras-chave "Naruto, Anime, Cosplay"
- [ ] Criar rotas para as categorias de notícias
    - Criação
    - Edição
    - Remoção
    - Buscar por id
    - Buscar por filtro
- [ ] Criar uma migração para a tabela de categoria x fonte de notícias
    - É basicamente a tabela relacional entre categoria e fonte de notícias
    - O arquivo `PROJECT.md` explica melhor como essa tabela funciona
    - Armazene os seguintes dados:
        - id (`UUID` v7)
        - id da categoria de notícia (`UUID` v7 existente em categorias de notícia)
        - id da fonte de notícia (`UUID` v7 existente em fontes de notícia)
- [ ] Atualizar todas as rotas criadas ou alteradas ao Bruno
- [ ] Atualizar testes unitários para todas as rotas criadas ou alteradas
- [ ] Atualizar testes de integração para todas as rotas criadas ou alteradas
- [ ] Atualizar testes E2E para todas as rotas criadas ou alteradas

## Versão 0.14.0.0

- [ ] Filtragem de notícias
    - [ ] Buscar notícias de todas as fontes cadastradas
        - O processo ocorre através de cron interna que é executada periodicamente
        - Utilize as variáveis de ambiente para controlar a atividade da cron interna (RSS_FEED_CRON_ACTIVE e RSS_FEED_CRON_SCHEDULE)
    - [ ] Implementar filtragem por palavra-chave
    - [ ] Integrar Gemini 2.5 Flash para:
        - [ ] Validação inteligente
        - [ ] Personalização
        - [ ] Tradução

## Versão 0.15.0.0

- [ ] Deploy
    - [ ] Docker funcionando
    - [ ] docker-compose orquestrando corretamente

## Futuro

Planos que não serão aplicados agora. Use para entender evolução futura do código:

- Multi feed
- Editar informações básicas do usuário
- Cascade de tabelas
- Limpar o main
- Transportar a iniciação de endpoints para outro lugar, health para /api
- Criar uma rota para aceitar sugestões de fontes de notícias
- Logger e observabilidade
- Swagger
- Usuário administrador
- Opções de usuário administrador (desativar X, habilitar Y)
- Compartilhamento de notícias
- Múltiplos modelos de IA (Claude, GPT)
- LangChain para orquestração complexa
- Resumo de notícias longas
- Resumo diário de notícias (newsletter)
- Notificações em tempo real
- Sistema de feedback de notícias
- Descoberta automática de novas RSS feeds
- Regras de administradores
- Machine Learning: aprender com leitura/descarte do usuário
