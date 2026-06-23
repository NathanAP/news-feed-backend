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

## Ambiente

- O atual ambiente sempre está na variável de ambiente chamada `ENVIRONMENT` e devem estar sempre em um desses três valores:
    - "development": ambiente de desenvolvimento.
    - "staging": ambiente de homologação.
    - "production": ambiente de produção.

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

- Ao criar um usuário, suas preferências devem ser criadas automaticamente também.
    - Valores padrão:
        - modo dark
        - idioma português
        - traduzir conteúdo em `true`
        - personalidade em `misto`
- O usuário tem liberdade de alterar suas preferências para utilização do sistema da forma que preferir.
- Usuários removidos (`status` em `false`) devem ficar com suas preferências excluídas também (`status` também deve ser setado para `false`)
- O idioma preferido não afeta em nada das respostas da API.
- Apenas os próprios usuários podem alterar suas preferências.
- Alterar as preferências do usuário faz com que um novo `access_token` seja gerado e retornado também pela rota, já com as novas informações atualizadas nele.

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
- Isso implica em uma tabela relacional (junction table) na qual armazenamos o `id` da categoria assim como o `id` da fonte.
    - Essa tabela é preenchida automaticamente quando um usuário associa em sua categoria uma fonte.
    - Essa tabela perde o registro automaticamente quando um usuário desassocia em sua categoria uma fonte.
- Registros nesta tabela são removidos permanentemente ao serem excluídos (hard remove).

# Administradores

- Ainda não há uma implementação de administradores por enquanto.
- Ações que deveriam ser feitas pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
- Endpoints que deveriam ser acessados pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
    - Atualmente são eles: `DELETE base_url/v1/auth/invalidate`, `DELETE base_url/v1/auth/invalidate_all`, `POST base_url/v1/sources/create`, `PUT base_url/v1/sources/{id}`, `DELETE base_url/v1/sources/{id}`

# Ambiente de testes

As regras abaixo devem estar presente durante qualquer teste proposto:

- Devem garantir que não podem ser executados em ambiente de produção ou homologação.
- Durante a necessidade de banco de dados, devem garantir que estão sendo usados bases temporárias exclusivamente para cumprir seus objetivos.

## Mocks

- Servem para simular dados reais mas sem precisar de um serviço para obtenção.
- Devem ser o mais próximo da realidade possível.
- Não podem conter informações ou dados considerados sensíveis, proibidos ou ofensivos.

## Fixtures

- Servem como um atalho para diversas operações repetitivas dos testes, como por exemplo simular a criação de um usuário ou a renovação de um `access_token`.

## Unitários

- Servem apenas para nos garantir que os dados que chegam sejam validados corretamente e que suas respostas sejam adequadas aos problemas e sucessos encontrados.
- Devem ser bastante simples e diretos, apenas simulando chamadas chegando e saindo na API.
- Não dependem que a API ou qualquer serviço esteja de pé para serem feitas.

## Integração

- Servem para nos garantir que as integrações da aplicação cumprem seu papel mínimo. As atuais integrações e objetivos dos testes nelas são:
    - API interna: garantir a capacidade de ficar online, aceitar requisições e garantir respostas adequadas conforme cada situação proposta por cada endpoint.
        - Exemplo: se o endpoint `base_url/v1/users/me` se propõe a responder por 200 e 401, ambas as situações devem ser testadas adequadamente.
        - A nuance de simplicidade entre o teste unitário e teste de integração neste caso é bem baixa e isso pode ser considerado normal.
    - Banco de dados interno: garantir a capacidade de fazer o CRUD básico proposto pela pasta e arquivos presentes em `raiz/sqlc/`.
    - `OAuth2`: garantir que a configuração atende aos requisitos mínimos para funcionamento natural do processo de testes (Exemplo: checagem de variáveis de ambiente `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` e `GOOGLE_REDIRECT_URL`).
- Devem ser menos simples e diretos em comparação aos unitários, mas acabam por abranger mais setores do código.
- Dependem que os serviços estejam de pé para funcionamento garantido e correto.

## End-to-end

- Servem para nos garantir que a lógica das rotas e seus códigos estão em perfeito estado.
- Devem evitar ao máximo o uso de mocks, apenas para casos especiais. São eles:
    - `OAuth2`: é impossível simular os cliques e processos da autenticação feita no Google. A simulação aqui pode cobrir estas situações de forma totalmente aceitável:
        - a de que um usuário fez este processo com sucesso.
        - a de que um usuário fez este processo com falhas.
        - a de que um usuário cancelou este processo.
- Devem representar o máximo de situações que uma rota pode oferecer, aqui estão alguns exemplos para abrir sua mente:
    - Rota inexistente.
    - Falta de autenticação quando há obrigatoriedade, `access_token` inválido ou `refresh_token` inválido ou inexistente.
    - Header inválido, inexistente ou inútil.
    - Body inválido, inexistente ou inútil.
    - Query ou filtragem inválida ou inexistente.
- Dependem que os serviços estejam de pé para funcionamento garantido e correto.

## Utils

- Servem como scripts utilitários para a execução de testes. Sinta-se livre para criar qualquer utilidade aqui, como subir uma instância SQLite temporária com as migrações, por exemplo.

# Logs

- A pasta `middlewares` contém um handler simples que é capaz de ser chamado em qualquer lugar da aplicação.
- Serve apenas para mostrar qualquer tipo de dado em uma determinada cor (padrão azul) no terminal.
