# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.4.0.9

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

- [ ] Preparar testes criando `mocks`
    - `google_oauth` (na pasta `external`)
    - `users` (na pasta `repositories`)
    - `refresh_tokens` (na pasta `repositories`)
    - `jwt`(na pasta services)
- [ ] Preparar testes criando `fixtures`
    - `user`
    - `refresh_token`
- [ ] Criar testes na pasta `unit` para as rotas `auth` e `users`

## Versão 0.6.0.0

- [ ] Implementaremos os testes de integração
    - Para `users` e `refresh_tokens`
    - Mock para o OAuth
- [ ] Implementaremos os testes end-to-end de autenticação
    - 100% livre de qualquer mock ou simulação de informações
    - Testado diretamente pelas rotas

## Versão 0.7.0.0

- [ ] Criar uma migração para a tabela de preferências do usuário
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
- [ ] Criar rotas para as preferências do usuário
    - Edição, que pode ser PUT em `base_url/v1/users/me/preferences`
    - Buscar, que pode ser GET em `base_url/v1/users/me/preferences`
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
- [ ] Criar uma migração para a tabela de categoria x fonte de notícias
    - É basicamente a tabela relacional entre categoria e fonte de notícias
    - O arquivo `PROJECT.md` explica melhor como essa tabela funciona
    - Armazene os seguintes dados:
        - id (`UUID` v7)
        - id da categoria de notícia (`UUID` v7 existente em categorias de notícia)
        - id da fonte de notícia (`UUID` v7 existente em fontes de notícia)

## Versão 0.8.0.0

- [ ] Cadastro de notícias
    - Acho que a melhor nomenclatura para essa tabela é 'Article'
    - Título da notícia
    - Conteúdo da notícia, já personalizado e traduzido conforme o usuário preferiu
    - URL da notícia original
    - Notícia foi lida pelo usuário (true ou false)
    - Associação com usuário (1 notícia pertence a 1 usuário)

## Versão 0.9.0.0

- [ ] Filtragem de notícias
    - [ ] Buscar notícias de todas as fontes cadastradas
        - O processo ocorre através de cron interna que é executada periodicamente
        - Utilize as variáveis de ambiente para controlar a atividade da cron interna (RSS_FEED_CRON_ACTIVE e RSS_FEED_CRON_SCHEDULE)
    - [ ] Implementar filtragem por palavra-chave
    - [ ] Integrar Gemini 2.5 Flash para:
        - [ ] Validação inteligente
        - [ ] Personalização
        - [ ] Tradução

## Versão 0.10.0.0

- [ ] Deploy
    - [ ] Docker funcionando
    - [ ] docker-compose orquestrando corretamente

## Futuro

Planos que não serão aplicados agora. Use para entender evolução futura do código:

- Transportar a iniciação de endpoints para outro lugar
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
