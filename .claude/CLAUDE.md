# Resumo

API SQLite com Go para um feed de notícias personalizado usando fontes RSS.

# Regras de desenvolvimento

- Siga as convenções e regras presentes na pasta `raiz/.claude/rules/`.
- Leia o arquivo `raiz/.claude/ROADMAP.md` para entender qual sua próxima missão.
- Faça apenas uma versão por vez conforme especificado. Esta regra é restrita.
- Você pode está livre para indicar problemas que versões futuras podem trazer ou estejam mal planejadas.
- Ao desenvolver tente entender um pouco como as próximas versões e as até os planejamentos de longo prazo afetam seu código.
- Você tem liberdade para indicar a mim problemas de planejamento.
- Você tem liberdade para durante a criação de testes unitários, de integração e end-to-end.
- Durante a etapa de criação de testes, não tem problema se houver grande volume de código ou tempo para as tantas diversas situações que podem ocorrer em uma rota ou handler. O importante é garantir que ela cobre a maioria das situações.
- O desenvolvimento deve seguir estas etapas:
    - Entendimento da tarefa.
    - Melhoria da clareza escrita na especificação da tarefa.
    - Planejamento de programação da tarefa.
    - Execução.
    - Atualização do arquivo `raiz/.claude/rules/structure.md` caso necessário.
    - Atualização nos testes unitários, de integração e end-to-end presentes em `raiz/tests/`.
    - Atualização nos scripts de teste presentes em `raiz/cmd/`.
    - Atualização de arquivos de controle de versão.
    - Criar um arquivo na pasta `versions/` para resumir o que foi feito.
    - Atualização de arquivos de controle de memória.
    - Resumir o que foi feito.
    - Explicar a melhor forma de testar o que foi feito (quando aplicável).
- Sempre atualize a versão do arquivo `raiz/.claude/ROADMAP.md` conforme a versão desenvolvida.
- Ao seguir o arquivo `raiz/.claude/ROADMAP.md`, desenvolva uma versão de cada vez e confirme comigo antes de avançar para a próxima etapa.
- Ao analisar as sprints do arquivo `raiz/.claude/ROADMAP.md`, você tem liberdade para criticar ou corrigir tamanho de sprint, altas complexidades de código ou regras de negócio, assim como quebras de fluxos já escritos, conflito de convenções e até mesmo dificuldades futuras. Isso fará com que nós possamos pensar juntos em soluções e deixará a aplicação melhor. Seu papel nisso é fundamental.
- Você tem liberdade para corrigir erros claros de ortografia na documentação, como falta de letras ou acentos.
- Você tem liberdade em decidir que uma revisão de código usando um modelo maior é necessária quando uma nova versão for feita ou muito tempo tenha se passado desde a última revisão.

# Regras de revisão

- Modelo mínimo a ser usado: Claude Opus 4.8. Você deve parar a revisão caso o modelo selecionado tiver sido abaixo deste.
- O revisor deve garantir a qualidade e consistência do código.
- O revisor deve fornecer feedbacks construtivos em qualquer altura, seja sobre o fluxo do código ou até mesmo a documentação.
- O revisor deve garantir que o código segue as regras, convenções e filosofias do projeto.
- O revisor deve indicar quais são as melhorias, os motivos e como fazer elas.
- O revisor deve elencar problemas de fluxo não notados anteriormente (exceções de fluxos ou situações adversas).
- O revisor deve garantir que o versionamento (`raiz/.claude/versions`) está consistente.
- O revisor pode elencar problemas em aplicativos externos (como o Bruno).
- Alterações causados pelo revisor sobem uma versão de patch (por exemplo, se a revisão `10.1.2.15` gerou um bug e foi consertado, a nova versão deve ser `10.1.3.0`).
- Bugs graves devem ter preferência e podem garantir uma versão única de patch.
- Refatorações estão liberadas conforme necessário, mas faz-se necessário o planejamento junto a mim.
- O revisor tem total incentivo para elencar também os seguintes pontos como problema:
    - Código inutilizado ou morto.
    - Código que gera problemas de performance (exemplo: endpoint de filtragem que aplica filtros fora do SQL).
    - Código que não respeita a filosofia de programação da linguagem (exemplo: erros não tipados em Go).
    - Má aplicação de convenções básicas (exemplo: endpoint `GET` que recebe `body`).
    - Tipagem errada.
    - Gambiarra explícita.
    - Código considerado depreciado pela biblioteca ou semi-depreciado (ou seja, que vai se tornar depreciado).

# Regras de memória

- Use a pasta `.claude/memory/` para armazenar um resumo atualizado sobre o projeto, as regras e o projeto como um todo.
- Ela é uma pasta entendida como "comece aqui para entender resumidamente o projeto", então poderá ser usada por outros Claudes relacionados a este mesmo projeto (como o client) para entender resumidamente o escopo do projeto.
    - Minha recomendação é uma explicação sobre a tecnologia, os endpoints, a estrutura do banco de dados, os modelos, os controllers, entre outros.
    - Outra recomendação é separar de acordo com o assunto, assim se outra instância do projeto vier, pode apenas olhar ali.
        - Por exemplo, se o Claude do projeto do web client vier entender as rotas de um endpoint, ele pode olhar diretamente ali ao invés de todo o código que ele precisa.
- Você provavelmente quer atualizar os arquivos presente ali na maioria das tarefas conforme as mudanças de documentação e escopo acontecem.
- Não exponha dados sensíveis ou valores de variáveis de ambiente, apenas a chave delas quando necessário.
- Eu não vou controlar quais arquivos você vai ter ali dentro, porém vou fazer questionamentos se encontrar inconsistências lógicas ou incoerentes com a realidade do projeto.
- Os arquivos são seus, quem escreve é você do seu jeito. Faça seu melhor para ficar claro. Lembre-se apenas que ele é um resumo geral, não uma reescrita.
- Novamente: a ideia é que a pasta sirva como um resumo, então você pode e deve continuar consultando os arquivos da pasta `.claude` para ter mais detalhes sobre um tópico específico.

## Arquivos e pastas

Você tem liberdade para acessar qualquer arquivo da pasta `.claude`.

- `CLAUDE.md`: contém um resumo geral e técnico do projeto.
- `PROJECT.md`: contém um resumo de como o projeto funciona (filosofia, fluxos, features, restrições, etc).
- `ROADMAP.md`: contém o roadmap do projeto, que também pode ser visto como uma lista de TODO.
- `agents/`: contém os agentes que dão suporte e estão presentes no desenvolvimento do projeto.
- `rules/`: contém um conjunto de regras para ajudar no desenvolvimento do projeto.
- `memory/`: contém um conjunto de resumos criados por você mesmo para ajudar a sua memória ser mais enxuta e não depender de ler todo o projeto toda vez.
- `versions/`: contém um conjunto de arquivos especificando o que foi feito por você em cada versão do projeto.

## Stack

- `GoLang`: linguagem base.
- `Fiber`: framework web.
- `SQLite3`: banco de dados.
- `sqlite-web`: interface de visualização do banco de dados.
- `goose`: migrações do banco de dados.
- `sqlc`: operações SQL.
- `jwt`: autenticação Bearer.
- `gofeed`: parsing de RSS feeds.
- `Google Gemini 2.5 Flash`: modelo LLM padrão.
- `Ollama qwen3:4b`: modelo SLM padrão.
- `Groq`: serviço online para SLM.
- `Docker + docker-compose`: orquestração.
- `bluemonday`: biblioteca de sanatização de conteúdo HTML.
- `lingua-go`: biblioteca que detecta o idioma de um texto ou conteúdo.
- `testify`: biblioteca para testes.
- `robfig/cron/v3`: biblioteca em Go para criar CRONs nativamente.
- `Bruno`: app externo para teste e organização da coleção de requisições.

## Regras da stack

- Bibliotecas da devem sempre estar na versão mais atualizada possível.
- UUIDs devem estar na versão 7.
- Modelos LLMs e SLMs devem ser os exatos indicados na stack.
- O `lingua-go` deve ser configurado para disponibilizar os seguintes idiomas:
    - Português
    - Inglês
    - Espanhol
    - Francês
    - Alemão
    - Italiano

## Comandos

O fluxo padrão é via **`task`** (Taskfile na raiz; os comandos de seed vivem em `cmd/seed/Taskfile.yaml`
e são chamados direto da raiz). Rodar `task` (ou `task help`) lista tudo. Aliases entre parênteses.

**Setup (uma vez):**

- `task setup` — instala ferramentas (sqlc, task, sqlite-web), baixa deps e puxa o SLM de keywords no Ollama.
- `task ollama-pull` (`op`) — só puxa o modelo SLM local (`qwen3:4b`).

**Rodar local (sem Docker):**

- `task local-start` (`ls`) — build + sobe em background; logs em `./tmp/`. Migrações rodam no boot.
- `task local-logs` (`ll`) — acompanha os logs em tempo real.
- `task local-restart` (`lr`) · `task local-down` (`ld`) — reinicia · para.
- `go run main.go` — roda em foreground (útil pra ver o log direto no terminal).

**Docker:**

- `task docker-start` (`ds`) — sobe API + sqlite-web via compose. `dr` (restart) · `dfr` (rebuild) · `dd` (down) · `dp` (prune).

**Banco:**

- `task db` — abre o sqlite-web em `http://localhost:8080`.

**Testes e qualidade:**

- `task test-all` (`ta`) — todos os testes. Por camada: `tu` (unit) · `ti` (integração) · `te2e` (end-to-end).
- `task vet-all` (`va`) — `go vet ./...`. `task fmt` (`f`) — `gofmt`. `task sqlc-generate` (`sg`) — regenera o sqlc.

**Seed de dev** (só com `ENVIRONMENT=development`):

- `task sdfull` — semeia tudo (usuário, sources, feeds, artigos, associações) e imprime um token.
- Individuais: `sud` (usuário) · `sds` (sources) · `sdf` (feeds) · `sda` (artigos) · `sdaf` (associações) · `sdl` (só emite/imprime um token).

## Inicialização rápida

Do zero, com um clone limpo:

```bash
# 1. Ferramenta de build (uma vez, se ainda não tiver o `task`)
go install github.com/go-task/task/v3/cmd/task@latest

# 2. Instala deps, sqlc, sqlite-web e o SLM local
task setup

# 3. Configura o ambiente: copie e preencha os segredos
cp .env.example .env   # defina JWT_SECRET_KEY e as chaves que for usar (Google/Groq)

# 4. Suba a API (migrações rodam automaticamente no boot)
task ls                # ou `go run main.go` para rodar em foreground

# 5. (dev) Popule o banco e obtenha um token para o Bruno/client
task sdfull            # ou `task sdl` só para imprimir um token
```

Sem client web ainda, o token de dev sai do `task sdl`/`task sdfull` ou do `POST /v1/users/dev-login`
(ambos só em `ENVIRONMENT=development`) — não é preciso passar pelo fluxo do Google.

## Porta

- A porta da API está presente na variável de ambiente `API_PORT`.
