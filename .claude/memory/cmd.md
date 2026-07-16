# Pasta `cmd/` (utilitários de desenvolvimento)

> Resumo. Detalhes de uso em `cmd/seed/instructions.md`. Regras de ambiente em
> `.claude/PROJECT.md` (seção "Ambiente de desenvolvimento").

## O que é

Scripts e comandos **exclusivos de desenvolvimento** para cortar caminhos (semear dados, logar o
usuário dev, etc.). **Nunca** rodam em homologação/produção — o CI/CD apaga a pasta `cmd/` nesses
ambientes, e cada comando recusa rodar fora de `ENVIRONMENT=development`.

## `cmd/seed` — semeadura do banco de dev

- **Binário único** com subcomando (`go run ./cmd/seed <cmd>`). Um `cmd/seed/Taskfile.yaml` (incluído
  com `flatten` **e `optional: true`** pelo Taskfile raiz) expõe cada subcomando como um `task`
  rodável da raiz. O `optional` é essencial: quando o CI/CD apaga `cmd/` em staging/prod, o Task
  ignora o include ausente em vez de quebrar o Taskfile inteiro.
- Dados de exemplo em `cmd/seed/examples.json` (`user`, `user_preferences`, `sources`, `feeds`,
  `articles`). Artigos não têm `url_original` (gerado aleatório por run) nem `source_id` (ligado às
  sources existentes no banco).
- Escreve **através dos controllers da API** (mesmas regras: UUID v7, encode de keywords, limite de
  feeds, etc.). Acha a raiz via `go.mod`, conecta no Postgres pela `DATABASE_URL` (a mesma da API) e
  roda migrações (funciona em banco zerado). Exige, portanto, o banco no ar (`task dbs`). Como o
  `main.go`, fixa `time.Local = time.UTC` antes de tocar no banco.
- Estrutura: `main.go` (dispatcher + wiring dos controllers), `seed.go` (guard de ambiente, raiz,
  abertura+migração do DB, `report`), `examples.go` (structs + loader), e um `dev_*.go` por comando.

### Comandos (task / subcomando)

- `sud` / `user` — cria usuário dev + preferências. Idempotente. `google_id` fixo
  (`schemas.DevUserGoogleID`) para o dev-login achar o usuário.
- `sdl` / `login` — imprime `access_token` + `refresh_token` do usuário dev.
- `sds` / `sources` — cria as sources. Idempotente (pula url já existente).
- `sda` / `articles` — cria os artigos. **Re-executável** (url aleatória por run). Exige sources.
- `sdf` / `feeds` — cria feeds do usuário dev. Idempotente (pula nome já existente).
- `sdaf` / `articles-feeds` — liga cada artigo a cada feed do usuário dev (ignora julgamento).
  Idempotente.
- `sdfull` / `full` — roda tudo em ordem (user → sources → feeds → articles → articles-feeds → login).

Comandos idempotentes pulam o que já existe e reportam criados vs. pulados. Validação de enums das
preferências: valor inválido em `examples.json` falha claro.

## Login do usuário dev (sem Google)

- **CLI:** `task sdl`.
- **HTTP:** `POST /v1/users/dev-login` — mesma coisa, para um botão "login dev" no front. Registrado
  **só quando `ENVIRONMENT=development`** (registro condicional no `main.go`), então não existe em
  staging/produção.

## Regra de manutenção

Mudanças que afetam a pasta `cmd/` devem manter esses scripts e os docs (`instructions.md`, este
arquivo) em sincronia — ver `CLAUDE.md`.
