# Projeto: API de feed de notícias personalizado

## Foco deste projeto

- Backend + API Rest

## Ideia geral

Um feed de notícias hiper personalizado que coleta, filtra, traduz e resume notícias baseado nas preferências do usuário.

## Características

- Gerencia usuários com autenticação Google.
- Permite criação de feeds personalizados através de palavras-chave.
- Descobre automaticamente notícias através do RSS das fontes existentes.
- Filtra notícias por palavras-chave + inteligência artificial.
- Julga a quais feeds dos usuários a notícia descoberta pertence.
- Traduz e classifica notícias através da inteligência artificial.
- Resume notícias de forma personalizada através da inteligência artificial.

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

As regras do fluxo principal estão detalhadas por toda parte neste arquivo.

0. O usuário se cadastra através da sua conta Google.
1. O usuário cria um novo feed e o personaliza conforme preferir.
2. O sistema descobre notícias automaticamente através das fontes RSS disponíveis.
3. Cada nova notícia descoberta recebe um tratamento que envolve melhora do texto e atribuição de palavras-chave para ser gravada no banco de dados.
4. Identifica-se a quais feeds a notícia pertence.
5. O usuário acessa a notícia e ela é marcada como lida.
6. (opcional) O usuário requisita um resumo totalmente personalizado para aquela notícia de acordo com suas preferências.

## Fluxo de cadastro

- Usuários são cadastrados exclusivamente pelo Google.

## Fluxo de descoberta

0. Uma source é cadastrada na aplicação.
1. A CRON é disparada quando for o momento.
2. O processo busca por todas as fontes de notícias ativas.
3. Para cada uma delas faz o parsing RSS com o `gofeed`.
4. Deduplica em comparação à `url_original` das notícias já existentes.
5. Notícias passam pelo tratamento de notícias para terem seu conteúdo tratado.
6. Notícias são salvas no banco de dados.
7. Cada notícia passa pelo processo de julgamento para saber a quais feeds ela pertence.
    - Primeira camada de palavras-chave.
    - Segunda camada de inteligência artificial e `score`.
8. Notícia e feed se relacionam através de um novo registro em `article_feeds`.

## Fluxo de tratamento

0. Uma nova notícia descoberta entra em etapa de tratamento.
1. Uma primeira chamada para a inteligência artificial faz a notícia ser revisada, corrigida e melhorada conforme as convenções da aplicação.
    - É feita através de uma LLM configurada nas variáveis de ambiente `TREATMENT_PROVIDER` e `TREATMENT_MODEL`.
2. Uma segunda chamada para a inteligência artificial faz a notícia receber palavras-chave correspondente ao seu conteúdo.
    - É feita através de uma SLM configurada na variável de ambiente `KEYWORDS_PROVIDER` e `KEYWORDS_MODEL`.
3. A notícia é salva no banco de dados.
4. A notícia segue para a etapa de julgamento.

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
- Se o usuário sofrer soft remove, os seus `refresh_tokens` também devem sofrer soft remove.
- Se o usuário sofrer hard remove, os seus `refresh_tokens` também devem sofrer hard remove.

## Usuário

- A edição de dados do usuário ainda não são possíveis e serão feitas futuramente.
- Alterar os dados do usuário faz com que um novo `access_token` seja gerado e retornado também pela rota, já com as novas informações atualizadas nele.
    - O `access_token` anterior (usado para ativar a atualização dos dados e agora possui informações desatualizadas) vai continuar válido até bater o tempo de expiração. Esse comportamento é considerado normal aqui pois fazem parte de um trecho não crítico da aplicação. Se em algum momento houver dados críticos ligado ao `access_token` e dados do usuário, isso terá que ser mudado.
- Usuários não podem ser excluídos neste momento diretamente via endpoint mas será pensado em um momento futuro.
    - Por enquanto, vamos deixar pronta a exclusão junto com os cascades via código.
    - Testes de cascade envolvendo o usuário podem ser ignorados por enquanto.
    - Consequência atual: um usuário inativo (soft removed) não consegue logar atualmente pois o login busca apenas usuários ativos e a unicidade de `google_id`/`email` impediria um novo cadastro.
        - Isso é aceitável hoje porque não há endpoint de exclusão. Quando esse endpoint for criado, o comportamento (reativar o registro vs. recadastrar vs. tratar como LGPD/erasure) precisa ser decidido. Por ser uma decisão mais complexa do que parece vamos manter assim por enquanto.

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
- Se o usuário sofrer soft remove, as suas preferências também devem sofrer soft remove.
- Se o usuário sofrer hard remove, as suas preferências também devem sofrer hard remove.

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
    - Palavras-chave não podem estar repetidas.
    - Palavras-chave devem ser armazenadas em letras minúsculas.
    - O campo de palavras-chave é uma lista de `string`(no formato `JSON array (TEXT)`).
    - Quanto mais palavras-chave um feed tem, mais amplo vai ser o recebimento de notícias durante o julgamento.
- Um usuário pode ter até 5 feeds ativos por vez.
- Alterar as palavras-chave de um feed não faz com que um novo processo de julgamento das notícias aconteça.
- Os feeds só podem ser visualizados e manipulados pelos seus próprios usuários associados (`user_id`).
- Apenas para documentação: uma reativação de feeds, apesar de viável (logicamente falando), pode nunca acontecer para evitar futuros problemas graves. Vamos considerar essas exclusões como uma exclusão permanenente.
- Feeds excluídos (inativos) não podem receber novas notícias através da tabela de relacionamento (junction table) com notícias.
- Se o usuário sofrer soft remove, todos os seus feeds também devem sofrer soft remove.
- Se o usuário sofrer hard remove, todos os seus feeds também devem sofrer hard remove.

## Notícias

- As notícias são o principal motivo da aplicação existir e podem ser sub-entendidas com a nomenclatura "artigo" também.
- As notícias são descobertas automaticamente através das fontes de notícias.
- Para uma notícia ser descoberta e registrada, o RSS de cada fonte registrada é consultado de tempos em tempos. Ao notar uma nova notícia presente, tratamento de conteúdo também é acionado para fazer suas ações até gravar essa versão em nosso banco de dados.
- Uma notícia do RSS é considerada nova quando a URL original dela não está presente na nossa lista de notícias.
    - Ou seja, outras notícias já existentes não devem ser passadas adiante para o tratamento e julgamento de notícias.
    - Dito isso, a `url_original` da notícia é única dentro das outras notícias ativas no banco de dados.
- Um usuário tem acesso a qualquer notícia registrada na aplicação.
    - As regras para quando a notícia não está em nenhum dos feeds do usuário estão explicadas na sessão "Feed x Notícias".
- As notícias só podem ser criadas, editadas ou excluídas por usuário administradores.
    - Essa regra existe apenas para casos extremos de uma notícia que saiu do controle.
- Quando descobertas, as notícias passam por um julgamento através de uma inteligência artificial para definir palavras-chave às quais ela pertence. As palavras-chave definidas servirão como base para saber em quais feeds ela aparecerá ou não.
    - Esse processo deve ser disparado automaticamente após cada criação de notícias.
- As notícias devem possuir pelo menos 5 palavras-chave com limite de 20.
    - Palavras-chave não podem estar repetidas.
    - O campo de palavras-chave é uma lista de `string` (no formato `JSON array (TEXT)`).
    - Quanto mais palavras-chave uma notícia tem, mais amplo vai ser a distribuição aos feeds durante o julgamento.
- Alterar as palavras-chave de uma notícia não faz com que um novo julgamento aconteça.
- As notícias estão diretamente ligadas à uma fonte de notícias, por isso é obrigatório também passar uma referência (`source_id`) à qual ela pertence.
    - Este campo é obrigatório e imutável.
- Se a fonte de notícias sofrer soft remove, todas as suas notícias também devem sofrer soft remove.
- Se a fonte de notícias sofrer hard remove, todas as suas notícias também devem sofrer hard remove.

## Feed x Notícias

- O feed e as notícias se relacionam de forma múltipla, ou seja, um feed pode possuir diversas notícias e as notícias podem estar presentes em diversos feeds.
- Isso implica em uma tabela relacional (junction table) na qual armazenamos o `id` do feed assim como o `id` da notícia.
    - Essa tabela é preenchida automaticamente quando uma notícia é atrelada a um feed.
    - Essa tabela perde o registro automaticamente quando uma notícia é retirada daquele feed na qual estava associado (provavelmente por alguma opção implementada no futuro).
- Registros nesta tabela são removidos permanentemente ao serem excluídos (hard remove).
- A população dessa tabela acontece no momento na qual uma nova notícia é descoberta e julgada como hábil a estar naquele feed.
    - Considerar apenas feeds que estão ativos.
- O campo `is_read` dessa tabela é marcado quando um usuário a lê.
    - A verificação precisa passar pelo usuário que está ligado ao feed.
- Acessar uma notícia diretamente pelo feed do usuário faz com que uma requisição seja feita para o endpoint de marcação de leitura de notícia.
    - Se a mesma notícia estiver em mais de um feed de um mesmo usuário, todas são marcadas como lida.
    - Note que a URL para se ter acesso à uma notícia é o sempre o mesmo para qualquer usuário com acesso à ela.
        - Assim, qualquer usuário pode ver a notícia através da URL `client_url/articles/{id}`.
        - Ao abrir a URL `client_url/articles/{id}`, uma requisição para `base_url/v1/articles/{id}/read` deve ser disparada. Neste momento abrem-se as seguintes situações:
            - O usuário possui aquela notícia em um ou mais feed: a marcação de leitura daquela notícia é feita em todos os feeds que estão ligados à notícia (retorno do endpoint se torna 200).
            - O usuário não recebeu aquela notícia em seu(s) feed(s): nada acontece (retorno do endpoint se torna 204).
            - O usuário já leu aquela notícia (já está marcado `true` em `is_read`): nada acontece (retorno do endpoint também é 200).
- Registros nessa tabela só são considerados ativos quando o feed e a notícia estão ativos (`status` marcados em `true` e sem `removed_at`).
    - Se o feed ou a notícia sofrer soft remove, o registro dessa tabela não é excluído, ele apenas fica invisível (inacessível).
    - Dito isso, é importante ter essa conferência para garantir que tanto o feed quanto a notícia estão ativas.
- Registros nessa tabela sofrem de hard remove quando o feed ou a notícia atrelado a ele sofrem de hard remove (efeito cascata de ambos os lados).

## Descobrindo uma notícia

- A descoberta de notícias opera de acordo com as seguintes variáveis de ambiente:
    - `RSS_FEED_CRON_ACTIVE`: deve estar em `true` para ser considerado ativo.
    - `RSS_FEED_CRON_SCHEDULE`: o intervalo na qual a CRON irá rodar.
    - `RSS_FEED_CRON_VERBOSE_MODE`: apesar de não ser importante para rodar a CRON, essa variável de ambiente mostra junto as logs a saída de cada chamada da CRON quando estiver marcada como `true`.
- A descoberta de notícias deve acessar cada uma das fontes de notícias cadastradas no banco de dados, olhando pelo RSS de cada uma delas para decidir se há alguma nova notícia.
    - Uma notícia do RSS é considerada nova quando a URL original dela não está presente na nossa lista de notícias.
        - Ou seja, outras notícias já existentes não devem ser passadas adiante para o tratamento e julgamento de notícias.
- Quando descoberta uma nova notícia o tratamento de notícias é iniciado (as regras de tratamento de notícias estão na sessão "tratamento de notícias").
- Ao final da descoberta de notícias a variável `last_article_discovery_at` na tabela `system` é escrita com a hora atual.
- Em termos de código esse método precisa ser independente para poder ser chamado fora da CRON caso necessário.
- Um endpoint de teste para esse processo pode ser encontrado em `GET base_url/v1/sources/{id}/article-discovery`.
    - Deve simular os exatos mesmos processos que rodaria na CRON.
    - Esse endpoint deve ser exclusivo para administradores.
    - Esse endpoint é considerada uma dry-run, ou seja, ela não cria ou altera nenhum registro do banco de dados (incluindo `last_article_discovery_at` de `system`).
    - Esse endpoint deve aceitar os seguintes parâmetros:
        - `id`: uma fonte de notícias válida.
        - `last_article_discovery_at` (query opcional): uma data para fazer o teste sem depender da espera de uma notícia nova naquela fonte de notícias.
    - Esse endpoint responde pelos dados dos artigos descobertos.

## Tratamento de notícias

- O tratamento de notícias ocorre em duas etapas principais:
    - Tratamento principal: traz dinamismo, organização e possíveis correções ao conteúdo da notícia.
    - Nomeação de palavras-chave: elenca palavras-chave para a notícia.
    - Gravação no banco de dados: forma um registro de notícia no banco de dados.
- O tratamento de notícias tem como objetivo:
    - Trazer mais dinamismo ao conteúdo.
    - Organizar conteúdo confuso ou mal escrito.
    - Corrigir erros de ortografia.
    - Nomear palavras-chave para a notícia.
    - Salvar a notícia no banco de dados.
- Os modelos de SLM e LLM disponibilizados durante todas as etapas devem estar na stack em `CLAUDE.md`.
- Em termos de código, o método completo precisa ser independente para poder ser chamado fora da CRON caso necessário.
- Um endpoint de teste para esse processo pode ser encontrado em `POST base_url/v1/articles/treatment`.
    - Deve simular os exatos mesmos processos que rodaria na CRON.
    - Esse endpoint deve ser exclusivo para administradores.
    - Esse endpoint é considerada uma dry-run, ou seja, ela não cria ou altera nenhum registro do banco de dados.
    - O body deste endpoint deve aceitar:
        - `article`: um `json` contendo os dados de uma notícia, obtidos diretamente através da descoberta de notícias.
    - Esse endpoint responde pelos dados do tratamento de notícias.

### Tratamento principal

- A etapa de tratamento principal opera de acordo com as seguintes variáveis de ambiente:
    - `TREATMENT_PROVIDER`: o provedor do modelo para ser usado durante esta etapa.
    - `TREATMENT_MODEL`: o modelo em si para ser usado durante esta etapa.
    - `TREATMENT_VERBOSE_MODE`: `boolean` que decide se os logs são exibidos no terminal ou não durante esta etapa.
- Esta etapa não deve:
    - Traduzir notícias: estritamente proibido fazer tradução neste momento. Melhores informações na sessão "traduzindo notícias".
    - Resumir notícias: estritamente proibido fazer resumo neste momento. Melhores informações na sessão "resumindo notícias".
    - Manter URLs externas: sessões de "leia mais", "veja também" ou afins não podem aparecer no resultado final do conteúdo salvo no banco de dados.
    - Alterar o sentido, sintaxe, ideia ou contexto do conteúdo da notícia.
    - Personalizar a notícia: tentaremos manter a seriedade e tom de humor que a notícia tem originalmente.
    - Trazer opinião própria.

### Nomeação de palavras-chave

- Esta etapa opera em três modos:
    - `local`: utiliza o modelo escolhido localmente para realizar a execução.
    - `small`: utiliza serviços de SLMs para realizar a execução.
    - `large`: utiliza serviços de LLMs para realizar a execução.
- Esta etapa opera de acordo com as seguintes variáveis de ambiente:
    - `KEYWORDS_MODE`: o modo atualmente utilizado durante esta etapa.
    - `KEYWORDS_PROVIDER`: o provedor do modelo para ser usado durante esta etapa.
    - `KEYWORDS_MODEL`: o modelo em si para ser usado durante esta etapa.
    - `KEYWORDS_VERBOSE_MODE`: `boolean` que decide se os logs são exibidos no terminal ou não durante esta etapa.
- As palavras-chave nomeadas devem estar em inglês para facilitar o entendimento de quais notícias estão relacionadas.
- As palavras-chave devem ser armazenadas em letras minúsculas.

### Gravação no banco de dados

- Depois de tratada, a notícia é finalmente salva no banco de dados e pode passar então para o julgamento.

## Julgando se uma notícia pertence ao feed do usuário

- Quando uma notícia é descoberta um processo de julgamento é acionado para saber a qual feed aquela notícia será associada.
- O julgamento funciona através de duas camadas que define se os registros estão relacionados ou não:
    - A primeira camada é a mais simples envolvendo uma comparação de palavras-chave da notícia descoberta com o feed existente.
    - A segunda camada é a garantia através da IA que define um `score` entre 0 e 100, com `threshold` presente na variável de ambiente `JUDGE_SCORE_THRESHOLD`, e define quanto aquela notícia pertence ao feed.
    - Quando forem julgados como associados, um novo registro na tabela associativa (junction table) entre feed e notícias é criado.
- O julgamento de notícias nunca é retroativo.
- O julgamento de notícias só pode considerar feeds que estão ativos.
- Em termos de código esse método precisa ser independente para poder ser chamado fora da CRON caso necessário.

## Traduzindo notícias

- A tradução de notícias é uma opção dada ao usuário em suas preferências.
- Ao entrar na notícia o usuário que tiver a preferência `translate_content` marcada como `true` e a preferência `language` identificada como diferente da original irá automaticamente receber ela traduzida.
    - Para não abusar da funcionalidade o client deve guardar essa tradução em cache para uso futuro.
    - O client também pode oferecer a opção de mostrar o conteúdo original salvo.
    - A personalidade escolhida também deve ser levada em conta na hora de fazer a tradução.
- A tradução deve:
    - Manter a originalidade do conteúdo.
    - Personalizar a tradução da notícia de acordo com a preferência escolhida em `ai_personality`.
- Dito isso, a tradução não pode:
    - Resumir notícias: estritamente proibido fazer resumo neste momento. Melhores informações na sessão "resumindo notícias".
    - Alterar sentido, sintaxe, ideia ou contexto do conteúdo da notícia.
    - Trazer opinião própria.

## Resumindo notícias

- O resumo de notícias é uma opção dada ao usuário pelo client.
- Ao entrar na notícia o usuário tem a opção de resumir a notícia.
    - Para não abusar da funcionalidade o client deve guardar esse resumo em cache para uso futuro.
    - A personalidade escolhida também deve ser levada em conta na hora de fazer o resumo.
- O resumo deve:
    - Manter a originalidade do conteúdo.
    - Personalizar o resumo da notícia de acordo com a preferência escolhida em `ai_personality`.
- Dito isso, o resumo não pode:
    - Alterar sentido, sintaxe, ideia ou contexto do conteúdo da notícia.
    - Trazer opinião própria.
- Notícias que passaram pelo processo de tradução devem ser resumidos no mesmo idioma.

## Administradores

- Ainda não há uma implementação de administradores por enquanto.
- Ações que deveriam ser feitas pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
- Endpoints que deveriam ser acessados pelos usuários administradores por enquanto podem ser feitas por qualquer usuário "comum".
    - Atualmente são eles: `DELETE base_url/v1/auth/invalidate`, `DELETE base_url/v1/auth/invalidate_all`, `POST base_url/v1/sources/create`, `PUT base_url/v1/sources/{id}`, `DELETE base_url/v1/sources/{id}`

### Painel de controle

- Usuários administradores tem acesso à alguns endpoints exclusivos da tabela `system`.
- A tabela `system` é um grande painel de controle que é capaz de ativar ou desativar funcionalidades de forma temporária para resolver erros momentâneos.
- As atuais funcionalidades são:
    - `app_status`: estado da aplicação. Todos os endpoints devem garantir que o estado atual da aplicação é `true`. Quando em `false` o erro deve ser 503.
    - `last_article_discovery_at`: data do último descobrimento de notícias.
- Novas funcionalidades virão futuramente.

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
