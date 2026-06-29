# Convenções do código

Aqui estão as convenções de código que devem ser seguidas para garantir um código limpo, consistente e de fácil manutenção:

- Código estrito, altamente tipado.
- Garanta sempre estar o mais próximo da orientação à objetos.
- Variáveis, comentários e mensagens finais sempre em inglês.
- Garanta que variáveis tenham nomes claros.
- Nunca use IDs numéricos, sempre utilize UUID.
- Prefira o uso de Enums em campos de múltipla escolha ao invés de string crua.
- Use context.Context para operações async.
- Sempre retorne erros tipados.
- Use pointers para mutação, values para imutabilidade.
- Utilize arquivos de ambiente local (arquivos .env) para guardar informações secretas ou confidenciais.
- Utilize arquivos de ambiente local (arquivos .env) para preferências de modo desenvolvimento, homologação ou produção (Exemplo: ENVIRONMENT, MAILER_ACTIVE, NOTIFICATOR_ACTIVE).
- Prefira manter fluxo de regras obrigatórias dentro dos arquivos de `controllers` presente em `raiz/services/controllers`, assim os mesmos fluxos sempre serão seguidos corretamente e nunca teremos problemas de dependências.
    - Por exemplo: ao criar um usuário a regra obrigatória é criar também uma preferência de usuário em seguida, assim como quando o usuário sofrer soft-remove, o registro das suas preferências també sofre soft-remove. Todo esse código de dependêcia lógica deve estar no arquivo `raiz/services/controllers/users.go`.

# Versionamento

- Utilize a pasta `.claude/versions` para especificar o que foi feito em cada versão por você.
    - Arquivos nesta pasta sempre em formato `md`.
    - A nomenclatura de arquivos nesta pasta deve ser `timestamp_versão`, por exemplo `20260623090000_0.1.0.0.md`.
    - Perceba que este arquivo também é uma ótima fonte de informações para entender as mudanças em qualquer altura da vida útil da aplicação.
    - Pode colocar bastante detalhes caso ache necessário.
- As versões devem seguir o padrão `major.minor.patch.docs`.
    - Subir uma versão também zera todos à sua direita. Ou seja:
        - se a versão `10.5.2.14` sofrer uma atualização `docs`, a nova versão é `10.5.2.15`.
        - se a versão `10.5.2.15` sofrer uma atualização `patch`, a nova versão é `10.5.3.0`.
        - se a versão `10.5.3.0` sofrer uma atualização `minor`, a nova versão é `10.6.0.0`.
        - se a versão `10.6.0.0` sofrer uma atualização `major`, a nova versão é `11.0.0.0`.
- Ao concluir uma alteração major, minor ou patch o agente `test_manager` deve ser acionado para que seja efetuada uma nova rotina de testes. A falha dessa rotina deve impedir a continuidade do processo de desenvolvimento.

# Convenções de workaround

- Workarounds são necessários mas a preferência é no ajuste do código para evitar essas necessidades. Sabemos que um refatoramento é demorado e perigoso, mas quando necessário, tem que ser feito o quanto antes para evitar problemas maiores no futuro.
- Ao notar que um workaround é necessário, sempre avise e explique o motivo e a solução proposta. Caso seja muito grande ou seja considerada uma gambiarra de código, considere parar o processo para falar sobre isso.

# Convenções de arquivos e pastas

- Nomes de arquivos e pastas devem estar em snake case (Exemplo: user.go, user_preference.go).
- Nomes de arquivos de testes deve utilizar sufixo "\_test" (Exemplo: user_test.go, user_prefence.go).

# Convenções de banco de dados

- Nomes de tabelas do banco de dados devem estar no plural e em snake case (Exemplo: users, user_preferences).
- Tabelas de relacionamento (junction tables) devem conter os dois nomes das tabelas envolvidas. Exemplo: `articles_feeds` pertence ao relacionamento entre as tabelas de notícias e feeds.
- `id` deve ser do tipo UUID v7 e devem ser imutáveis.
- As tabelas devem ter pelo menos os campos `id`, `status`, `created_at`, `modified_at` e `removed_at` do tipo timestamp.
    - A exceção imediata dessa regra são as tabelas de relacionamento (junction tables) que devem ter pelo menos o campo `id`.
- As tabelas de relacionamento (junction tables) devem conter registros de `id` existentes nas tabelas relacionadas, por exemplo, se a tabela contém um `user_id`, todos os registros devem ter um `user_id` válido.
- O campo `status` deve ser utilizado conforme regras em `PROJECT.md` para indicar o estado de um registro no banco de dados.
- Ao alterar um registro, o campo `modified_at` deve ser atualizado com o timestamp atual.
- Registros de uma tabela de relacionamento (junction tables) só podem ser considerados válidos quando todos os registros relacionados estiverem ativos (`status` marcados em `true` e com `removed_at` sem valor).
    - Isso implica que, ao buscar um registro na tabela relacional (junction table), além do `join` por `id` também faz-se necessário uma validação pelo `status` e `removed_at` para garantir que aquele registro está apto a estar no resultado da query.
- Tabelas de relacionamento (junction tables) não sofrem alterações em campos relacionados a `id`. Para "trocar" qualquer um dos `id` do relacionamento, o registro antigo deve ser removido e um novo registro deve ser criado.
    - Por exemplo, se a tabela de relacionamento (junction table) AB possui `id`, `a_id` e `b_id`, nenhum deles podem ser alterados.
    - Campos "extras" (não relacionados à `id`) dessas tabelas ainda podem ser alterados normalmente.
- Exclusões de registros em tabelas comuns devem ser feitas através de soft delete, utilizando um campo booleano chamado `status` e um campo de timestamp `removed_at`.
    - Nesse caso, o campo `removed_at` deve ser atualizado com o timestamp atual e o campo `status` deve ser definido como false (0).
    - A exceção imediata dessa regra são as tabelas de relacionamento (junction tables) que devem ser feitas através de hard delete, ou seja, o registro deve ser removido da tabela permanentemente.
- Registros que pertencem exclusivamente a um usuário não podem ser encontrados por outros usuários.
    - Isso implica que ao buscar um desses registros, é obrigatório também a passagem do `user_id` que está procurando o registro.
    - São eles:
        - Preferências de usuário.
        - Feed.
        - Article x Feed (junction table): neste caso, deve-se olhar pelo `user_id` presente na tabela `feed`.
- As regras de `cascade` devem sempre ser elaboradas durante o processo de planejamento de cada versão.
    - Ao realizar uma exclusão em um registro (tanto soft quanto hard remove), todas as tabelas dependentes deste registro devem sofrer o efeito de `cascade`, ou seja, também são excluídos seguindo o padrão de exclusão (soft ou hard).
- A desnormalização entre tabelas não é recomendada para evitar joins pois o custo é baixo comparada à bagunça que o código pode acabar se tornando ao quebrar esse paradigma.

## Transações

- As transações do banco de dados sempre são executadas fora do `controller`. Isso significa que os métodos presentes na pasta `controllers` são apenas métodos intermediários, ou seja, eles não são o passo final para uma operação no banco de dados. Isso evita a situação onde temos que criar um número elevado de registros de uma vez sem correr o risco de falhar no meio do caminho e por conta disso sobrar registros órfãos.
    - Essa necessidade surgiu a partir do momento que precisamos fazer mais de uma operação por vez (como gravar o usuário e depois suas preferências ao mesmo tempo). Fazer o primeiro modelo ser gravado sem garantia do sucesso da segunda gravação tornaria o primeiro registro um órfão.
- O responsável pelo `commit` ou `rollback` é sempre a conclusão do método `WithTransaction` presente no arquivo `raiz/services/controllers/transaction.go`.
    - Isso significa que para fazer qualquer transação no banco de dados, o método `WithTransaction` precisa ser chamado.
    - Ao manipular o banco de dados é papel do `controller` manter os valores atualizados, seja através da instância original ou do `return` do método. Tenha atenção às situações de hard remove de registros (como para tabelas de relacionamento (junction tables)).
- Métodos que não envolvem manipulação direta (como buscas, geração de relatórios) também usam `WithTransaction` mesmo que um `commit` ou `rollback` não seja aplicado.

# Convenções de migrações

- Migrações devem ser criadas utilizando o `goose` e seguindo a convenção de nomeação de arquivos (Exemplo: `20240101120000_create_users_table.go` para criar a tabela de usuários).
- Migrações devem ser versionadas e aplicadas em ordem cronológica.
- Migrações devem sempre conter um up e um down funcionais.
- Testes automatizados de migrações serão implementados futuramente.

# Convenções da API

- Todos os dados e preferências públicas do usuário estão no `access_token`. Se um endpoint precisar de um dado ou uma preferência do usuário não mapeada no `access_token`, avise-nos para que possamos tomar as medidas necessárias, como invalidar tokens antigos ou renovar os tokens automaticamente.
- Seguir os padrões de respostas tradicionais para RESTful com os códigos de status HTTP apropriados.
- Middlewares devem estar na pasta `raiz/middlewares/` e seguir a convenção de nomeação de arquivos (Exemplo: `auth.go` para middleware de autenticação, `logging.go` para middleware de logging).
- Endpoints devem estar em seu próprio arquivo, organizado dentro de uma pasta do modelo correspondente (Exemplo: `raiz/services/endpoints/v1/users/` para endpoints relacionados a usuários; arquivo `login.go` corresponde à rota de login do usuário).
- Endpoints devem seguir o padrão `base_url/versao_da_api/modelo/acao` (Exemplo: `http://localhost:3000/v1/users/create`, `http://localhost:3000/v1/users/login`).
- Endpoints de criação deve sempre ser um POST e seguir o padrão `base_url/versao_da_api/modelo/create` (Exemplo: `http://localhost:3000/v1/sources/create`).
- Endpoints de atualização deve sempre ser um PUT e seguir o padrão `base_url/versao_da_api/modelo/{id}` (Exemplo: `http://localhost:3000/v1/sources/{id}`).
- Endpoints de remoção deve sempre ser um DELETE e seguir o padrão `base_url/versao_da_api/modelo/{id}` (Exemplo: `http://localhost:3000/v1/sources/{id}`).
- Endpoints de pesquisa por ID deve sempre ser um GET e seguir o padrão `base_url/versao_da_api/modelo/{id}` (Exemplo: `http://localhost:3000/v1/sources/{id}`).
- Endpoints de pesquisa por múltiplos parâmetros deve sempre ser um GET e seguir o padrão `base_url/versao_da_api/modelo?parametro1=valor1&parametro2=valor2` (Exemplo: `http://localhost:3000/v1/sources?url=example&url_rss=example`).
    - O campo `status` nunca deve ser exposto como filtro de busca: registros inativos jamais podem ser retornados, conforme as regras de `status` em `PROJECT.md`.
- Endpoints criados devem ser acrescentados nos testes da pasta `tests/` seguindo as convenções de testes abaixo.
- Endpoints alterados devem ser corrigidos (caso necessário) nos testes da pasta `tests/` seguindo as convenções de testes abaixo.
- Endpoints removidos devem ser removidos dos testes da pasta `tests/`.
- Endpoints devem possuir logs como especificado na sessão de logs / debug manual.
- A filosofia para endpoints que retornam um resultado equivalente a dizer "não foi encontrado" deve ser a seguinte:
    - Se o papel de um endpoint é encontrar registros e o resultado dele for vazio (array vazio), o `status_code` dele deve ser `200`.
        - Por exemplo, se o endpoint de filtragem de notícias buscou pelo termo "Metallica" e nenhum registro foi encontrado, o retorno deve ser `200` com o `body` contendo uma lista vazia.
    - Se o papel de um endpoint é encontrar um registro único e o resultado dele for vazio (item não existente), o `status_code` dele deve ser `404`.
        - Por exemplo, se o endpoint de notícias recebe um `id` e não encontra um registro para aquele valor, o retorno deve ser `404` e sem `body`.
- Utilize verbos HTTP adequados para cada ação (GET para leitura, POST para criação, PUT/PATCH para atualização, DELETE para remoção).
- Utilize JSON como formato de resposta padrão.
- Inclua mensagens de erro claras e consistentes em caso de falhas.
- Implemente autenticação e autorização adequadas para proteger os endpoints sensíveis.

# Convenções de datas

- Todas as datas devem ser tratadas como UTC nesta aplicação.
- Endpoints que recebem em valor de data em algum header, body ou query devem garantir que o valor está em UTC, mesmo que uma conversão seja necessária.
- Endpoints que respondem valores de data devem garantir que o valor está em UTC.

# Convenções de programação com inteligência artificial

- Os prompts utilizados devem ficar na pasta `raiz/services/prompts/` seguindo o padrão de pastas de `structure.md`.
- Prompts devem estar em inglês.
- Você tem liberdade de escrever os prompts.
- Os prompts devem ser escritos de maneira eficiente e clara.
- Atuais prompts do projeto:
    - Prompt para tratamento de notícia: deve melhorar a notícia de forma que ela seja mais clara, corrigindo textos mal elaborados ou erros de digitação.
    - Prompt para decisão de palavras-chave: deve avaliar a notícia para definir quais palavras-chave que mais se encaixam com a informação nela.
    - Prompt para julgamento de notícias (segunda camada): deve avaliar se as palavras-chave de um feed e uma notícia estão relacionadas de alguma forma.
    - Prompt de tradução: deve fazer a tradução de uma notícia para o idioma desejado de forma eficaz e sem alterar o contexto e a informação passada pela notícia.
    - Prompt de resumo: deve fazer o resumo de uma notícia usando o idioma desejado de forma eficaz e sem alterar o contexto e a informação passada pela notícia.
- Para o futuro do projeto, queremos implementar:
    - Guardrails.
    - Uso do Claude ao invés do Gemini.
    - O uso da biblioteca `LangChain` para padronização de chamadas.
    - O uso da biblioteca `LangSmith` para testes de comportamento.
        - Dito isso, testes de comportamento neste momento não serão realizados.
        - Testes que necessitam respostas sempre utilização mocks.

# Convenções de testes

- Testes nunca podem ser executados em ambientes de homologação ou produção.
- Não há testes ligados diretamente ao banco de dados. Ao invés disso, faremos todos esses testes através dos endpoints da API, garantindo que o endpoint e o banco de dados estejam funcionando corretamente ao mesmo tempo.
- Os testes de API devem ficar dentro da pasta `raiz/tests/unit/` seguindo o padrão de pastas de `structure.md`.
- Ao criar um novo endpoint na API, um teste unitário correspondente deve ser criado para ele, garantindo que aquela funcionalidade esteja funcionando corretamente e que o código esteja testável.
- Ao alterar um endpoint existente na API, o teste unitário correspondente deve ser atualizado para refletir as mudanças feitas, garantindo que a funcionalidade continue funcionando corretamente e que o código continue testável.
- Ao remover um endpoint existente na API, o teste unitário correspondente deve ser removido também, garantindo que o código continue limpo e que não haja testes desnecessários para endpoints que não existem mais.
- A alteração de qualquer arquivo em `fixtures`, `integration`, `mocks` ou `unit` deve acionar uma rotina de testes completa para garantir que as mudanças feitas não afetaram negativamente a funcionalidade da aplicação e que o código continua funcionando corretamente.
- Cada teste individual deve conter sua própria instância do banco de dados em memória, ou seja, se no teste A foi criado um usuário, o teste B não o verá. Caso o teste B precise de um usuário, ele deve criar novamente este usuário na sua própria instância.
    - Boas `fixtures` são essenciais para esta regra ser seguida.

## Sobre a pasta fixtures

- Utilize esta pasta para criar métodos reaproveitáveis pelos testes, como criar um usuário de teste, notícias de teste, fonte de notícias de teste, entre outros, onde cada arquivo contém um tipo específico de método (Exemplos: `user_fixtures.go` para métodos relacionados a usuários de teste, `refresh_token_fixtures.go` para métodos relacionados a token de refresh).

## Sobre a pasta integration

- Cada pasta presente em `raiz/services/endpoints/v1/` deve ter uma pasta correspondente dentro de `raiz/tests/integration/api/` contendo os testes relacionados a cada endpoint presente nela (Exemplo: `raiz/tests/integration/api/users/me_test.go` seria o teste de ver dados do usuário).
- Os arquivos da pasta `raiz/tests/mocks/` estão disponíveis para serem utilizados livremente durante os testes de integração.

## Sobre a pasta end-to-end

- Cada pasta presente em `raiz/services/endpoints/v1/` deve ter uma pasta correspondente dentro de `raiz/tests/end-to-end/api/` contendo os testes relacionados a cada endpoint presente nela (Exemplo: `raiz/tests/end-to-end/api/users/me_test.go` seria o teste de ver dados do usuário).
- Os testes devem ser chamados na ordem que melhor couber para fazer o teste completo (Exemplo: primeiro cadastra um usuário, depois realiza login, em seguida cria um feed e assim por diante).
- Um banco de dados temporário deve ser criado exclusivamente para cumprir este teste.
- Os arquivos da pasta `raiz/tests/mocks/` estão disponíveis para serem utilizados, porém apenas os seguintes mocks estão liberados:
    - Autenticação via `OAuth2`: como é impossível fazer o processo de cliques e respostas do Google, vamos simular o login através da criação de um mock de `refresh_token` que ficará gravado no banco de dados e será utilizado para criar o `access_token` e dar prosseguimento aos testes que dependem disso. Enquanto isso, o endpoint de login deve simular que esse processo do Google foi realizado com sucesso e prosseguir naturalmente com a sequência lógica dele.

## Sobre a pasta mocks

- Utilize esta pasta para criar mocks de integrações externas (como Google OAuth2, Gemini e RSS), do próprio repositório (como dados do banco de dados) ou de serviços (como geração de um JWT).

## Sobre a pasta unit

- Cada pasta presente em `raiz/services/endpoints/v1/` deve ter uma pasta correspondente dentro de `raiz/tests/unit/` contendo os testes relacionados a cada endpoint presente nela (Exemplo: `raiz/tests/unit/users/me_test.go` seria o teste de ver dados do usuário).
- Os arquivos da pasta `raiz/tests/mocks/` estão disponíveis para serem utilizados livremente durante os testes unitários.

## Sobre a pasta utils

- Esta pasta é sua para criar utilitários gerais que não tenham a ver especificamente com alguma parte dos testes.

# Logs / debug manual

- O sistema de logs é global e deve ser iniciado uma única vez, assim ele sempre está ativo ou sempre está inativo.
- O sistema de logs se mantém ativado quando a variável de ambiente `VERBOSE_MODE` estiver com o valor `true`.
- Aceitar todos os níveis de mensagens através do `fmt`. Você tem liberdade para decidir qual a melhor forma de se mostrar dados simples (primitivos) e complexos (map, struct, banco de dados).
- O método chamado deve possuir dois parâmetros:
    - um para indicar a mensagem a ser mostrada.
    - um para indicar a cor da mensagem a ser mostrada, com padrão em azul (utilizando a classe Color).
- Nos endpoints:
    - Todas devem começar disparando uma chamada desse handler para indicar que o endpoint foi chamado. Exemplo de mensagem: "@@@ ROUTE START - /v1/users/me - 2026-01-01 12:00:00 @@@".
    - Todas devem terminar disparando uma chamada desse handler para indicar que o endpoint foi concluído. Exemplo de mensagem: "@@@ ROUTE END - /v1/users/me - 2026-01-01 12:00:01 @@@".

# Observabilidade

- Ainda não está implementado e será feito futuramente.

# Lidando com erros e exceções

- Utilize tratamento de exceções em todas as operações críticas que envolvam banco de dados, integrações ou comunicação externa.
- Exceções devem retornar erro 500 como padrão da API.
- Exceções não devem derrubar a aplicação.
- Feeds indisponíveis não devem derrubar a aplicação.
- Implemente retry com backoff exponencial.
- Log detalhado de falhas.

# Aplicativos externos

## Bruno

- Todas as requisições da aplicação devem estar mapeados e prontos para serem executados via Bruno.
- O environment do Bruno deve possuir o mínimo de dados para reprodução de endpoints básicos, mas não pode possui dados sensíveis (como `access_token` ou qualquer tipo `api_key` salvo diretamente) ou informações indevidas (como ofensas ou apologias).
