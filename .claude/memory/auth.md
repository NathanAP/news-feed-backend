# Autenticação

> Resumo. Detalhes em `.claude/PROJECT.md` (seção Autenticação).

## Fluxo de login (Google OAuth2 + JWT)

1. Client redireciona para `GET /v1/auth/google` → Google autentica.
2. Google chama `GET /v1/auth/google/callback` → o backend troca o code, busca o perfil,
   cria/encontra o usuário e devolve JSON `{ access_token, refresh_token, expires_in }`.
3. Client usa `Authorization: Bearer <access_token>` nas chamadas seguintes.
4. Ao expirar o `access_token`, client chama `POST /v1/auth/refresh` com `{ refresh_token }`
   e recebe um novo `access_token` (e o `refresh_token` tem o `expires_at` renovado).

Login novo **revoga todas as sessões anteriores** do usuário (política de sessão única).
Logout faz soft-remove do `refresh_token`; o `access_token` segue válido até expirar
(máx. ~1h).

## Tokens

- **access_token**: JWT HS256, expira em `JWT_ACCESS_TOKEN_EXPIRY_MINUTES`. Segredo em
  `JWT_SECRET_KEY`.
- **refresh_token**: registro na tabela `refresh_tokens`; o `id` do registro É o valor do
  token. Expira em `JWT_REFRESH_TOKEN_EXPIRY_DAYS`. Só o endpoint `/auth/refresh` renova a
  expiração.

## Atividade do usuário (`last_active_at`, 0.43)

O login, o `/auth/refresh` e o `dev-login` carimbam `users.last_active_at` com a hora atual. É o sinal
de **atividade** usado pela descoberta para pular feeds de usuários inativos (não confundir com
`last_login_at`, que só muda no login completo do Google). Não vai no `access_token` — é campo só de
banco. Ver `database.md` e `PROJECT.md` (seção Usuário / "Busca por usuários aptos").

## Claims do access_token (struct `schemas.Claims`)

`user_id`, `email`, `name`, `picture`, `created_at`, `refresh_token_id`,
`language_to_translate` (anulável), `ai_personality`, `admin`. Ou seja, as preferências viajam no
token — alterar preferências regenera o token. Se um endpoint precisar de um dado do usuário que
**não** está no token, é preciso reavaliar (renovar/invalidar tokens).
A rota de tradução (`GET /articles/:id/translate`) usa o `language_to_translate` do token como
idioma-alvo (nulo = sem alvo → 400); a `ai_personality` ajusta o tom.

O claim `admin` é **dica de client**, não autorização: ver a seção de autorização abaixo.

## Middleware (`middlewares/auth.go`)

Duas etapas: (1) `parseJWT` valida assinatura/expiração e popula `claims` no contexto;
(2) `validateSession` confirma que o `refresh_token_id` do token ainda existe e está ativo
no banco (logout/invalidate derrubam a sessão imediatamente). Falha em qualquer etapa → 401.
`GetClaims(c)` recupera os claims dentro dos handlers.

As duas peças internas (`parseAccessToken` e `checkSession`) são exportadas dentro do pacote porque o
guard de manutenção também precisa identificar quem está chamando, e ele roda **antes** de qualquer
autenticação por rota — sem isso haveria uma segunda cópia das regras, fadada a divergir.

## Autorização de administrador (`middlewares/admin.go`, 0.40)

- `AdminResolver` é a única definição de "é administrador" da aplicação, e ela **sempre lê
  `users.admin` no banco** — nunca o claim. Motivo: se a decisão viesse do token, revogar o acesso de
  alguém só valeria quando o token expirasse. O custo é uma query, paga só nas rotas de administrador.
- `RequireAdmin` é montado **depois** do `authMiddleware` (ele responde "pode?", não "quem é?").
  Não-admin → **403**; falha de banco → **500**, nunca 403.
- `AdminResolver.IsRequestFromAdmin` serve o guard de manutenção: parseia o token do request cru,
  valida a sessão e lê a flag. É fail-closed — qualquer coisa não confirmada vira "usuário comum".
- Criar administrador não passa pelo login: a query `CreateUser` não escreve a coluna. Promoção é
  `SetUserAdmin`, hoje chamada só pelo seed de desenvolvimento.

## Manutenção e o grupo `/auth`

Todo o grupo `/v1/auth` é **isento** do guard de `app_status` (junto com `/v1/health` e
`PUT /v1/system/app-status`). Não é conveniência: o bypass identifica o administrador pelo
`access_token`, que expira em cerca de uma hora, e o `refresh_token` não carrega identidade que o
bypass leia. Com o `/auth` atrás do guard, um administrador cujo token expirasse durante a manutenção
tomaria 503 do próprio `refresh` e ficaria trancado fora da aplicação, sem conseguir religá-la.

## Variáveis de ambiente relevantes (apenas as chaves)

`JWT_SECRET_KEY`, `JWT_ACCESS_TOKEN_EXPIRY_MINUTES`, `JWT_REFRESH_TOKEN_EXPIRY_DAYS`,
`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`.

## Estado atual / limitações

- Cadastro é **exclusivamente** via Google.
- Não há rota de exclusão de usuário; um usuário inativo não consegue logar (ver
  `.claude/PROJECT.md` → "Usuário").
- As rotas de invalidação (`/auth/invalidate`, `/auth/invalidate-all`) são exclusivas de
  administrador desde a 0.40 — até então estavam abertas, sem exigir nem token.
- Não há endpoint para promover alguém a administrador: é alteração manual no banco (em dev, o
  usuário do seed já nasce administrador).
