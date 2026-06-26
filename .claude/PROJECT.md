# Projeto: API de feed de notícias personalizado

## Foco deste projeto

- Backend + API Rest

## Ideia geral

Um feed de notícias hiper personalizado que coleta, filtra, traduz e resume notícias baseado nas preferências do usuário.

## Características

- Gerencia usuários com autenticação Google.
- Permite criação de feeds personalizados através de `tags`.
- Descobre automaticamente notícias através do RSS das fontes existentes.
- Filtra notícias por palavras-chave + inteligência artificial.
- Julga à quais feeds dos usuários a notícia descoberta pertence.
- Traduz e clafica notícias através da inteligência artificial.
- Resume notícias através da IA de forma personalizada através da inteligência artificial.

## Features

- **Descoberta automática de RSS**: Detecta feeds sem input manual, apenas através da URL principal
- **Filtragem dual-layer**: Palavras-chave rápidas + IA inteligente
- **Tradução de notícias**: Traduz notícias de acordo com as preferências do usuário
- **Personalização de notícias**: Personaliza notícias de acordo com as preferências do usuário
- **Endpoints REST**: API simples e intuitiva
- **Banco SQLite integrado**: Zero dependencies de infraestrutura

## Público alvo

Usuários que querem um feed de notícias confiável e personalizável do seu jeito.

# Como funciona

## Fluxo principal

0. O usuário se cadastra através da sua conta Google.
1. O usuário cria um novo feed e o personaliza conforme preferir.
2. O sistema descobre notícias automaticamente através das fontes RSS disponíveis.
3. Cada nova notícia descoberta recebe um tratamento de tradução, melhora e atribuição de palavras-chave para ser gravada no banco de dados.
4. Identifica-se a quais feeds a notícia pertence.
5. O usuário acessa a notícia e ela é marcada como lida.
6. (opcional) O usuário requisita um resumo totalmente personalizado para aquela notícia de acordo com suas preferências.

## Fluxo de cadastro

- Usuários são cadastrados exclusivamente pelo Google.

## Autenticação

- O secret dos tokens está na variável de ambiente chamada `JWT_SECRET_KEY`.
- Fluxo de login: client redireciona o usuário para `/v1/auth/google` → Google autentica → Google redireciona para o nosso callback (`/v1/auth/google/callback`) → nosso callback retorna um JSON com o JWT → client captura esse token e passa a usar como header (`Authorization: Bearer <token>`) nas próximas chamadas.
- Tokens são divididos em dois níveis:
    - o primeiro é um `access_token` que expira de acordo com a variável de ambiente `JWT_ACCESS_TOKEN_EXPIRY_MINUTES` (em minutos) e que é usado pelo client em todas as requisições através de Authorization, como citado acima.
    - o segundo é um `refresh_token` que expira de acordo com a variável de ambiente `JWT_REFRESH_TOKEN_EXPIRY_DAYS` (em dias) e que é usado pelo client quando precisar renovar seu `access_token`. Esse token está presente em uma tabela simples chamada `refresh_tokens` que é usado silenciosamente quando for buscar um novo.
- Ao expirar um `access_token` o client chama pela endpoint de refresh (`base_url/v1/auth/refresh`) e recebe um novo `access_token` renovado. Ao fazer essa operação, uma nova data de expiração é gerada ao `refresh_token`.
- O único responsável pela renovação da data de expiração do `refresh_token` é o endpoint de `refresh` (`base_url/v1/auth/refresh`); outras regenerações, como a de alteração de preferências de usuário, por exemplo, não o fazem.
- Logout faz com que o `refresh_token` seja removido de forma soft (através do campo `status`). Naturalmente, o `access_token` será expirado em no máximo 1 hora e o usuário terá que refazer o processo de login novamente.
- O campo `status` da tabela `refresh_token` indica se o token está expirado ou não também.
- Ao gerar o `access_token` uma série de informações são inclusas nele durante sua geração. São eles:
    - `user_id`: `UUID` do usuário relacionado.
    - `email`: e-mail do usuário relacionado.
    - `name`: nome do usuário relacionado.
    - `picture`: URL da foto do usuário relacionado.
    - `created_at`: data de criação do usuário relacionado.
    - `refresh_token_id`: `UUID` do `refresh_token` relacionado.
    - `theme`: `enum` contendo o atual tema e presente nas preferências do usuário relacionado.
    - `language`: `enum` contendo o idioma preferido e presente nas preferências do usuário relacionado.
    - `translate_content`: `bool` sobre a necessidade de tradução do conteúdo das notícias e presente nas preferências do usuário relacionado.
    - `ai_personality`: `enum` contendo a personalidade da IA e presente nas preferências do usuário relacionado.
- Um struct chamado `Claims` mantém também esse mapeamento no código.

## Dados do usuário

- A edição de dados do usuário ainda não são possíveis e serão feitas futuramente.
- Alterar os dados do usuário faz com que um novo `access_token` seja gerado e retornado também pela rota, já com as novas informações atualizadas nele.
    - O `access_token` anterior (usado para ativar a atualização dos dados e agora possui informações desatualizadas) vai continuar válido até bater o tempo de expiração. Esse comportamento é considerado normal aqui pois fazem parte de um trecho não crítico da aplicação. Se em algum momento houver dados críticos ligado ao `access_token` e dados do usuário, isso terá que ser mudado.

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
- Alterar as preferências do usuário faz com que um novo `access_token` seja gerado, retornando junto ao client, já com as novas informações atualizadas nele.
    - O `access_token` anterior (usado para ativar a atualização das preferências e agora possui dados desatualizados) vai continuar válido até bater o tempo de expiração. Esse comportamento é considerado normal aqui pois fazem parte de um trecho não crítico da aplicação. Se em algum momento houver dados críticos ligado ao `access_token` e preferências do usuário, isso terá que ser mudado.

## Fontes de notícias (sources)

- As fontes de notícias são nossa principal fonte para obtenção de informações brutas.
- A visibilidade das fontes de notícias são públicas a todos os usuários do sistema.
- A manipulação (criação, edição ou remoção) de fonte de notícias é exclusiva para administradores do sistema, ou seja, para os usuários "comuns" as fontes de notícias parecem como pré-definidas.
- Duas fontes de notícias não podem ter a mesma `url` ou o mesmo `url_rss`.
- O payload de cadastro de uma fonte de notícias obriga o valor de `url_rss`. Para facilitar o encontro dessa URL, temos a rota `base_url/v1/sources/rss_discovery` que tenta descobrir automaticamente e fazer o parsing através do `gofeed` desse valor através dos seguintes padrões:
    - padrões comuns como acessar `/rss/`, `/feed/`, `/rss.xml/`, `/feed.xml`.
    - padroes de parsing HTML para encontrar `<link rel="alternate" type="application/rss+xml">`.

## Feed

- Os feeds são registros que pode ser criado livremente por qualquer usuário e ele só pode ser visualizado pelo usuário que o criou.
- Os feeds devem possuir pelo menos 5 palavras-chave com limite de 20.
    - O campo de palavras-chave é uma lista de `string`(no formato `JSON array (TEXT)`).
    - Quanto mais palavras-chave um feed tem, mais amplo vai ser o recebimento de notícias durante o julgamento.
- Um usuário pode ter até 5 feeds.
- Alterar as palavras-chave de um feed não faz com que um novo processo de julgamento das notícias aconteça.

## Notícias

- As notícias são o principal motivo da aplicação existir e podem ser sub-entendidas com a nomenclatura "artigo" também.
- As notícias são descobertas automaticamente através das fontes de notícias.
- Para uma notícia ser descoberta e registrada, o RSS de cada fonte registrada é consultado de tempos em tempos. Ao notar uma nova notícia presente, uma inteligência artificial é acionada para tratar o conteúdo e gravar essa versão em nosso banco de dados.
- Um usuário vai ter acesso ao registro da notícia quando ela for julgada como hábil a estar no feed que ele cadastrou.
- As notícias não podem ser criadas manualmente, porém podem ser editadas ou excluídas por usuário administradores.
    - Essa regra existe apenas para casos extremos de uma notícia que saiu do controle.
- Quando descobertas, as notícias passam por um julgamento através de uma inteligência artificial para definir palavras-chave às quais ela pertence. As palavras-chave definidas servirão como base para saber em quais feeds ela aparecerá ou não.
- As notícias devem possuir pelo menos 5 palavras-chave com limite de 20.
    - O campo de palavras-chave é uma lista de `string` (no formato `JSON array (TEXT)`).
    - Quanto mais palavras-chave uma notícia tem, mais amplo vai ser a distribuição aos feeds durante o julgamento.
- Alterar as palavras-chave de uma notícia não faz com que um novo julgamento aconteça.

## Feed x Notícias

- O feed e as notícias se relacionam de forma múltipla, ou seja, uma feed pode possuir diversas notícias e as notícias podem estar presentes em diversos feeds.
- Isso implica em uma tabela relacional (junction table) na qual armazenamos o `id` d feed assim como o `id` da notícia.
    - Essa tabela é preenchida automaticamente quando uma notícia é atrelada a um feed.
    - Essa tabela perde o registro automaticamente quando uma notícia é retirada daquele feed na qual estava associado (provavelmente por alguma opção implementada no futuro).
- Registros nesta tabela são removidos permanentemente ao serem excluídos (hard remove).
- A população dessa tabela acontece no momento na qual uma nova notícia é descoberta e julgada como hábil a estar naquele feed.

## Descobrindo uma notícia

## Julgando se uma notícia está no feed do usuário ou não

- Quando uma notícia é descoberta um processo de julgamento é acionado para saber a qual feed aquela notícia será associada.
- O julgamento funciona através de duas camadas que define se os registros estão relacionados ou não:
    - A primeira camada é a mais simples envolvendo uma comparação de palavras-chave da notícia descoberta com o feed existente.
    - A segunda camada é a garantia através da IA que define um `score` entre 0 e 100 (threshold configurável) e define quanto aquela notícia pertence ao feed.
    - Quando forem julgados como associados, um novo registro na tabela associativa (junction table) entre feed e notícias é criado.
- O julgamento de notícias nunca é retroativo.

## Administradores

- Ainda não há uma implementação de administradores por enquanto.
- Ações que deveriam ser feitas pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
- Endpoints que deveriam ser acessados pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
    - Atualmente são eles: `DELETE base_url/v1/auth/invalidate`, `DELETE base_url/v1/auth/invalidate_all`, `POST base_url/v1/sources/create`, `PUT base_url/v1/sources/{id}`, `DELETE base_url/v1/sources/{id}`

## Exclusão de registros

- A filosofia do projeto para exclusão de registros segue a convenção de soft remove.
- As tabelas de relacionamento (junction tables) são as exceções dessa regra. Elas devem sofrer hard remove quando perderem um registro.
- Registros que estão sendo excluído por soft remove devem:
    - Ficar com o campo `status` marcados em `false` e com o campo `removed_at` preenchidos.
    - Continuar presente nas atuais tabelas de relacionamento (junction tables).
- Registros que estão sendo excluído por hard remove devem:
    - Ter seus relacionamentos através das tabelas de relacionamento (junction tables) também excluídos.
- Registros excluídos em soft remove não podem ser considerados na hora de:
    - Serem manipulado.
    - Serem buscados individualmente (através do `id`).
    - Serem buscados em grupo (listados através de filtragem).
    - Serem buscados através de suas tabelas relacionais (junction tables).
    - Participarem de estatísticas, indicadores ou métricas.
    - Bloquearem a criação de novos registros através de constraints de unicidade.
- Toda query de visualização ou busca deve considerar apenas campos em `status` considerados ativos (`1`, `active` ou equivalente) e com o campo `removed_at` nulo (vazio, `null` ou equivalente).
- Campos de unicidade devem valer apenas entre registros ativos, ou seja, devem ser implementadas como índices únicos parciais com `WHERE removed_at IS NULL`. Assim, um registro soft-deleted nunca impede a criação de um novo registro equivalente.
    - Exemplo: o campo `url` de souces é único mas ele não deve concorrer na unicidade com registros inativos.

## Ambiente de testes

As regras abaixo devem estar presente durante qualquer teste proposto:

- Devem garantir que não podem ser executados em ambiente de produção ou homologação.
- Durante a necessidade de banco de dados, devem garantir que estão sendo usados bases temporárias exclusivamente para cumprir seus objetivos.

### Mocks

- Servem para simular dados reais mas sem precisar de um serviço para obtenção.
- Devem ser o mais próximo da realidade possível.
- Não podem conter informações ou dados considerados sensíveis, proibidos ou ofensivos.
- Os controllers e a camada de queries (`sqlc`) são testados via testes de integração com banco em memória real ao invés de mocks de repositório escritos à mão pois correm o risco de ficar desatualizados a cada nova query.
    - Se um dia for necessário isolar um controller em teste unitário, o mock deve ser gerado a partir da interface `db.Querier` (ex: `mockgen`) e nunca escrito manualmente para evitar o esquecimento em uma atualização.

### Fixtures

- Servem como um atalho para diversas operações repetitivas dos testes, como por exemplo simular a criação de um usuário ou a renovação de um `access_token`.

### Unitários

- Servem apenas para nos garantir que os dados que chegam sejam validados corretamente e que suas respostas sejam adequadas aos problemas e sucessos encontrados.
- Devem ser bastante simples e diretos, apenas simulando chamadas chegando e saindo na API.
- Transações no banco de dados são descartados durante este teste.
- Não dependem que a API ou qualquer serviço esteja de pé para serem feitas.

### Integração

- Servem para nos garantir que as integrações da aplicação cumprem seu papel mínimo. As atuais integrações e objetivos dos testes nelas são:
    - API interna: garantir a capacidade de ficar online, aceitar requisições e garantir respostas adequadas conforme cada situação proposta por cada endpoint.
        - Exemplo: se o endpoint `base_url/v1/users/me` se propõe a responder por 200 e 401, ambas as situações devem ser testadas adequadamente.
        - A nuance de simplicidade entre o teste unitário e teste de integração neste caso é bem baixa e isso pode ser considerado normal.
    - Banco de dados interno: garantir a capacidade de fazer o CRUD básico proposto pela pasta e arquivos presentes em `raiz/sqlc/`.
        - O banco de dados utilizado deve ser sempre em memória durante este teste.
    - `OAuth2`: garantir que a configuração atende aos requisitos mínimos para funcionamento natural do processo de testes (Exemplo: checagem de variáveis de ambiente `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` e `GOOGLE_REDIRECT_URL`).
- Devem ser menos simples e diretos em comparação aos unitários, mas acabam por abranger mais setores do código.
- Dependem que os serviços estejam de pé para funcionamento garantido e correto.

### End-to-end

- Servem para nos garantir que a lógica dos endpoints e seus códigos estão em perfeito estado, ou seja, os endpoints devem ser chamados diretamente de forma explícita.
- O banco de dados utilizado deve ser sempre em memória durante este teste.
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

### Utils

- Servem como scripts utilitários para a execução de testes. Sinta-se livre para criar qualquer utilidade aqui, como subir uma instância SQLite temporária com as migrações, por exemplo.

## Ambiente

- O atual ambiente sempre está na variável de ambiente chamada `ENVIRONMENT` e devem estar sempre em um desses três valores:
    - "development": ambiente de desenvolvimento.
    - "staging": ambiente de homologação.
    - "production": ambiente de produção.

## Logs / debug manual

- O objetivo é ter melhor visão dos acontecimentos do que acontece na execução da aplicação, portanto não faz parte da observabilidade.
- Serve apenas para mostrar qualquer tipo de dado em uma determinada cor (padrão azul) no terminal.

## Observabilidade

- Ainda não está implementado e será feito futuramente.

# Aplicativos externos

## Bruno

- O Bruno é um aplicativo usado para organizar coleções de requisições para potencializar e automatizar testes.
- Uma documentação para o aplicativo é mantido na pasta `raiz/bruno` que utiliza os padrões do aplicativo e são gerenciados por você.
- No atual momento, o aplicativo será usado internamente para testes rápidos, mas futuramente será criada rotinas de testes dentro do aplicativo para alavancar a manutenção e garantia da funcionalidade da aplicação.
