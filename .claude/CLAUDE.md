# Resumo

API SQLite com Go para um feed de notícias personalizado usando fontes RSS.

# Regras de desenvolvimento

- Ao seguir o arquivo `ROADMAP.md`, desenvolva uma versão de cada vez e confirme comigo antes de avançar para a próxima etapa.

## Arquivos e pastas

- `CLAUDE.md`: contém um resumo geral e técnico do projeto.
- `PROJECT.md`: contém um resumo de como o projeto funciona (filosofia, fluxos, features, restrições, etc).
- `ROADMAP.md`: contém o roadmap do projeto, que também pode ser visto como uma lista de TODO.
- `agents/`: contém os agentes que dão suporte e estão presentes no desenvolvimento do projeto.
- `rules/`: contém um conjunto de regras para ajudar no desenvolvimento do projeto.

## Stack

- `GoLang` - linguagem base
- `Fiber` - framework web
- `SQLite3` - banco de dados
- `sqlite-web` - interface de visualização do banco de dados
- `goose` - migrações do banco de dados
- `sqlc` - operações SQL
- `jwt` - autenticação Bearer
- `gofeed` - parsing de RSS feeds
- `Gemini 2.5 Flash` - agente básico
- `Docker + docker-compose` - orquestração
- `testify` - biblioteca para testes

## Regras da stack

- Dependências devem sempre estar na versão mais atualizada possível.
- UUIDs devem estar na versão 7.

### RSS Discovery

- Quando um usuário cadastra uma fonte, tente descobrir automaticamente:
    1. Padrões comuns: `/rss/`, `/feed/`, `/feed.xml`
    2. Parse HTML para `<link rel="alternate" type="application/rss+xml">`
    3. Fallback: retorne erro se não encontrar
- Use `gofeed` para parsing robusta

### Filtragem de notícias

- **Primeira camada**: Filtragem por keywords (rápido)
- **Segunda camada**: Validação com Gemini (inteligente)
- **Score system**: 0-100, threshold configurável
- Sempre salve a razão da inclusão (`keyword`, `ai_match`, `manual`)

## Comandos

- `go run main.go` - Rodar localmente
- `go build -o api` - Build executável
- `docker-compose up` - Rodar com Docker
- `go mod tidy` - Limpar dependências

## Inicialização rápida

```bash
go mod download
go run main.go
```

## Porta

- A API roda em `http://localhost:3000`.
