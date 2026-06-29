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

## Claims do access_token (struct `schemas.Claims`)

`user_id`, `email`, `name`, `picture`, `created_at`, `refresh_token_id`, `theme`,
`language`, `translate_content`, `ai_personality`. Ou seja, as preferências viajam no token —
alterar preferências regenera o token. Se um endpoint precisar de um dado do usuário que
**não** está no token, é preciso reavaliar (renovar/invalidar tokens).

## Middleware (`middlewares/auth.go`)

Duas etapas: (1) `parseJWT` valida assinatura/expiração e popula `claims` no contexto;
(2) `validateSession` confirma que o `refresh_token_id` do token ainda existe e está ativo
no banco (logout/invalidate derrubam a sessão imediatamente). Falha em qualquer etapa → 401.
`GetClaims(c)` recupera os claims dentro dos handlers.

## Variáveis de ambiente relevantes (apenas as chaves)

`JWT_SECRET_KEY`, `JWT_ACCESS_TOKEN_EXPIRY_MINUTES`, `JWT_REFRESH_TOKEN_EXPIRY_DAYS`,
`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`.

## Estado atual / limitações

- Cadastro é **exclusivamente** via Google.
- Não há rota de exclusão de usuário; um usuário inativo não consegue logar (ver
  `.claude/PROJECT.md` → "Usuário").
- As rotas de invalidação (`/auth/invalidate`, `/auth/invalidate-all`) estão **abertas** por
  enquanto (serão restritas a admin no futuro).
