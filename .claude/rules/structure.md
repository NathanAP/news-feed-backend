# Estrutura

Este arquivo descreve a estrutura base atual do projeto. Ele é um guia para entender onde cada parte do código deve ser colocada e como o projeto está organizado junto com alguns exemplos de arquivos.

Você não precisa alterar esse arquivo. A ideia é que eu possa atualizar ele conforme o projeto evolui, para servir como exemplo para você.

Arquivos ignorados pelo git (como arquivos temporários, de configuração local, etc.) não estão listados aqui.

```

/
├── main.go
├── go.mod
├── go.sum
├── schema.sql
├── sqlc.yaml
├── Dockerfile
├── docker-compose.yaml
├── .env.example
├── .gitignore
├── bruno/
├──── ...
├── cmd/
├──── seed/
├──── ...
├── logger/
├──── logger.go
├── middlewares/
├──── auth.go
├──── ...
├── migrations/
├──── 20260618120000_create_users_table.sql
├──── ...
├── schemas/
├────── users.go
├────── ...
├── services/
├──── ai/
├────── gemini.go
├────── ...
├──── controllers/
├────── users.go
├────── ...
├──── cron/
├────── cron.go
├────── ...
├──── discovery/
├────── discovery.go
├────── ...
├──── endpoints/
├────── v1/
├──────── users/
├────────── me.go
├────────── ...
├──────── ...
├──── judgement/
├────── judgement.go
├────── ...
├──── langdetect/
├────── langdetect.go
├────── ...
├──── prompts/
├────── article_translation.yaml
├────── ...
├──── rss/
├────── discovery.go
├────── ...
├──── sanitize/
├────── sanitize.go
├────── ...
├── sqlc/
├──── db.go
├──── models.go
├──── users.sql.go
├──── ...
├──── queries/
├────── users.sql
├────── ...
├── tests/
├───── end-to-end/
├────── api/
├───────── users/
├─────────── me_test.go
├─────────── ...
├───── unit/
├─────── users/
├───────── me_test.go
├───────── ...
├───── integration/
├─────── api/
├───────── users/
├─────────── me_test.go
├─────────── ...
├───── fixtures/
├─────── users.go
├─────── ...
├───── mocks/
├─────── external/
├───────── gemini_client.go
├───────── google_oauth.go
├───────── rss_client.go
├───────── ...
├─────── repositories/
├───────── users_repository.go
├───────── ...
├─────── services/
├───────── jwt.go
├───────── ...
├───── utils/
├───────── db.go
├───────── ...
└── ...
```
