# Estrutura

Este arquivo descreve a estrutura atual do projeto. Ele é um guia para entender onde cada parte do código deve ser colocada e como o projeto está organizado.

- `/main.go` - Entrada da aplicação
- `/schema.sql` - Schema do banco de dados
- `/db/` - Arquivos dos dados do banco de dados
- `/models/` - Modelos do banco de dados
- `/services/` - Lógica de negócio da API (endpoints, helpers, autenticação, etc)
- `/sqlc/` - Queries geradas
- `/src/` - Código fonte da aplicação
- `Dockerfile` - Build da aplicação
- `docker-compose.yaml` - Orquestração
