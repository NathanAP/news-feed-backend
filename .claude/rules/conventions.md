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

# Versionamento

- As versões devem seguir o padrão major.minor.patch.docs.
- Alterações de docs geralmente não são geradas por você, elas sofrem mudança cada vez que um documento é alterado.
- Ao concluir uma alteração major, minor ou patch o agente `test_manager` deve ser acionado para que seja efetuada uma nova rotina de testes. A falha dessa rotina deve impedir a continuidade do processo de desenvolvimento.

# Convenções de workaround

- Workarounds são necessários mas a preferência é no ajuste do código para evitar essas necessidades. Sabemos que um refatoramento é demorado e perigoso, mas quando necessário, tem que ser feito o quanto antes para evitar problemas maiores no futuro.
- Ao notar que um workaround é necessário, sempre avise e explique o motivo e a solução proposta. Caso seja muito grande ou seja considerada uma gambiarra de código, considere parar o processo para falar sobre isso.

# Convenções de arquivos e pastas

- Nomes de arquivos e pastas devem estar em snake case (Exemplo: user.go, user_preference.go).
- Nomes de arquivos de testes deve utilizar sufixo "\_test" (Exemplo: user_test.go, user_prefence.go).

# Convenções de banco de dados

- Nomes de tabelas do banco de dados devem estar no plural e em snake case (Exemplo: users, user_preferences).
- As tabelas devem ter pelo menos os campos `id`, `status`, `created_at`, `modified_at` e `removed_at` do tipo timestamp.
    - A exceção imediata dessa regra são as tabelas de relacionamento (junction tables) que devem ter pelo menos o campo `id`.
- As tabelas de relacionamento (junction tables) devem conter registros de `id` existentes nas tabelas relacionadas, por exemplo, se a tabela contém um `user_id`, todos os registros devem ter um `user_id` válido.
- `id` deve ser do tipo UUID v7.
- O campo `status` deve ser utilizado para indicar o estado de um registro no banco de dados.
    - Geralmente esse campo vai ser um `true` ou `false`, mas em alguns casos pode ser um `enum` para indicar mais estados, como "active", "inactive", "pending", etc.
- Ao alterar um registro, o campo `modified_at` deve ser atualizado com o timestamp atual.
- Tabelas de relacionamento (junction tables) não sofrem alterações. Para "trocar" qualquer um dos `id` do relacionamento, o registro antigo deve ser removido e um novo registro deve ser criado.
- Exclusões de registros em tabelas comuns devem ser feitas através de soft delete, utilizando um campo booleano chamado `status` e um campo de timestamp `removed_at`.
    - Nesse caso, o campo `removed_at` deve ser atualizado com o timestamp atual e o campo `status` deve ser definido como false (0).
    - A exceção imediata dessa regra são as tabelas de relacionamento (junction tables) que devem ser feitas através de hard delete, ou seja, o registro deve ser removido da tabela permanentemente.

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
- Endpoints de pesquisa por múltiplos parâmetros deve sempre ser um GET e seguir o padrão `base_url/versao_da_api/modelo?parametro1=valor1&parametro2=valor2` (Exemplo: `http://localhost:3000/v1/sources?name=example&status=true`).
- Utilize verbos HTTP adequados para cada ação (GET para leitura, POST para criação, PUT/PATCH para atualização, DELETE para remoção).
- Utilize JSON como formato de resposta padrão.
- Inclua mensagens de erro claras e consistentes em caso de falhas.
- Implemente autenticação e autorização adequadas para proteger os endpoints sensíveis.

# Convenções de testes

- Não há testes ligados diretamente ao banco de dados. Ao invés disso, faremos todos esses testes através das rotas da API, garantindo que a rota e o banco de dados estejam funcionando corretamente ao mesmo tempo.
- Os testes de API devem ficar dentro da pasta `raiz/tests/unit` seguindo o padrão de pastas de `structure.md`.
- Ao criar uma nova rota na API, um teste unitário correspondente deve ser criado para essa rota, garantindo que aquela funcionalidade esteja funcionando corretamente e que o código esteja testável.
- Ao alterar uma rota existente na API, o teste unitário correspondente deve ser atualizado para refletir as mudanças feitas, garantindo que a funcionalidade continue funcionando corretamente e que o código continue testável.
- Ao remover uma rota existente na API, o teste unitário correspondente deve ser removido também, garantindo que o código continue limpo e que não haja testes desnecessários para rotas que não existem mais.
- A alteração de qualquer arquivo em `fixtures`, `integration`, `mocks` ou `unit` deve acionar uma rotina de testes completa para garantir que as mudanças feitas não afetaram negativamente a funcionalidade do sistema e que o código continua funcionando corretamente.

## Sobre a pasta fixtures

- Utilize esta pasta para criar métodos reaproveitáveis pelos testes, como criar um usuário de teste, criar uma categoria de notícias de teste, criar uma fonte de notícias de teste, entre outros, onde cada arquivo contém um tipo específico de método (Exemplos: `user_fixtures.go` para métodos relacionados a usuários de teste, `refresh_token_fixtures.go` para métodos relacionados a token de refresh).

## Sobre a pasta integration

- Faremos os testes de integração posteriormente.

## Sobre a pasta mocks

- Utilize esta pasta para criar mocks de integrações externas (como Google OAuth2, Gemini e RSS), do próprio repositório (como dados do banco de dados) ou de serviços (como geração de um JWT).

## Sobre a pasta unit

- Os `unit` são testes de simulações de chamadas simuladas à API, ou seja, não são testes de integração ou end-to-end completos. Ela vai servir para nos garantir que os dados de chegada e de saída do dado estão corretos.
- Cada pasta presente em `raiz/services/endpoints/v1/` deve ter uma pasta correspondente dentro de `raiz/tests/unit/` contendo os testes relacionados a cada endpoint presente nela (Exemplo: `raiz/tests/unit/users/me_test.go` seria o teste de ver dados do usuário).
- Os arquivos da pasta `raiz/tests/mocks` estão disponíveis para serem utilizados livremente durante os testes unitários.

# Lidando com erros e exceções

- Utilize tratamento de exceções em todas as operações críticas que envolvam banco de dados, integrações ou comunicação externa.
- Exceções devem retornar erro 500 como padrão da API.
- Exceções não devem derrubar o sistema.
- Feeds indisponíveis não devem derrubar o sistema.
- Implemente retry com backoff exponencial.
- Log detalhado de falhas.
