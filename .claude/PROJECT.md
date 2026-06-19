# Projeto: API de feed de notícias personalizado

## Foco deste projeto

- Backend + API Rest

## Ideia geral

Um feed de notícias hiper personalizado que coleta, filtra, traduz e resume notícias baseado nas preferências do usuário. O sistema:

- Gerencia usuários com autenticação Google.
- Permite cadastro de categorias temáticas (Metallica, Anime, Tech, etc).
- Descobre automaticamente feeds RSS das fontes informadas.
- Filtra notícias por palavras-chave + IA (Gemini 2.5 Flash).
- Traduz notícias através da IA (Gemini 2.5 Flash).
- Personaliza notícias através da IA (Gemini 2.5 Flash).
- Entrega notícia personalizada.

## Features

- **Descoberta automática de RSS**: Detecta feeds sem input manual, apenas através da URL principal
- **Filtragem dual-layer**: Palavras-chave rápidas + IA inteligente
- **Tradução de notícias**: Traduz notícias de acordo com as preferências do usuário
- **Personalização de notícias**: Personaliza notícias de acordo com as preferências do usuário
- **Endpoints REST**: API simples e intuitiva
- **Banco SQLite integrado**: Zero dependencies de infraestrutura

## Público alvo

Usuários que buscam personalizar seus feeds de notícias do seu jeito.

# Como funciona

## Fluxo principal

0. O usuário se cadastra através da sua conta Google.
1. O usuário cria uma categoria "Metallica" e seleciona sua(s) fonte(s) de notícias.
2. O sistema descobre automaticamente RSS de fontes relacionadas informadas pelo usuário.
3. Novas notícias são descobertas automaticamente.
    - Um processo percorre cada fonte cadastrada em busca de notícias recentes dela.
4. Identifica-se a qual usuário aquela notícia pertence.
    - As notícias sobre a categoria cadastrada aparecem em tempo real de forma personalizada àquele usuário.
    - Os usuários com a mesma categoria e fonte cadastrada acabam recebendo diferentes versões por conta da hiper personalização ().
5. Verifica-se as prefêrencias do usuário que receberá a notícia e a personaliza.
6. O usuário vê a notícia e ela é marcada como lida.

## Fluxo de cadastro

- Usuários são cadastrados exclusivamente pelo Google.

## Autenticação

- O secret dos tokens está na variável de ambiente chamada `JWT_SECRET_KEY`.
- Fluxo de login: client redireciona o usuário para `/v1/auth/google` → Google autentica → Google redireciona para o nosso callback (`/v1/auth/google/callback`) → nosso callback retorna um JSON com o JWT → client captura esse token e passa a usar como header (`Authorization: Bearer <token>`) nas próximas chamadas.
- Tokens são divididos em dois níveis:
    - o primeiro é um `access_token` que expira de acordo com a variável de ambiente `JWT_ACCESS_TOKEN_EXPIRY_MINUTES` (em minutos) e que é usado pelo client em todas as requisições através de Authorization, como citado acima.
    - o segundo é um `refresh_token` que expira de acordo com a variável de ambiente `JWT_REFRESH_TOKEN_EXPIRY_DAYS` (em dias) e que é usado pelo client quando precisar renovar seu `access_token`. Esse token está presente em uma tabela simples chamada `refresh_tokens` que é usado silenciosamente quando for buscar um novo.
- Ao expirar um `access_token` o client chama pela rota de refresh (`base_url/v1/auth/refresh`) e recebe um novo `access_token` renovado. Ao fazer essa operação, uma nova data de expiração é gerada ao `refresh_token`.
- Logout faz com que o `refresh_token` seja removido de forma soft - através do campo `status`. Naturalmente, o `access_token` será expirado em no máximo 1 hora e o usuário terá que refazer o processo de login novamente.
- O campo `status` da tabela `refresh_token` indica se o token está expirado ou não também.

## Preferências do usuário (user preferences)

- O usuário tem liberdade de alterar suas preferências para utilização do sistema da forma que preferir.
- Usuários removidos (`status` em `false`) devem ficar com suas preferências excluídas também (`status` também deve ser setado para `false`)
- O idioma preferido não afeta em nada das respostas da API.
- Apenas os próprios usuários podem alterar suas preferências.

## Categorias de notícias

- As categorias de notícias são manipuladas apenas pelo seu usuário associado.
- As keywords são uma lista de palavras-chave 100% abertas e controladas pelo usuário e que serão utilizados pelas fontes para entregar novas notícias a ele.
- As categorias precisam de pelo menos uma fonte de notícias obrigatoriamente.

## Fontes de notícias (sources)

- As fontes de notícias são nossa principal fonte para obtenção de informações brutas.
- As visibilidade das fontes de notícias são públicas a todos os usuários do sistema.
- A manipulação (criação, edição ou remoção) de fonte de notícias é exclusiva para administradores do sistema, ou seja, para os usuários "comuns" as fontes de notícias parecem como pré-definidas.
- Duas fontes de notícias não podem ter a mesma `url` ou o mesmo `url_rss`.
- O payload de cadastro de uma fonte de notícias obriga o valor de `url_rss`. Para facilitar o encontro dessa URL, temos a rota `base_url/v1/sources/rss_discovery` que tenta descobrir automaticamente e fazer o parsing através do `gofeed` desse valor através dos seguintes padrões:
    - padrões comuns como acessar `/rss/`, `/feed/`, `/rss.xml/`, `/feed.xml`.
    - padroes de parsing HTML para encontrar `<link rel="alternate" type="application/rss+xml">`.

## Categorias de notícias x Fontes de notícias

- As categorias e fontes de notícias se relacionam de forma múltipla, ou seja, uma categoria pode possui diversas fontes e as fontes podem estar presentes em diversas categorias.
- Isso implica em uma tabela relacional na qual armazenamos o `id` da categoria assim como o `id` da fonte.
    - Essa tabela é preenchida automaticamente quando um usuário associa em sua categoria uma fonte.
    - Essa tabela perde o registro automaticamente quando um usuário desassocia em sua categoria uma fonte.

# Administradores

- Ainda não há uma implementação de administradores por enquanto.
- Ações que deveriam ser feitas pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
- Endpoints que deveriam ser acessados pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
    - Atualmente são eles: `DELETE base_url/v1/auth/invalidate`, `DELETE base_url/v1/auth/invalidate_all`, `POST base_url/v1/sources/create`, `PUT base_url/v1/sources/{id}`, `DELETE base_url/v1/sources/{id}`
