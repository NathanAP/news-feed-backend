# Contrato API ↔ Web Client (filosofias que afetam o client)

> Este arquivo está sendo enviado em conjunto com o `endpoints.md` ao client.

---

## 1. Renderização do conteúdo da notícia (o mais importante)

O campo `content` de uma notícia é **HTML sanitizado pelo servidor** (não Markdown). O client renderiza
esse HTML diretamente. A sanitização é determinística (biblioteca `bluemonday`) e é a garantia
anti-XSS do lado da API — o client **não precisa re-sanitizar**, mas deve renderizar com as práticas
normais (não injetar de formas inseguras além do render de HTML).

**O que PODE aparecer no `content`:**

- **Formatação** (sem atributos): `p`, `br`, `hr`, `span`, `strong`, `b`, `em`, `i`, `u`, `s`, `sub`,
  `sup`, `small`, `mark`, `abbr`, `cite`, `q`, `h1`–`h6`, `ul`, `ol`, `li`, `dl`, `dt`, `dd`,
  `blockquote`, `code`, `pre`, `kbd`, `samp`, `figure`, `figcaption`, `table`, `thead`, `tbody`,
  `tfoot`, `tr`, `th`, `td` (com `colspan`/`rowspan`), `caption`, `colgroup`, `col`.
- **Links** `<a href>` (sempre com `rel="nofollow"`), esquemas `http`/`https`/`mailto`. Dois tipos:
    - **Internos** (para outra notícia nossa): `href = {CLIENT_URL}/articles/{id}`. O client **deve**
      tratar essas URLs como navegação interna (rota do próprio client) e, ao abrir, disparar a marcação
      de leitura — ver §4.
    - **Externos**: qualquer outro `http(s)`/`mailto`. O client decide como abrir (ex.: `target="_blank"`).
- **Imagens** `<img src>` (com `alt`/`width`/`height`), esquemas `http`/`https`. São **externas** (do
  site da fonte) — trate lazy-loading e imagem quebrada. ⚠️ Privacidade: carregam de terceiros (podem
  ser pixels de tracking); decida se quer proxiar/bloquear no client.
- **Embeds de vídeo**: `<iframe>` **apenas** de **YouTube** (`youtube.com/embed/`,
  `youtube-nocookie.com/embed/`) e **Twitch** (`player.twitch.tv/`, `clips.twitch.tv/embed`). Vêm com
  `width`/`height`/`allowfullscreen`/`allow`/`title`/`loading`. Renderize responsivo (ex.: wrapper 16:9).
- **Instagram (e embeds baseados em `<script>`)**: **NÃO** vêm como embed. A API os converte em um
  **link** (`<a href>` para a postagem). Não espere `<blockquote class="instagram-media">` nem scripts
  do Instagram — se quiser um card rico, faça no client a partir do link.

**O que NUNCA aparece** (pode confiar): `<script>`, `<style>`, handlers de evento (`onclick`, etc.),
`<iframe>` de qualquer outro domínio, e atributos fora da lista acima — em especial **`class`, `style`,
`id`, `srcset` não existem** no conteúdo. **Consequência para o client: estilize por seletor de tag/
contexto, nunca por classe do conteúdo.** (A `ai_personality` não muda a estrutura, só o tom da tradução.)

O `content` da **tradução** (§5) segue exatamente estas mesmas regras (é re-sanitizado).

---

## 2. Como uma notícia entra num feed (julgamento)

- Um **feed** é uma coleção de **palavras-chave** (5 a 20, minúsculas, sem repetição). Quanto mais
  keywords, mais amplo o feed recebe.
- A descoberta é automática (CRON no servidor). Cada notícia nova passa por **julgamento em 2 camadas**:
    1. **Keywords** (SQL): feeds cujas keywords têm interseção com as da notícia viram candidatos.
    2. **IA**: dá um `score` 0–100 de pertencimento por candidato; passa se ≥ um threshold do servidor.
       Passou → a notícia é associada ao feed.
- **Não é retroativo**: alterar as keywords de um feed **não** re-julga notícias já existentes; o
  julgamento só acontece na descoberta. Ou seja, ao criar/editar um feed, ele **não** volta a se
  preencher com notícias antigas — vai enchendo conforme a CRON descobre coisas novas.
- Só **feeds ativos** participam. Máximo de **5 feeds ativos** por usuário.
- As keywords das notícias são **canônicas em inglês** (mesmo para notícias em outros idiomas), para
  casar fontes de idiomas diferentes. Não traduzimos keywords.

---

## 3. Sessão e autenticação (ciclo de vida)

- **Login (OAuth Google, via redirect):** o client manda o usuário para
  `GET {API}/v1/auth/google?redirect_uri=<sua_uri>` (a `redirect_uri` precisa estar na allowlist do
  servidor, `OAUTH_ALLOWED_REDIRECT_URIS`). Ao final, o servidor **redireciona de volta** para a sua
  `redirect_uri` com os tokens no **fragment** da URL:
  `#access_token=...&refresh_token=...&expires_in=...`. O client **captura do fragment e limpa o
  fragment** (history replace). Cancelamento/erro no Google volta com `?error=...` na query.
- **Tokens:** o `access_token` (JWT Bearer em toda request) expira em minutos; o `refresh_token` (é um
  **id**) dura dias.
- **Renovação:** ao receber `401` (ou perto de expirar), chame `POST /v1/auth/refresh` com
  `{ refresh_token }` → novo `access_token` (e estende o refresh). Se o refresh falhar (`401`) →
  refazer o login.
- **Claims dentro do `access_token`** (o client pode ler sem bater na API): `user_id`, `email`, `name`,
  `picture`, `created_at`, `refresh_token_id`, `language_to_translate` (anulável), `ai_personality`.
  Use para header do usuário (nome/foto) e para decidir mostrar a opção de tradução (§5).
- **Logout:** `POST /v1/auth/logout` (soft-remove do refresh). O `access_token` ainda vale até expirar.
- **Regeneração do token:** alterar preferências (e, no futuro, dados do usuário) **regenera o
  `access_token`** — a resposta já traz o novo. O client **deve substituir** o token guardado por esse.

---

## 4. Abrir uma notícia + marcação de leitura

- A URL canônica de uma notícia é `{CLIENT_URL}/articles/{id}` — **a mesma para todos os usuários** (é a
  URL que aparece nos links internos do conteúdo, §1). O client precisa ter essa rota.
- Ao abrir `/articles/{id}`, dispare `PUT /v1/articles/{id}/read`:
    - **200** = a notícia estava em ≥1 feed do usuário (marcou como lida) **ou** já estava lida (idempotente).
    - **204** = a notícia não está em nenhum feed do usuário (nada a marcar).
    - **404** = notícia inexistente.
- A notícia (`GET /v1/articles/{id}`) é uma entidade **global**: qualquer usuário autenticado vê
  qualquer notícia, mesmo fora dos seus feeds. O campo `is_read` vem enriquecido: `null` (não está em
  nenhum feed seu) | `false` (em ≥1 feed, com algum não lido) | `true` (em feeds, todos lidos).

---

## 5. Tradução (a decisão é da API, o client só mostra a opção)

- `GET /v1/articles/{id}/translate` — **sem idioma na URL**. O idioma-alvo vem da preferência
  `language_to_translate` do usuário (lida do token). Retorna
  `{ title, content, language, language_original }`, com `content` em HTML sanitizado (§1).
- **O client decide se MOSTRA o botão** de tradução com base em `language_to_translate` (null → não
  mostrar). Mas quem traduz e valida é a API — a preferência não é um gate cosmético só, ela define o alvo.
- **Cache é responsabilidade do client** (o servidor não cacheia; Redis virá no futuro).
- Keywords não são traduzidas. É read-only (não grava nada).
- Devolve **400** quando: sem alvo configurado (`language_to_translate` null), alvo == `language_original`
  da notícia, ou `language_original` desconhecido (null). O client pode antecipar esses casos
  (esconder/desabilitar a tradução) usando `language_original` da notícia + `language_to_translate` do token.

---

## 6. Preferências do usuário

- `language_to_translate` (anulável): **é dica de client + alvo de tradução**. `null` = o client **não
  mostra** a opção de tradução. Fora ser o alvo do endpoint de tradução, não afeta mais nada da API.
- `ai_personality` (`fun` | `informative` | `mixed`): tom usado na tradução.
- **Tema (light/dark) NÃO é da API** (removido na 0.33): é responsabilidade do client (ex.: localStorage).
- `GET/PUT /v1/users/me/preferences` — o `PUT` retorna o `access_token` já regenerado (§3).

---

## 7. Feeds: tela principal e novidades

- `GET /v1/feeds/{id}/articles` é a **tela principal** do usuário (as notícias do feed). Paginada.
  Filtros: `is_read` (`true`/`false`), janela `period_starting_at`/`period_ending_at` (UTC, sobre a data
  de criação), e `with_sources=true` (inclui os dados da fonte em cada notícia).
- `GET /v1/feeds/check-for-new-articles` — **poll leve**: objeto plano `{ "<feed_id>": n_não_lidas }` só
  com feeds que têm ≥1 não lida (nenhuma → `{}`). Não paginado. Bom para um "badge" de novidades.
  Futuro: pode virar SSE/WebSocket.
- Feeds são **por-usuário**: um feed de outro usuário responde **404** (a API não vaza existência). Já
  **notícias e fontes são globais** (qualquer autenticado acessa).

---

## 8. Convenções gerais que o client deve seguir

- **Datas: sempre UTC** (ISO-8601) em toda entrada e saída. Converta para o fuso local só na exibição.
- **Paginação**: `{ docs: [...], pagination: { actual_page, total_pages, actual_count, total_count,
has_next_page, has_previous_page } }`. Query `page` (≥1) e `page_size` (1–100, padrão 20). Página fora
  do range → `docs` vazio, **sem erro**.
- **Erros**: sempre `{ error: string }` + status HTTP adequado. Filosofia de "não encontrado": busca de
  **coleção** vazia → `200` com lista vazia; busca de **item único** inexistente → `404`.
- **Manutenção**: quando o servidor está desligado (`app_status=false`), **toda rota responde `503`**
  (exceto `GET /v1/health`), inclusive antes da autenticação. Trate `503` como "app em manutenção".
- **CORS**: só as origens configuradas no servidor (`CORS_ALLOWED_ORIGINS`). Auth é Bearer — **sem
  cookies/credentials**.
- **`GET /v1/health`** (sem auth) devolve `{ status, version, app_status, server_time }` — útil para um
  healthcheck/tela de status no client.

---

## Onde buscar o resto

- **Shapes exatos** de cada rota (body, query, respostas, status): `endpoints.md` (envie junto).
- **Filosofia completa** da aplicação: `PROJECT.md` (na API). Este arquivo é o recorte do que afeta o client.
- Manter este arquivo em sincronia com a API é responsabilidade do time da API — se algo aqui divergir
  do comportamento real, a API é a fonte da verdade.
