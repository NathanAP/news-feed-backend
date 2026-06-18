# Estrutura

Este arquivo descreve a estrutura base atual do projeto. Ele é um guia para entender onde cada parte do código deve ser colocada e como o projeto está organizado junto com alguns exemplos de arquivos.

Você não precisa alterar esse arquivo. A ideia é que eu possa atualizar ele conforme o projeto evolui, para servir como exemplo para você.

/
├──main.go
├──go.mod
├──go.sum
├──schema.sql
├──sqlc.yaml
├──Dockerfile
├──docker-compose.yaml
├──.env (gitignored)
├──.env.example
├──.gitignore
├──db/
├──middlewares/
├────auth.go
├──migrations/
├────20260618120000_create_users_table.sql
├──models/
├──services/
├────controllers/
├──────users.go
├────endpoints/
├──────v1/
├────────users/
├──────────me.go
├──sqlc/
├────queries/
├──tests/
├────unit/
├────integration/
├────fixtures/
└────mocks/
