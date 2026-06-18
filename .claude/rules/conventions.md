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

# Convenções de arquivos e pastas

- Nomes de arquivos e pastas devem estar em snake case (Exemplo: user.go, user_preference.go).
- Nomes de arquivos de testes deve utilizar sufixo "\_test" (Exemplo: user_test.go, user_prefence.go).

# Convenções de banco de dados

- Nomes de tabelas do banco de dados devem estar no plural e em snake case (Exemplo: users, user_preferences).
- Exclusões devem ser feitas através de soft delete, utilizando um campo booleano chamado `status` e um campo de timestamp `removed_at`.

# Convenções da API

- Armazene todos os dados do usuário no JWT, evitando a necessidade de consultas adicionais ao banco de dados para autenticação e autorização.
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
