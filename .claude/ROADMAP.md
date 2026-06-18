# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

# Atual versão

0.2.0

## Versão 0.1.0

- [x] Setup inicial
    - [x] Criar o projeto para o Claude
    - [x] Definir arquitetura e padrões técnicos

## Versão 0.2.0

- [x] Criar o projeto utilizando a stack principal com suas dependências e estrutura de pastas básicas (Exemplos: mod, .gitignore, Dockerfile, .env)

## Versão 0.2.1

- [ ] Corrigir estrutura do projeto conforme as mudanças no arquivo `./rules/structure.md`
    - Talvez o .env precise estar presente ao lado de `main.go`?
    - Decidir exatamente onde a pasta `/sqlc/` deve ficar
        - Talvez na pasta `/database/`?
    - Decidir se o `sqlc.yaml` e o `docker-compose.yaml` são realmente separados um do outro
    - Alterar o `Dockerfile` e `docker-compose` conforme necessidade

## Versão 0.3.0

- [ ] CRUD básico
    - [ ] Autenticação Google + JWT (Bearer)
    - [ ] Cadastro de usuários (apenas através do Google)
        - Armazenar apenas dados da conta Google

## Versão 0.4.0

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

## Versão 0.5.0

- [ ] Criação de testes unitários
    - Utilizando o agent `test_manager`

## Versão 0.6.0

- [ ] Filtragem de notícias
    - [ ] Buscar notícias de todas as fontes cadastradas
        - O processo ocorre através de cron interna que é executada periodicamente
        - Utilize as variáveis de ambiente para controlar a atividade da cron interna (RSS_FEED_CRON_ACTIVE e RSS_FEED_CRON_SCHEDULE)
    - [ ] Implementar filtragem por palavra-chave
    - [ ] Integrar Gemini 2.5 Flash para:
        - [ ] Validação inteligente
        - [ ] Personalização
        - [ ] Tradução

## Versão 0.6.0

- [ ] Deploy
    - [ ] Docker funcionando
    - [ ] docker-compose orquestrando corretamente

## Futuro

Planos que não serão aplicados agora. Use para entender evolução futura do código:

- Logger e observabilidade
- Swagger
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
