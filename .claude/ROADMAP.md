# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.4.0

## Versão 0.1.0

- [x] Setup inicial
    - [x] Criar o projeto para o Claude
    - [x] Definir arquitetura e padrões técnicos

## Versão 0.2.0

- [x] Criar o projeto utilizando a stack principal com suas dependências e estrutura de pastas básicas (Exemplos: mod, .gitignore, Dockerfile, .env)

## Versão 0.3.0

- [x] Preparar banco de dados para receber migrações utilizando o `goose`
- [x] Criar uma migração para a tabela de usuários
- [x] Cadastro de usuários (lembrete: apenas através do Google)
    - Sem endpoints na API, apenas tabela
    - Armazenar apenas dados da conta Google e:
        - id (UUID v7)
        - status (true ou false)
        - last_login_at (timestamp)
        - created_at (timestamp)
        - modified_at (timestamp opcional)
        - removed_at (timestamp opcional)

## Versão 0.4.0

- [x] Criar uma migração para a tabela de `refresh_tokens`
    - Armazenar os seguintes dados:
        - id (UUID v7)
        - status (true ou false)
        - user_id (UUID do usuário)
        - expires_at (timestamp)
        - created_at (timestamp)
        - modified_at (timestamp opcional)
        - removed_at (timestamp opcional)
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

## Versão 0.5.0

- [ ] Cadastro de preferências
    - Modo light ou dark
    - Idiomas preferidos
    - Traduzir para o idioma preferido (true ou false)
    - Personalidade preferida (mais divertido, mais informativo ou misto)
    - Associação com usuário (1 preferência pertence a 1 usuário)
- [ ] Cadastro de categorias
    - Nome da categoria
    - Palavras-chaves da categoria (string)
    - Associação com usuário (1 usuário possui N categorias)
    - Exemplos: categoria "Metallica" com palavras-chaves "Metallica, Rock"; categoria "Anime" com palavras-chave "Naruto, Anime, Cosplay"
- [ ] Cadastro de fontes (URLs)
    - URL base da fonte (Exemplo: https://g1.globo.com/politica/)
    - URL da RSS (Exemplo: https://g1.globo.com/rss/g1/politica/)
    - Sistema descobre automaticamente URL de RSS
    - Valida a URL antes de salvar
    - Tabela pública (todos os usuários podem cadastrar fontes)
    - Associação com categorias (cardinalidade 'muitos para muitos')
- [ ] Cadastro de notícias
    - Título da notícia
    - Conteúdo da notícia, já personalizado e traduzido conforme o usuário preferiu
    - URL da notícia original
    - Notícia foi lida pelo usuário (true ou false)
    - Associação com usuário (1 notícia pertence a 1 usuário)

## Versão 0.6.0

- [ ] Criação de testes unitários
    - Utilizando o agent `test_manager`

## Versão 0.7.0

- [ ] Filtragem de notícias
    - [ ] Buscar notícias de todas as fontes cadastradas
        - O processo ocorre através de cron interna que é executada periodicamente
        - Utilize as variáveis de ambiente para controlar a atividade da cron interna (RSS_FEED_CRON_ACTIVE e RSS_FEED_CRON_SCHEDULE)
    - [ ] Implementar filtragem por palavra-chave
    - [ ] Integrar Gemini 2.5 Flash para:
        - [ ] Validação inteligente
        - [ ] Personalização
        - [ ] Tradução

## Versão 0.8.0

- [ ] Deploy
    - [ ] Docker funcionando
    - [ ] docker-compose orquestrando corretamente

## Futuro

Planos que não serão aplicados agora. Use para entender evolução futura do código:

- Logger e observabilidade
- Swagger
- Usuário administrador
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
