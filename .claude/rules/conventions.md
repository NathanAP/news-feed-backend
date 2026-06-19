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

- As versões devem seguir o padrão SemVer (MAJOR.MINOR.PATCH).

# Convenções de workaround

- Workarounds são necessários mas a preferência é no ajuste do código para evitar essas necessidades. Sabemos que um refatoramento é demorado e perigoso, mas quando necessário, tem que ser feito o quanto antes para evitar problemas maiores no futuro.
- Ao notar que um workaround é necessário, sempre avise e explique o motivo e a solução proposta. Caso seja muito grande ou seja considerada uma gambiarra de código, considere parar o processo para falar sobre isso.

# Convenções de arquivos e pastas

- Nomes de arquivos e pastas devem estar em snake case (Exemplo: user.go, user_preference.go).
- Nomes de arquivos de testes deve utilizar sufixo "\_test" (Exemplo: user_test.go, user_prefence.go).

# Convenções de banco de dados

- Nomes de tabelas do banco de dados devem estar no plural e em snake case (Exemplo: users, user_preferences).
- Todas as tabelas devem ter os campos `created_at`, `modified_at` e `removed_at` do tipo timestamp.
- O campo `status` deve ser utilizado para indicar o estado de um registro no banco de dados.
- Ao alterar um registro, o campo `modified_at` deve ser atualizado com o timestamp atual.
- Ao remover um registro, o campo `removed_at` deve ser atualizado com o timestamp atual e o campo `status` deve ser definido como false (0).
- Exclusões devem ser feitas através de soft delete, utilizando um campo booleano chamado `status` e um campo de timestamp `removed_at`.

# Convenções da API

- Todos os dados e preferências públicas do usuário estão no `access_token`. Se um endpoint precisar de um dado ou uma preferência do usuário não mapeada no `access_token`, avise-nos para que possamos tomar as medidas necessárias, como invalidar tokens antigos ou renovar os tokens automaticamente.
- Seguir os padrões de respostas tradicionais para RESTful com os códigos de status HTTP apropriados.
- Middlewares devem estar na pasta `raiz/middlewares/` e seguir a convenção de nomeação de arquivos (Exemplo: `auth.go` para middleware de autenticação, `logging.go` para middleware de logging).
- Endpoints devem estar em seu próprio arquivo, organizado dentro de uma pasta do modelo correspondente (Exemplo: `raiz/services/endpoints/v1/users/` para endpoints relacionados a usuários; arquivo `login.go` corresponde à rota de login do usuário).
- Endpoints devem seguir o padrão `base_url/versao_da_api/modelo/acao` (Exemplo: `http://localhost:3000/v1/users/create`, `http://localhost:3000/v1/users/login`).
- Utilize verbos HTTP adequados para cada ação (GET para leitura, POST para criação, PUT/PATCH para atualização, DELETE para remoção).
- Utilize JSON como formato de resposta padrão.
- Inclua mensagens de erro claras e consistentes em caso de falhas.
- Implemente autenticação e autorização adequadas para proteger os endpoints sensíveis.

# Lidando com erros e exceções

- Utilize tratamento de exceções em todas as operações críticas que envolvam banco de dados, integrações ou comunicação externa.
- Exceções devem retornar erro 500 como padrão da API.
- Exceções não devem derrubar o sistema.
- Feeds indisponíveis não devem derrubar o sistema.
- Implemente retry com backoff exponencial.
- Log detalhado de falhas.
