# Projeto: API de feed de notícias personalizado

## Foco deste projeto

- Backend + API Rest

## Ideia geral

Um feed de notícias hiper personalizado que coleta e filtra notícias baseado nas preferências do usuário.

## Características

- Gerencia usuários com autenticação Google.
- Permite criação de feeds personalizados através de palavras-chave.
- Descobre automaticamente notícias através do RSS das fontes existentes.
- Filtra notícias por palavras-chave + inteligência artificial.
- Julga a quais feeds dos usuários a notícia descoberta pertence.

## Features

- Descoberta automática de RSS: detecta feeds sem input manual, apenas através da URL principal
- Filtragem dual-layer: palavras-chave rápidas + IA inteligente
- Personalização de notícias: personaliza notícias de acordo com as preferências do usuário
- Endpoints REST: API simples e intuitiva
- Banco PostgreSQL integrado: zero dependencies de infraestrutura

## Público alvo

Usuários que querem um feed de notícias confiável e personalizável do seu jeito.

# Como funciona

## Fluxo principal

As regras do fluxo principal estão detalhadas por toda parte neste arquivo.

0. O usuário se cadastra através da sua conta Google.
1. O usuário cria um novo feed e o personaliza conforme preferir.
2. O sistema descobre notícias automaticamente através das fontes RSS disponíveis.
3. Cada nova notícia descoberta recebe um tratamento antes de ser gravado no banco de dados. Ela envolve:
    - normalização de URLs.
    - normalização de elementos HTML.
    - atribuição de palavras-chave.
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
    - Primeira etapa de palavras-chave.
    - Segunda etapa de inteligência artificial e `score`.
8. Notícia e feed se relacionam através de um novo registro em `article_feeds`.

## Fluxo de tratamento

0. Uma nova notícia descoberta entra em etapa de tratamento de URLs.
1. Todas as URLs do conteúdo (tags `<a>`) da notícia são identificados.
2. Para cada URL identificada verifica-se cada uma delas olhando pela exata `url_original` nos registros de notícias.
   2.1. Caso a notícia exista, altera-se aquela URL do conteúdo da notícia para apontar para a do client.
   2.2. Caso contrário nada acontece.
3. Uma chamada para a inteligência artificial faz a notícia receber palavras-chave correspondente ao seu conteúdo.
4. Uma série de pequenas operações são realizadas no nível de banco de dados (gravação da notícia, alterações em URLs externas já existentes, entre outros).
5. A notícia segue para a etapa de julgamento.

## Autenticação

- O secret dos tokens está na variável de ambiente chamada `JWT_SECRET_KEY`.
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
    - `language_to_translate`: `enum` (opcional) com o idioma-alvo de tradução das preferências; `null` quando o usuário não quer tradução (apenas dica de client, não afeta a API).
    - `ai_personality`: `enum` contendo a personalidade da IA e presente nas preferências do usuário relacionado.
    - `admin`: `boolean` indicando se o usuário é administrador. **Apenas dica de client**: a API sempre reconfere a flag no banco antes de autorizar uma ação de administrador (ver "Administradores").
- Um struct chamado `Claims` mantém também esse mapeamento no código.
- Se o usuário sofrer soft remove, os seus `refresh_tokens` também devem sofrer soft remove.
- Se o usuário sofrer hard remove, os seus `refresh_tokens` também devem sofrer hard remove.

### Fluxo de login

1. Client redireciona o usuário para `/v1/auth/google?redirect_uri={uri}`.
2. Recebemos a requisição.
3. Validamos o match exato entre a URI enviado contra os itens de uma lista de URIs permitidas (allowlist).
4. Um valor aleatório secreto (`CSRF`) é criado para o `redirect_uri` do client para ser passado como `state` adiante.
5. Redirecionamos o usuário para o fluxo de autenticação do Google, passando o nosso próprio `redirect_uri` fixo (`GET base_url/v1/auth/google/callback`) e o `state` (gerado no passo anterior).
6. Usuário se autentica pelo Google.
7. Recebemos a confirmação de sucesso contendo um `code` e o mesmo `state` através do `redirect_uri`.
8. Confirmamos que o `state` é realmente válido.
9. Trocamos o `code` pelos dados reais do usuário no Google (`email`, `google_id`, ...).
10. Com os dados recebidos, identificamos se o usuário pode ser encontrado diretamente ou se precisa criar ele.
11. Prossegue com a geração dos `tokens` e dados necessários.
12. Nosso callback redireciona de volta à URI enviada lá no passo 1 junto com os `tokens` gerados através de `fragment`.
13. Client captura os dados e limpa o `fragment`.
14. Client passa a usar o `access_token` como header (`Authorization: Bearer {token}`) nas próximas requisições.

### Erros no fluxo de login

- Quando `redirect_uri` for inválido ou não estiver na allowlist: erro 400.
- Troca de `code` falha ou `state` inválido: erro 401.
- Desistência ou cancelamento no fluxo de login da Google: redirect de volta ao cliente com erro na query (`?error=access_denied`).

## Usuário

- A edição de dados do usuário ainda não são possíveis e serão feitas futuramente.
- Alterar os dados do usuário faz com que um novo `access_token` seja gerado e retornado também pela rota, já com as novas informações atualizadas nele.
    - O `access_token` anterior (usado para ativar a atualização dos dados e agora possui informações desatualizadas) vai continuar válido até bater o tempo de expiração. Esse comportamento é considerado normal aqui pois fazem parte de um trecho não crítico da aplicação. Se em algum momento houver dados críticos ligado ao `access_token` e dados do usuário, isso terá que ser mudado.
- Usuários não podem ser excluídos neste momento diretamente via endpoint mas será pensado em um momento futuro.
    - Por enquanto, vamos deixar pronta a exclusão junto com os cascades via código.
    - Testes de cascade envolvendo o usuário podem ser ignorados por enquanto.
    - Consequência atual: um usuário inativo (soft removed) não consegue logar atualmente pois o login busca apenas usuários ativos e a unicidade de `google_id`/`email` impediria um novo cadastro.
        - Isso é aceitável hoje porque não há endpoint de exclusão. Quando esse endpoint for criado, o comportamento (reativar o registro vs. recadastrar vs. tratar como LGPD/erasure) precisa ser decidido. Por ser uma decisão mais complexa do que parece vamos manter assim por enquanto.
- O campo `last_login_at` faz o controle de quando o usuário logou pela última vez.
    - Diferente do campo `last_active_at`, este controla quando o usuário fez o último processo de login completo pelo Google (e consequentemente cria uma nova sessão / `refresh_token`).
- O campo `last_active_at` faz o controle de quando o usuário esteve ativo pela última vez na aplicação.
    - Ao criar um novo usuário, este campo já vem preenchido com `now` por padrão para evitar que o usuário de desenvolvimento tenha um valor de data fixado.
    - Este campo é atualizado cada vez que o usuário passar pelo endpoint que renova a validade do seu token por uma hora ou quando um login for realizado (atualizando tanto este campo quanto `last_login_at`).
    - Um usuário se torna automaticamente inativo para a descoberta de notícias após uma quantidade de dias de inatividade, definida pela variável de ambiente chamada `DAYS_UNTIL_USER_IS_INACTIVE`.
        - Este campo também pode receber o valor em `-1` para indicar que o tempo de dias é infinito, assim um usuário de desenvolvimento não se torna inativo nunca.
    - Isso implica que usuários inativos vão acabar perdendo as notícias que foram descobertas durante o tempo de inatividade.

## Preferências do usuário

- Ao criar um usuário, suas preferências devem ser criadas automaticamente também.
    - Valores padrão:
        - idioma para tradução português
        - personalidade em `misto`
- O usuário tem liberdade de alterar suas preferências para utilização da aplicação da forma que preferir.
- Apenas os próprios usuários podem alterar suas preferências.
- Usuários removidos (`status` em `false`) devem ficar com suas preferências excluídas também (`status` também deve ser setado para `false`)
- O campo `language_to_translate` pode ser nulo, isso quer dizer que a pessoa nunca vai receber a opção de tradução no client.
- O campo `language_to_translate` é o idioma-alvo usado pela rota de tradução: a API lê ele do `access_token` para decidir para qual idioma traduzir. O client apenas altera esse valor para o usuário; quem decide o comportamento da tradução é a API.
    - Quando nulo, não há alvo configurado e a tradução não pode ser feita (a rota responde 400).
- Alterar as preferências do usuário faz com que um novo `access_token` seja gerado, retornando junto ao client, já com as novas informações atualizadas nele.
    - O `access_token` anterior (usado para ativar a atualização das preferências e agora possui dados desatualizados) vai continuar válido até bater o tempo de expiração. Esse comportamento é considerado normal aqui pois fazem parte de um trecho não crítico da aplicação. Se em algum momento houver dados críticos ligado ao `access_token` e preferências do usuário, isso terá que ser mudado.
- Se o usuário sofrer soft remove, as suas preferências também devem sofrer soft remove.
- Se o usuário sofrer hard remove, as suas preferências também devem sofrer hard remove.

## Fontes de notícias (sources)

- As fontes de notícias são nossa principal fonte para obtenção de informações brutas.
- A visibilidade das fontes de notícias são públicas a todos os usuários da aplicação.
- A manipulação (criação, edição ou remoção) de fonte de notícias é exclusiva para administradores da aplicação, ou seja, para os usuários "comuns" as fontes de notícias parecem como pré-definidas.
- Duas fontes de notícias não podem ter a mesma `url` ou o mesmo `url_rss`.
- O payload de cadastro de uma fonte de notícias obriga o valor de `url_rss`. Para facilitar o encontro dessa URL, temos a rota `base_url/v1/sources/rss-discovery` que tenta descobrir automaticamente e fazer o parsing através do `gofeed` desse valor através dos seguintes padrões:
    - padrões comuns como acessar `/rss/`, `/feed/`, `/rss.xml/`, `/feed.xml`.
    - padroes de parsing HTML para encontrar `<link rel="alternate" type="application/rss+xml">`.
- Algumas fontes de notícias não possuem o conteúdo completo das notícias em seu RSS. Neste momento não tem nada a ser feito em relação a esses casos pois não há hoje uma forma de acessar a URL original e resgatar apenas o conteúdo original da notícia.

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
- O endpoint `GET base_url/v1/feeds/check-for-new-articles` é utilizado para detectar quais feeds possuem notícias não lidas.
    - Provavelmente no futuro esse endpoint se torne um websocket ou um SSE, mas por enquanto ela será consultada manualmente pelo client.
- Feeds excluídos (inativos) não podem receber novas notícias através da tabela de relacionamento (junction table) com notícias.
- Se o usuário sofrer soft remove, todos os seus feeds também devem sofrer soft remove.
- Se o usuário sofrer hard remove, todos os seus feeds também devem sofrer hard remove.

### Sugestão de palavras-chave

- Ao montar um feed, o usuário recebe sugestões de palavras-chave para adicionar, através do endpoint `GET base_url/v1/feeds/keyword-suggestions`.
    - As sugestões saem do acervo global de notícias, então não há escopo por usuário, mas o endpoint exige autenticação.
    - O objetivo de produto é empurrar o usuário para palavras-chave genéricas (gênero/categoria como `rock`, `concerts`, `albums`), pois o específico ("metallica") a pessoa lembra sozinha então o genérico é o que ela esquece e é justamente o que faz a notícia bater no feed na etapa de julgamento. Por isso o ranking é por contagem de ocorrência crua, sem penalizar termos genéricos.
- O endpoint aceita os seguintes parâmetros de query (todos opcionais):
    - `keywords`: as palavras-chave já escolhidas, separadas por vírgula. São normalizadas no servidor (trim, minúsculas, deduplicadas, vazias descartadas); passar mais que o teto de um feed (`FeedKeywordsMax`) resulta em 400.
    - `limit`: quantas sugestões trazer (padrão 10, mínimo 1, máximo 50).
- O endpoint opera em duas estratégias, decididas no servidor, e sempre devolve alguma sugestão, ou seja, nunca fica vazio quando há notícias:
    - `related`: quando há palavras-chave escolhidas, sugere as que mais co-ocorrem com elas (aparecem nas mesmas notícias). É topical, não temporal, ou seja, não sofre janela de tempo.
    - `popular`: quando nada foi escolhido ou quando a estratégia `related` não encontrou nada, sugere as palavras-chave mais frequentes nas notícias recentes.
        - "Recente" é controlado pela variável de ambiente `KEYWORD_SUGGESTIONS_WINDOW_DAYS` (padrão 30 dias; `-1` desliga a janela e considera todo o histórico). A janela existe tanto por produto ("popular agora") quanto por custo — é uma agregação da tabela inteira, então limitá-la no tempo evita varrer todo o acervo.
    - A palavra-chave já escolhida nunca é sugerida de volta (vale para as duas estratégias).
    - Este endpoint não é paginado (ver isenção em `conventions.md` e na seção "Paginação").
- A resposta ecoa qual estratégia gerou a lista, para o client rotular ("Relacionadas às suas escolhas" vs "Populares agora") e para o fallback ser observável sem ler log:
    ```json
    {
        "strategy": "related",
        "suggestions": [
            { "keyword": "rock", "count": 812 },
            { "keyword": "concerts", "count": 133 }
        ]
    }
    ```
- Apenas notícias ativas alimentam as contagens (uma notícia soft-removida é invisível aqui, como em qualquer busca).
- Ponto em aberto conhecido: a faixa máxima de palavras-chave do feed (`FeedKeywordsMax`, hoje 20) e os knobs de triagem do julgamento (`JUDGEMENT_AUTOASSOCIATE_RATIO` etc.) são as alavancas reais da qualidade do match, e devem ser recalibrados com dado real quando houver volume de feeds. A sugestão de keywords foi entregue com ranking simples (contagem crua) de propósito, para observar o comportamento real antes de investir em ranking mais elaborado (ex.: peso por especificidade / lift).

## Notícias

- As notícias são o principal motivo da aplicação existir e podem ser sub-entendidas com a nomenclatura "artigo" também.
- As notícias são descobertas automaticamente através das fontes de notícias.
- Para uma notícia ser descoberta e registrada, o RSS de cada fonte registrada é consultado de tempos em tempos. Ao notar uma nova notícia presente, o tratamento também é acionado para fazer suas ações até gravar essa versão em nosso banco de dados.
- Uma notícia do RSS é considerada nova quando a URL original dela não está presente na nossa lista de notícias.
    - Ou seja, outras notícias já existentes não devem ser passadas adiante para o tratamento e julgamento de notícias.
    - Dito isso, a `url_original` da notícia é única dentro das outras notícias ativas no banco de dados.
- Um usuário tem acesso a qualquer notícia registrada na aplicação.
    - As regras para quando a notícia não está em nenhum dos feeds do usuário estão explicadas na sessão "Feed x Notícias".
- O endpoint `GET base_url/v1/feeds/{id}/articles` é responsável por trazer as notícias de um feed específico.
    - Esse endpoint é a base da aplicação, é através dela que o usuário vê as notícias do(s) seu(s) feed(s).
    - Esse endpoint deve ser exclusivo do usuário.
    - Esse endpoint deve ser paginado.
    - Esse endpoint deve trazer também os dados da tabela relacional (junction table) entre cada notícia e o feed.
- A manipulção de notícias (criação, edição ou remoção) é exclusiva para administradores da aplicação.
    - Essa regra existe apenas para casos extremos de uma notícia que saiu do controle.
    - A criação não passa pelo julgamento automaticamente. A ideia desse endpoint na verdade é abrir a futura possibilidade de notícias patrocinadas. Por enquanto ela existe apenas por padronização geral mesmo.
- As notícias possuem um campo `language_original` que serve para detectar quando uma tradução pode ou não ser feita no client.
    - O valor deste campo deve ser o mesmo `enum` de idiomas usado globalmente na aplicação.
    - Utiliza-se a biblioteca `lingua-go` para fazer a detecção durante o tratamento de notícias.
        - Isso faz com que o campo precise ser tratado como opcional no banco de dados para quando há falha na detecção do idioma.
        - Entretanto, os endpoints de criação e alteração de notícias devem obrigar o valor a ser passado.
- Quando descobertas, as notícias passam por um julgamento através de uma inteligência artificial para definir palavras-chave às quais ela pertence. As palavras-chave definidas servirão como base para saber em quais feeds ela aparecerá ou não.
    - Esse processo deve ser disparado automaticamente após cada criação de notícias.
- As notícias devem possuir pelo menos 5 palavras-chave com limite de 30.
    - Palavras-chave não podem estar repetidas.
    - O campo de palavras-chave é uma lista de `string` (no formato `JSON array (TEXT)`).
    - Quanto mais palavras-chave uma notícia tem, mais amplo vai ser a distribuição aos feeds durante o julgamento.
    - As palavras-chave devem misturar termos específicos (pessoas, marcas, lugares, eventos) e genéricos (gênero/categoria/domínio, ex.: `rock`, `metal`, `music`, `sports`).
        - Os genéricos são o que permite a um feed genérico casar uma notícia de entidades específicas na etapa de julgamento mais facilmente.
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

## URLs externas de notícias

- A tabela de URLs externas de notícias (`article_outbound_links`) serve para corrigir o link entre notícias de forma retroativa.
- Ao detectar uma URL em uma notícia em uma tag `<a href="{url}">`, um novo registro deve ser criado nessa tabela e a tag passa a receber o `id` criado, resultando em `<a href="{article_outbound_link_id}">`.
- A existência dessa tabela se motiva ao fato de que a URL presente no conteúdo de uma notícia já existente nunca será alterada. Fazer uma varredura cada vez que uma notícia chega é muito pesada, cara e difícil de ser processada.
    - Por exemplo, se a notícia `A` possui uma URL externa `url_b` e coincidentemente a notícia `B` chega mais tarde como representante da URL externa `url_b`, o conteúdo da notícia `A` nunca é alterado, mesmo que aquela URL esteja presente aqui.
    - O ideal é que a notícia `A` tenha inicialmente a URL externa (`url_b`) arquivada e quando chegar a notícia `B` ela sofra alteração para a nova URL.
- A solução para isso é a criação dessa tabela, que possui os seguintes campos além dos convencionais (`id`, `created_at`, etc):
    - `article_id`: a notícia à qual essa tabela esta se referindo.
    - `href`: a URL da notícia, seja ela interna ou externa, com índice.
- Essa tabela não possui os campos `status` e `removed_at` e seus registros não podem ser excluídos diretamente por endpoints. Dito isso:
    - Se uma notícia sofrer soft remove, nada acontece ao(s) registro(s) de `article_id` desta tabela.
    - Se uma notícia sofrer hard remove, o(s) registro(s) que possuam seu `article_id` nesta tabela também são excluídos (efeito cascata).
- É necessária uma funcionalidade centralizada para todas as vezes que uma notícia servir de resposta em um endpoint, trocando de `<a href="{article_outbound_link.id}">` para `<a href="{article_outbound_link.href}">`.

## Descobrindo uma notícia

- A descoberta de notícias opera de acordo com as seguintes variáveis de ambiente:
    - `RSS_FEED_CRON_ACTIVE`: deve estar em `true` para ser considerado ativo.
    - `RSS_FEED_CRON_SCHEDULE`: o intervalo na qual a CRON irá rodar.
    - `RSS_FEED_CRON_VERBOSE_MODE`: apesar de não ser importante para rodar a CRON, essa variável de ambiente mostra junto as logs a saída de cada chamada da CRON quando estiver marcada como `true`.
    - `DISCOVERY_CONCURRENCY`: quantas fontes (varredura de RSS) e quantas notícias (pipeline de tratamento + julgamento) são processadas em paralelo. Valor padrão e mínimo é `1` (sequencial). O paralelismo é feito por um worker pool interno de goroutines, limitado por este valor.
        - Em `development` mantém-se `1` (sequencial e determinístico); em `staging`/`production` sobe-se o valor, limitado pelo rate limit do provedor de IA.
        - O gargalo do pipeline é a latência de rede das chamadas de IA (2-3 por notícia), por isso a concorrência é o que aumenta o throughput. Ver a seção "Escalabilidade futura" no `ROADMAP.md` para a evolução planejada (fila distribuída + workers, dependente do Postgres).
    - `DISCOVERY_MAX_ARTICLES`: cap de quantas notícias descobertas uma varredura entrega ao tratamento/julgamento. Valor `-1` (padrão) = sem cap (valor de ambiente de produção). É um freio para manter o uso de IA sob o rate limit do provedor durante testes; corta o lote combinado da varredura antes do dedup.
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
    - Esse endpoint responde dados dos artigos descobertos.
- O scheduler protege cada tick da CRON: `SkipIfStillRunning` (um sweep que passa do intervalo não gera um segundo concorrente — economia de IA, já que o índice único de `url_original` sozinho impede notícia duplicada) e `Recover` (panic numa varredura vira log, não derruba a app in-process).

## Tratamento de notícias

- O tratamento de notícias ocorre em alguns passos principais:
    - Detecção de idioma: detecta automaticamente qual é o idioma original da notícia.
    - Tratamento de URLs: busca URLs na notícia para tentar descobrir se ela está conectando à outra(s) notícia(s) existente(s) no nosso banco de dados.
    - Sanatização do conteúdo: utiliza a biblioteca `bluemonday` para filtrar e sanatizar trechos indesejados da notícia.
    - Nomeação de palavras-chave: elenca palavras-chave para a notícia.
    - Operações no banco de dados: realiza várias pequenas tarefas relacionadas aos registros no banco de dados.
- A IA não toca no corpo da notícia: o tratamento de URLs e a sanitização são determinísticos (sem IA). A única etapa que usa IA é a nomeação de palavras-chave.
- Os modelos de SLM e LLM utilizados na etapa de palavras-chave devem estar na stack em `CLAUDE.md`.
- Em termos de código, o método completo precisa ser independente para poder ser chamado fora da CRON caso necessário.
- Um endpoint de teste para esse processo pode ser encontrado em `POST base_url/v1/articles/treatment`.
    - Deve simular os exatos mesmos processos que rodaria na CRON.
    - Deve permitir a troca de modo na etapa de nomeação de palavras-chave através de uma chave chamada `keywords_mode` no body, aceitando os valores disponibilizados na descrição da etapa.
    - Esse endpoint é exclusivo do modo de desenvolvimento (através da variável de ambiente `ENVIRONMENT`): o endpoint não deve ser registrado fora de dev (mesmo padrão do `dev-login`), pois é uma ferramenta interna que chama a IA de verdade e jamais deve ser alcançada por clientes.
    - Esse endpoint é considerada uma dry-run, ou seja, ela não cria ou altera nenhum registro do banco de dados.
    - O body deste endpoint deve aceitar:
        - `article`: um `json` contendo os dados de uma notícia, obtidos diretamente através da descoberta de notícias.
    - Esse endpoint responde pelos dados do tratamento de notícias.

### Detecção de idioma

- A etapa de detecção de idioma utiliza a biblioteca `lingua-go` para guardar o idioma original da notícia a ser gravado no campo `language_original` no banco de dados.
- Em caso de falha, o campo `language_original` da notícia deve ficar `null`.

### Tratamento de URLs

- A etapa de tratamento de URLs opera de acordo com as seguintes variáveis de ambiente:
    - `URLS_TREATMENT_VERBOSE_MODE`: `boolean` que decide se os logs são exibidos no terminal ou não durante esta etapa.
    - `CLIENT_URL`: URL base do client, usada para construir o link interno (`{CLIENT_URL}/articles/{id}`). Quando não configurada, esta etapa é pulada (os links seguem inalterados) e o corpo apenas segue para a sanitização.
- Esta etapa tem como objetivo identificar cada URL presente na notícia em tratamento para descobrir quais estão presentes na nossa lista de notícias no campo `url_original` e apontar para nossa própria URL ao invés da externa.
    - Apenas considerar URLs presentes nas tags `<a>`.
    - Roda antes da sanitização pois o link interno reescrito precisa existir antes do `bluemonday` e como ele já permite links, nenhuma mudança de política é necessária.
    - É determinística e best-effort: uma falha de leitura no banco não derruba o artigo, assim o corpo original segue para a sanitização.
- Exemplo de quando aplicar o tratamento de URL:
    - A notícia `001` foi gravado em nosso banco de dados semana passada.
    - A notícia `100` surge hoje e o tratamento de URL identifica que a notícia `001` está presente nela.
    - O tratamento de URLs identifica a notícia `001` e substitui o atributo `href` da tag `<a>` encontrada pela nossa própria URL apontando para esta notícia.
- Mais detalhes desse fluxo na sessão "Fluxo de tratamento".
- A resposta desta etapa deve devolver a nova versão do conteúdo da notícia com tratamento realizado.
- Atenção: note que as URLs tratadas nesta etapa serão alteradas nas etapa "operações no banco de dados".

### Sanitização do conteúdo

- Esta etapa utiliza a biblioteca `bluemonday` para forçar uma `whitelist` de sanitização em cima da versão do conteúdo obtida na etapa anterior.
- Usa uma política customizada para permitir apenas:
    - Tags de formatação (texto, títulos, listas, tabelas e semânticas seguras): `p`, `br`, `hr`, `span`, `strong`, `b`, `em`, `i`, `u`, `s`, `sub`, `sup`, `small`, `mark`, `abbr`, `cite`, `q`, `h1`–`h6`, `ul`, `ol`, `li`, `dl`, `dt`, `dd`, `blockquote`, `code`, `pre`, `kbd`, `samp`, `figure`, `figcaption`, `table`, `thead`, `tbody`, `tfoot`, `tr`, `th`, `td`, `caption`, `colgroup`, `col`.
    - `<a href>` (esquemas `http`/`https`/`mailto`, marcado `rel=nofollow`) e `<img src>` (esquemas `http`/`https`, com `alt`/`width`/`height`).
    - Tags fora da whitelist são "desembrulhadas" (o texto fica, a tag some); atributos fora da whitelist (`class`, `style`, `srcset`, `on*`, ...) são removidos.

#### Whitelist de sanatização

- A `whitelist` de sanatização utilizada pelo `bluemonday` tem como objetivo principal garantir que não haja tags ou atributos ilícitos ou perigosos, como por exemplo:
    - Tags desconhecidas, não-existentes ou perigosas (Exemplo: `<iframe>` fora da `allowlist` abaixo, `<script>`, entre outros).
    - Atributos desconhecidos ou não-existentes ou perigosas (Exemplo: `onclick=`, `onhover=`, entre outros).
- As URLs internas, URLs externas e imagens estão liberdas normalmente.
- Embeddings estão liberados desde que façam parte de uma `allowlist` e não possuam `<script>` (YouTube, Twitch, entre outros).
    - Embeddings mais "complexos" como o Instagram devem ser substituídos por `<a href={url_da_postagem}>{url_da_postagem_reduzida}</a>`.
    - O embed do Twitch só reproduz quando o parâmetro `parent` do `src` bate com o domínio que renderiza. Por isso, na etapa de tratamento de embeds (antes da sanitização), o `parent` do iframe do Twitch é reescrito para o host do `CLIENT_URL`. Sem `CLIENT_URL` configurada, essa reescrita é pulada.

### Nomeação de palavras-chave

- Esta etapa opera em três modos:
    - `local`: utiliza SLM para realizar a execução.
    - `groq`: utiliza serviços do Groq + SLM para realizar a execução.
    - `gemini`: utiliza LLM para realizar a execução.
- Esta etapa opera de acordo com as seguintes variáveis de ambiente:
    - `KEYWORDS_MODE`: o modo atualmente utilizado durante esta etapa.
    - `KEYWORDS_VERBOSE_MODE`: `boolean` que decide se os logs são exibidos no terminal ou não durante esta etapa.
- As palavras-chave nomeadas devem estar em inglês para facilitar o entendimento de quais notícias estão relacionadas.
- As palavras-chave devem ser armazenadas em letras minúsculas.
- A IA recebe o texto puro do conteúdo sem HTML para economizar tokens. O conteúdo gravado continua sendo o HTML sanitizado.
- O prompt pede uma mistura de termos específicos + genéricos (ver regra de palavras-chave em "Notícias").

### Operações no banco de dados

- Esta etapa realiza pequenas operações no banco de dados. Para melhor organização, está separada em quatro passos menores:
    - Gravação da notícia.
    - Associação das URLs da notícia.
    - Reorganização das referências.
    - Alteração de URLs de outras notícias.
- Ao final desta etapa, o julgamento está pronto para ser realizado.

#### Gravação da notícia

- Neste passo a notícia é salva no banco de dados.
- Cada registro criado é mantido para a etapa de julgamento.

#### Associação das URLs da notícia

- Neste passo o conteúdo da notícia recém salva é analisado em busca de cada URL presente no atributo `href` das tags `<a>`.
    - Para cada uma encontrada, um registro é criado em `article_outbound_links`, preenchendo o campo `href` com a URL e o `article_id` com o id da notícia.
- Agora que temos os valores de `id` de cada registro em `article_outbound_links`, são eles quem substituem a URL presente no atributo `href` das tags `<a>` da notícia.

#### Reorganização das referências

- Neste passo, a notícia recém salva é alterada, trocando o `href` das tags `<a>` de seu conteúdo pelo `id` de `article_outbound_links`.

#### Alteração de URLs de outras notícias

- Neste passo a `url_original` da notícia recém salva é buscada dentro no campo da tabela `article_outbound_links`.
    - Para cada um encontrado, o registro é alterado com a nova URL interna (`{CLIENT_URL}/articles/{id}`) ao invés da externa original.

## Julgando se uma notícia pertence ao feed do usuário

- Quando uma notícia é descoberta um processo de julgamento é acionado para saber a qual feed aquela notícia será associada.
- Esta etapa opera em três modos:
    - `local`: utiliza SLM para realizar a execução da etapa de julgamento.
    - `groq`: utiliza serviços do Groq + SLM para realizar a execução da etapa de julgamento.
    - `gemini`: utiliza LLM para realizar a execução da etapa de julgamento.
- Esta etapa opera de acordo com as seguintes variáveis de ambiente:
    - `JUDGEMENT_MODE`: o modo atualmente utilizado durante esta etapa.
    - `JUDGEMENT_VERBOSE_MODE`: `boolean` que decide se os logs são exibidos no terminal ou não durante esta etapa.
    - `JUDGEMENT_THRESHOLD`: `threshold` de score da IA para considerar notícias pertencentes a um feed.
    - `JUDGEMENT_AUTOASSOCIATE_RATIO`: fração (0-1) das keywords do feed que a notícia precisa cobrir para ser auto-associada sem passar pela IA.
    - `JUDGEMENT_MIN_MATCHES`: mínimo de keywords em comum para um candidato chegar na IA; abaixo disso é descartado também sem IA.
- O julgamento funciona através de três etapas:
    - Preparação (busca por usuários aptos + comparação de palavras chave): busca quais usuários estão aptos a receber a notícia para depois realizar uma filtragem em SQL que encontra os feeds candidatos e conta quantas keywords cada um casou (overlap)
    - Julgamento (triagem + IA): usando o overlap, cada candidato é auto-associado (overlap alto), descartado (overlap trivial) ou enviado à IA (borderline). Isso mantém o custo de IA baixo mesmo quando uma notícia genérica casa muitos feeds.
    - Gravação no banco de dados: forma um registro da associção entre feed e notícia no banco de dados.
- O julgamento de notícias nunca é feito de forma retroativa.
- O julgamento de notícias só pode considerar feeds que estão ativos.
- Um endpoint de teste para esse processo pode ser encontrado em `POST base_url/v1/articles/judgement`.
    - Deve simular os exatos mesmos processos que rodaria na CRON.
    - Deve permitir a troca de modo na etapa de julgamento através de uma chave chamada `judgement_mode` no body, aceitando os valores disponibilizados na descrição da etapa.
    - Esse endpoint é exclusivo do modo de desenvolvimento, ou seja, a rota nem sequer é registrada fora de dev (mesmo padrão do `dev-login`), pois é uma ferramenta interna que chama a IA de verdade e jamais deve ser alcançada por clientes.
    - Esse endpoint é considerada uma dry-run, ou seja, ela não cria ou altera nenhum registro do banco de dados.
    - O body deste endpoint deve aceitar um `json` contendo os dados necessários para se fazer a simulação de um julgamento.
        - Evite a necessidade de passar um `id` de notícias válido, assim poderemos fazer simulações mais rapidamente.
    - Esse endpoint responde os dados do julgamento de notícias antes da gravação no banco de dados, ou seja, até a penúltima etapa.

### Preparação (busca por usuários aptos + comparação de palavras chave)

- A preparação serve para garantir que a notícia vá chegar apenas para os usuários ativos e realizar uma filtragem de quais feeds são os melhores candidatos a seguirem adiante através de uma comparação de palavras-chave da notícia.
    - A sessão de usuários trata as regras sobre inatividade.
- Essa filtragem é feita inteiramente em SQL.
    - As palavras-chave (tanto da notícia quanto dos feeds) são armazenadas como arrays JSON de strings minúsculas, então usamos `jsonb_array_elements_text` para expandir ambos os lados em linhas e um `JOIN` por igualdade exata de palavra-chave.
    - Um feed é candidato quando tem pelo menos uma palavra-chave em comum com a notícia.
    - A query agrupa por feed e **conta o overlap** (`COUNT(DISTINCT ...) AS overlap_count`) — esse número é o sinal usado pela triagem (query `FindCandidateFeedsByKeywords`).

### Julgamento (triagem + IA)

- A triagem é um passo barato baseada no overlap de keywords (o `ratio` é `overlap / nº de keywords do feed`):
    - `ratio >= JUDGEMENT_AUTOASSOCIATE_RATIO` faz uma auto-associação sem IA, ou seja, overlap forte é sinal suficiente.
    - overlap `< JUDGEMENT_MIN_MATCHES` faz descarte sem IA.
    - o resto (bateu >= `JUDGEMENT_MIN_MATCHES` mas ratio < `JUDGEMENT_AUTOASSOCIATE_RATIO`) marca para borderline e vai para a IA.
- Dessa forma o custo de IA não escala de forma notícias x feeds candidatos, a triagem tira o grosso (feeds fortes e feeds fracos) de graça e só manda o meio para a IA, desacoplando o custo do total de feeds.
- Filosofia dos riscos: auto-associar errado (falso positivo) é chato mas recuperável; descartar errado (falso negativo) faz o usuário nunca ver a notícia. Por isso o descarte é conservador (só 1 keyword), enquanto o auto-associar usa uma fração; e o threshold/ratio são calibráveis por env. O ajuste fino "de verdade" (peso por especificidade da keyword / embeddings) é futuro.
- O passo de IA define um `score` entre 0 e 100 a partir de `título + keywords` da notícia contra as keywords do feed, comparado ao `JUDGEMENT_THRESHOLD`. Como o corpo não é enviado, o threshold deve ser re-calibrado quando esses parâmetros mudarem.

### Gravação da associação no banco de dados

- Na última etapa, um novo registro na tabela associativa (junction table) entre feed e notícias é criado.

## Traduzindo notícias

- A tradução de notícias é uma capacidade read-only disparada pelo usuário através do client.
- O idioma-alvo da tradução é a preferência `language_to_translate` do usuário, lida do `access_token` (não vem da URL). Ou seja, é a API quem decide o alvo a partir da preferência; o client apenas altera esse valor.
    - Quando a preferência `language_to_translate` for nula, não há alvo configurado e a tradução não pode ser feita (400).
- A tradução de notícias só pode ser feita quando o `language_to_translate` alvo for diferente da `language_original` da notícia.
    - Caso a `language_original` esteja em `null`, a tradução não poderá ser feita.
- É responsabilidade do client guardar essa tradução feita como um cache.
    - O cache neste projeto via `Redis` será implementado futuramente.
- A tradução de notícias também leva em conta a personalidade escolhida pelas preferências do usuário na opção `ai_personality`.
- A tradução de notícias deve:
    - Traduzir o título e conteúdo da notícia para serem mostrados ao usuário.
    - Atuar sempre em read-only.
    - Preservar a estrutura HTML presente no conteúdo da notícia.
    - Manter o contexto do conteúdo.
    - Personalizar a tradução da notícia de acordo com a preferência escolhida em `ai_personality`.
- Dito isso, a tradução não pode:
    - Fazer operações no banco de dados: estritamente proibido fazer qualquer alteração no banco de dados neste momento.
    - Resumir notícias: estritamente proibido fazer resumo neste momento.
    - Alterar sentido, sintaxe, ideia ou contexto do conteúdo da notícia.
    - Trazer opinião própria.
- Ao final da chamada da tradução uma nova sanatização deve ser realizada no conteúdo do resultado final para garantir que não haja elementos HTML indesejados aplicados ao resultado final.
    - O método pode ser o exato mesmo utilizado no tratamento de notícias.
- O endpoint `GET /v1/articles/{id}/translate` é o responsável pela tradução de notícias:
    - O `id` é referente à notícia a ser procurada no banco de dados para ser traduzida.
    - O idioma-alvo **não** é passado na URL: vem da preferência `language_to_translate` do usuário (lida do `access_token`).
    - Se a preferência `language_to_translate` for nula (sem alvo configurado), o resultado deve ser 400.
    - Se o `language_to_translate` alvo e a `language_original` da notícia forem iguais, o resultado deve ser 400.
    - Se a `language_original` da notícia for `null`, o resultado deve ser 400.

## Resumindo notícias

- O resumo de notícias será feito em versões futuras.
- O resumo de notícias precisa obrigatoriamente da implementação do Redis no projeto.

## Administradores

- Os administradores da aplicação são usuários comuns que possuem uma flag `admin` marcada como `true` na tabela `users`.
- Uma vez administradores, os usuários sempre são tratados como administradores e não mais como usuários comuns.
    - Apenas para documentação: no client há um botão que faz com que usuários administradores possam ver a aplicação como usuarios comuns, mas isso não afeta as requisições feitas.
- A autorização deve sempre ler a flag no banco de dados e nunca do `access_token`.
    - O `access_token` carrega um claim `admin` e o `GET base_url/v1/users/me` devolve o mesmo campo, mas ambos servem apenas como dica para o client decidir o que renderizar. É a mesma separação já usada em `language_to_translate`.
    - O motivo é a revogação: se a decisão viesse do token, tirar o acesso de alguém só valeria quando o token dele expirasse (até `JWT_ACCESS_TOKEN_EXPIRY_MINUTES`), que é justamente o momento em que esperar é inaceitável. Lendo o banco, promover ou rebaixar vale já na requisição seguinte.
    - O custo é uma consulta por requisição, paga apenas nas rotas exclusivas de administrador (baixo tráfego por natureza) e durante manutenção.
    - A consequência é que ao promover alguém vale na API imediatamente, mas o client só enxerga a mudança quando o token for renovado. Como o claim só decide o que aparece na tela, o atraso é cosmético.
- Um token emitido antes da existência do claim decodifica `admin` como `false`, ou seja, o padrão é sempre apontar como usuário comum.
- Não é possível criar um administrador pelo fluxo de login: a query de criação de usuário não escreve essa coluna. A promoção é um ato separado e explícito.
- Os administradores não são afetados por flags como a `system.app_status` ou derivados. Eles sempre podem fazer o que quiser mesmo que algo esteja inativo no momento.
    - Dito isso, se uma requisição estiver sendo feita por um administrador, ela precisa ser executada independente de qualquer inatividade atual da aplicação.
    - Detalhes de como isso funciona (e por que `/v1/auth` é isento da manutenção) estão em "Painel de controle".
- Os seguintes endpoints são de acesso exclusivos pelos administradores:
    - `POST base_url/v1/articles/create`.
    - `DELETE base_url/v1/articles/{id}`.
    - `PUT base_url/v1/articles/{id}`.
    - `DELETE base_url/v1/auth/invalidate`.
    - `DELETE base_url/v1/auth/invalidate-all`.
    - `POST base_url/v1/sources/create`.
    - `PUT base_url/v1/sources/{id}`.
    - `GET base_url/v1/sources/{id}/article-discovery`
    - `DELETE base_url/v1/sources/{id}`.
    - `PUT base_url/v1/system/app-status`.
- Os códigos de resposta desses endpoints seguem o padrão:
    - Sem `access_token` válido: `401`.
    - Com `access_token` válido de usuário comum: `403`.
    - Falha ao consultar a flag no banco: `500`. Uma falha de infraestrutura nunca vira `403` pois dizer a um administrador legítimo que ele perdeu o acesso porque o banco piscou o mandaria caçar um problema de permissão que não existe.
    - A checagem roda antes da validação do corpo da requisição, então um usuário comum recebe `403` mesmo enviando um payload inválido.
- Por enquanto para um usuário se tornar administrador faremos apenas a alteração via banco de dados, ou seja, não há uma lista de e-mails fixa ou algo do gênero.
    - Em desenvolvimento, o usuário criado pelo seed (`task sud` / `task sdfull`) já nasce administrador, e um usuário de seed criado antes disso é promovido na próxima execução do comando.

### Painel de controle

- Usuários administradores tem acesso à alguns endpoints exclusivos da tabela `system`.
- A tabela `system` é um grande painel de controle que é capaz de ativar ou desativar funcionalidades de forma temporária para resolver erros momentâneos.
- As atuais funcionalidades são:
    - `app_status`: estado da aplicação. Todos os endpoints devem garantir que o estado atual da aplicação é `true`. Quando em `false` o erro deve ser 503.
    - `last_article_discovery_at`: data do último descobrimento de notícias.
- Novas funcionalidades virão futuramente.

#### Manutenção (`app_status` em `false`)

- As rotas abaixo continuam respondendo normalmente durante a manutenção, pois são registradas antes do guard:
    - `GET base_url/v1/health`: para o monitoramento continuar enxergando a aplicação.
    - `PUT base_url/v1/system/app-status`: para a aplicação sempre poder ser religada pela própria API.
    - Todo o grupo `base_url/v1/auth`: login, callback e refresh.
- A isenção do `/auth` não é conveniência, é a correção de um travamento: o bypass identifica o administrador pelo `access_token`, mas ele expira em `JWT_ACCESS_TOKEN_EXPIRY_MINUTES` e o `refresh_token` não carrega identidade que o bypass consiga ler. Com o `/auth` atrás do guard, um administrador cujo token expirasse durante a manutenção receberia 503 justamente do `refresh` — a única chamada que o levaria de volta ao endpoint que encerra a manutenção — e a única saída seria alterar o banco na mão.
    - O custo disso é que um usuário comum também consegue logar e renovar token durante a manutenção. Como todo o resto responde 503 para ele, isso não dá acesso a nada.
- Requisições de administradores atravessam a manutenção normalmente. A verificação de quem está chamando só acontece quando a aplicação já está desligada, então o caminho normal (aplicação no ar) não paga consulta nenhuma a mais.
- Durante a manutenção a identificação do administrador é fail-closed: header ausente, token inválido, sessão encerrada (logout/invalidate) ou falha de banco resultam em 503. Em particular, um administrador que fez logout não atravessa a manutenção só porque o token dele ainda não expirou.

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
    - Se um dia for necessário isolar um controller em teste unitário, o mock deve ser gerado a partir da interface `db.Querier` (exemplo: `mockgen`) e nunca escrito manualmente para evitar o esquecimento em uma atualização.

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
    - `OAuth2`: garantir que a configuração atende aos requisitos mínimos para funcionamento natural do processo de testes (exemplo: checagem de variáveis de ambiente `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` e `GOOGLE_REDIRECT_URL`).
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

- Servem como scripts utilitários para a execução de testes. Sinta-se livre para criar qualquer utilidade aqui, como subir uma instância PostgreSQL temporária com as migrações, por exemplo.

## Paginação

- A paginação deve estar presente em todos os endpoints de:
    - Buscas de múltiplos registros de um modelo (exemplo: `GET base_url/v1/articles/`).
    - Buscas de registros cruzados de modelos (exemplo: `GET base_url/v1/feeds/{id}/articles`)
- A paginação não precisa ser feita ao montar relatórios, métricas, indicadores ou afins.
    - Endpoints que devolvem um indicador ranqueado em vez de uma listagem de registros caem nesta isenção e usam um `limit` simples (sem `page`/`page_size`): hoje `GET /v1/feeds/check-for-new-articles` e `GET /v1/feeds/keyword-suggestions`.
- A paginação e a filtragem devem ser feitas **em SQL**, nunca em Go: a query traz apenas a página
  pedida (`LIMIT/OFFSET`) e uma query irmã de contagem, com os mesmos filtros, fornece o `total_count`.
    - O `total_count` é o total de registros que casam com o filtro, e não o tamanho da página. Por
      isso ele vem de uma contagem no banco: numa página fora do range não há linha nenhuma para
      carregar esse número.
    - As listagens são ordenadas com desempate pelo `id` (`created_at DESC, id DESC`). Sem uma
      ordenação total, registros criados no mesmo instante (um lote da CRON, por exemplo) podem
      repetir em uma página e sumir de outra ao paginar.
    - Processos internos que precisam do conjunto inteiro (a CRON e os scripts de `cmd/`) não usam as
      listagens paginadas: eles têm métodos próprios (`ListAll`) sem limite.
- Ao usar paginação, a resposta deve seguir o seguinte formato `json`:
    ```json
    {
        "docs": [{...}, {...}], // Lista de registros
        "pagination": {
            "actual_page": "a página atual",
            "total_pages": "total de páginas",
            "actual_count": "contador de registros desta página",
            "total_count": "contador total de registros em todas as páginas",
            "has_next_page": "indicador rápido para saber se há uma próxima página",
            "has_previous_page": "indicador rápido para saber se há uma página anterior",
        }
    }
    ```
- Queries devem usar as seguintes nomenclaturas:
    - `?page=`: trafega entre as páginas. Valor mínimo e padrão em `1`.
    - `?page_size=`: número de registros a serem trazidos. Valor mínimo `1`, máximo `100` e padrão `20`.
- Queries erradas de páginas retornam uma lista vazia ao invés de erro.
    - Por exemplo, se a query requisitar a página 5 mas há apenas 4 páginas disponíveis não há erro. O valor da paginação `actual_page` irá estar em `5` e o valor de `total_pages` estará em `4`.

## CORS

- As CORS são tratadas através da variável de ambiente `CORS_ALLOWED_ORIGINS` e deve ser uma lista separada por vírgula no formato `protocol+host+port`.
    - Em ambiente de desenvolvimento deve liberar `localhost` enquanto em produção e homologação apenas os domínios reais.

## CI/CD

- A estrutura de CI/CD fica organizada na pasta `.github/workflows/`.
- O CI constrói a imagem do Docker uma única vez e a envia para um registry (AWS ECR); depois o servidor apenas dá pull daquela tag e sobe.
    - Dessa forma, o servidor nunca vê o código-fonte e nem builda a imagem.
    - Assim o rollback se torna apenas apontar para uma tag anterior.
- O arquivo `ci.yml` roda em todo PR e push no branch padrão:
    - `gofmt`
    - `go vet`
    - `go test ./...`
- O arquivo `deploy.yml` funciona apenas nas branchs `staging` e `production`.
    - O fluxo é build -> push ECR -> deploy no EC2 via SSH.
    - Autenticação AWS por OIDC (sem chave estática).
    - Segredos de deploy\_(role AWS via OIDC, chave SSH, host) ficam no GitHub (secret do repositório + secrets por Environment). Não confundir com os segredos de runtime da aplicação (`JWT_SECRET_KEY`, senha do banco de dados, API keys): esses não ficam no GitHub, vêm sempre do `.env.<ambiente>` no próprio servidor (modelo atual).
        - Futuramente virá de um secret store da AWS (SSM Parameter Store / Secrets Manager).
    - A variável `DEPLOY_ENABLED` deve estar em `true` para que o deploy ocorra.
    - A lista completa de variáveis/secrets necessárias e o que o host precisa ter está em `.github/workflows/README.md`.

## Backup

- A implementação de backups será feita futuramente.

## Ambientes

- O atual ambiente sempre está na variável de ambiente chamada `ENVIRONMENT` e devem estar sempre em um desses três valores:
    - `development`: ambiente de desenvolvimento.
    - `staging`: ambiente de homologação.
    - `production`: ambiente de produção.

### Ambiente de desenvolvimento

- Este ambiente possui diversas habilidades exclusivas de desenvolvimento para cortar caminhos.
- As pastas e comandos relacionados ao `raiz/cmd/` são exclusivos para serem usados neste ambiente.
- As pastas e comandos relacionados ao `raiz/test/` está liberada para ser usada.
- Há um único `Taskfile.yaml`. As habilidades exclusivas de desenvolvimento (rotas `dev-login`/`treatment`/`judgement` e os comandos de seed em `cmd/`) são liberadas pela variável `ENVIRONMENT=development` ao invés de um Taskfile separado.
- Rotas exclusivas do ambiente de desenvolvimento:
    - `POST base_url/v1/users/dev-login`: loga o usuário dev diretamente, emitindo um `access_token` e um `refresh_token` sem a necessidade de `OAuth2`.

#### Pasta cmd

- Possui uma coleção de scripts e comandos task para serem executados no ambiente de desenvolvimento.
- Nunca devem ser usados em homologação ou produção.

### Ambiente de homologação

- Este ambiente simula o ambiente de produção da maneira mais próxima possível.
- A aplicação atua apenas via Docker neste ambiente, pelo overlay `docker-compose.staging.yaml` (`task staging-up`). Usa um Postgres em container (dado descartável).
- Push no branch `staging` dispara o `deploy.yml` → build da imagem → push no ECR → `pull` no host + `up -d`. O host não builda nem faz `git pull`; ele só tem os composes e o `.env.staging` (com os segredos) e roda a imagem nova.

### Ambiente de produção

- Este é o ambiente de produção, todo cuidado é pouco.
- A aplicação atua apenas via Docker neste ambiente, pelo overlay `docker-compose.production.yaml` (`task prod-up`).
- O banco de dados vive em RDS por padrão, ou seja, sem container de Postgres e com a variável de ambiente `DATABASE_URL` apontando pro RDS.
- Roda em **instância única**: a CRON de descoberta é in-process, então uma segunda réplica rodaria a descoberta de novo (notícia e chamada de IA duplicadas). Se precisar de mais réplicas, apenas uma pode ter `RSS_FEED_CRON_ACTIVE=true`.
- Push na branch `production` dispara o `deploy.yml`.

### Variáveis globais úteis

- A variável `CLIENT_URL` possui a URL base do client.

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
