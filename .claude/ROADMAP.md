# Roadmap

Este arquivo contém o atual estado de features.
Marcações em 'x' indica o que já está concluído.
Os níveis de tabulação indicam detalhes do assunto.

## Atual versão (0.1.0)

- [x] Setup inicial
    - [x] Criar o projeto para o Claude
    - [x] Definir arquitetura e padrões técnicos

## Versão 0.2.0

- [ ] Criar o projeto Go com suas dependências e estrutura de pastas básicas (Exemplos: mod, .gitignore, Dockerfile, .env)

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
        - Associação com usuário
    - [ ] Cadastro de categorias
        - Nome da categoria
        - Palavras-chaves da categoria (string)
        - Associação com usuário
        - Exemplos: categoria "Metallica" com palavras-chaves "Metallica, Rock"; categoria "Anime" com palavras-chave "Naruto, Anime, Cosplay"
    - [ ] Cadastro de fontes (URLs)
        - Sistema descobre automaticamente RSS da URL
        - Valida a URL antes de salvar
        - Armazena RSS URL no banco
        - Associação com categorias
    - [ ] Cadastro de notícias
        - Título da notícia
        - Conteúdo da notícia, já personalizado e traduzido conforme o usuário preferiu
        - URL da notícia original
        - Notícia foi lida pelo usuário (true ou false)
        - Associação com usuário

## Versão 0.5.0

- [ ] Filtragem de notícias
    - [ ] Buscar notícias de todas as fontes cadastradas
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
