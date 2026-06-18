# Estrutura

Este arquivo descreve a estrutura atual do projeto. Ele é um guia para entender onde cada parte do código deve ser colocada e como o projeto está organizado.

- `/db/` - Arquivos dos dados do banco de dados
- `/sqlc/` - Queries geradas
- `/api/src/` - Código fonte da API
- `/api/src/go.mod` - Arquivo de configuração do Go
- `/api/src/go.sum` - Arquivo de dependências do Go
- `/api/src/main.go` - Entrada da API
- `/api/src/services/` - Lógica de negócio da API (endpoints, helpers, autenticação, etc)
- `/database/src/models/` - Modelos do banco de dados
- `/database/src/schema.sql` - Schema do banco de dados
- `Dockerfile` - Build da aplicação
- `docker-compose.yaml` - Orquestração
- `.env` - Variáveis de ambiente (não versionado)
- `.gitignore` - Arquivos e pastas ignorados pelo Git

## Filosofia da estrutura

- A pasta `/.claude/` contém arquivos de configuração e instruções para o Claude, que é a ferramenta de desenvolvimento utilizada neste projeto.
- A pasta `/api/` é o coração do projeto da API, onde toda a lógica referente à API reside. Ela é organizada em subpastas para manter o código limpo e fácil de navegar.
- A pasta `/db/` é onde os dados do banco de dados são armazenados, enquanto a pasta `/sqlc/` contém as queries geradas automaticamente pelo sqlc.
- A pasta `/database/` é onde os arquivos relacionados ao banco de dados são mantidos, incluindo o schema e os arquivos de configuração do banco de dados.
- O `Dockerfile` e o `docker-compose.yaml` são usados para containerizar a aplicação e facilitar o desenvolvimento e a implantação.
