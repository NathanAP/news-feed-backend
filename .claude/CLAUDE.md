# Resumo

API SQLite com Go para um feed de notícias personalizado usando fontes RSS.

# Regras de desenvolvimento

- Siga as convenções e regras presentes na pasta `rules`.
- Leia o arquivo `ROADMAP.md` para entender qual sua próxima missão.
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
    - Atualização nos testes unitários, de integração e end-to-end.
    - Atualização de arquivos de controle de versão.
    - Criar um arquivo na pasta `versions` para resumir o que foi feito.
    - Atualização de arquivos de controle de memória.
    - Resumir o que foi feito.
    - Explicar a melhor forma de testar o que foi feito (quando aplicável).
- Sempre atualize a versão do arquivo `ROADMAP.md` conforme a versão desenvolvida.
- Ao seguir o arquivo `ROADMAP.md`, desenvolva uma versão de cada vez e confirme comigo antes de avançar para a próxima etapa.
- Ao analisar as sprints do arquivo `ROADMAP.md`, você tem liberdade para criticar ou corrigir tamanho de sprint, altas complexidades de código ou regras de negócio, assim como quebras de fluxos já escritos, conflito de convenções e até mesmo dificuldades futuras. Isso fará com que nós possamos pensar juntos em soluções e deixará a aplicação melhor. Seu papel nisso é fundamental.
- Você tem liberdade para corrigir erros claros de ortografia na documentação, como falta de letras ou acentos.
- Você tem liberdade em decidir que uma revisão de código usando um modelo maior é necessária quando uma nova versão for feita ou muito tempo tenha se passado desde a última revisão.

# Regras de revisão

- Modelo mínimo a ser usado: Claude Opus 4.8. Você deve parar a revisão caso o modelo selecionado tiver sido abaixo deste.
- O revisor deve garantir a qualidade e consistência do código.
- O revisor deve fornecer feedbacks construtivos em qualquer altura, seja sobre o fluxo do código ou até mesmo a documentação.
- O revisor deve garantir que o código segue as regras, convenções e filosofias do projeto.
- O revisor deve indicar quais são as melhorias, os motivos e como fazer elas.
- O revisor deve elencar problemas de fluxo não notados anteriormente (exceções de fluxos ou situações adversas).
- O revisor deve garantir que o versionamento (`.claude/versions`) está consistente.
- O revisor pode elencar problemas em aplicativos externos (como o Bruno).
- Alterações causados pelo revisor sobem uma versão de patch (por exemplo, se a revisão `10.1.2.15` gerou um bug e foi consertado, a nova versão deve ser `10.1.3.0`).
- Bugs graves devem ter preferência e podem garantir uma versão única de patch.
- Refatorações estão liberadas conforme necessário, mas faz-se necessário o planejamento junto a mim.
- O revisor tem total incentivo para elencar também os seguintes pontos como problema:
    - Código inutilizado ou morto.
    - Gambiarra explícita.
    - Inconsistência de versionamento (como `UUID4` ao invés de `UUID7`).
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

- `GoLang` - linguagem base
- `Fiber` - framework web
- `SQLite3` - banco de dados
- `sqlite-web` - interface de visualização do banco de dados
- `goose` - migrações do banco de dados
- `sqlc` - operações SQL
- `jwt` - autenticação Bearer
- `gofeed` - parsing de RSS feeds
- `Google Gemini 2.5 Flash` - modelo LLM padrão
- `Ollama qwen3:4b` - modelo SLM padrão
- `Groq` - serviço online para SLM
- `Docker + docker-compose` - orquestração
- `testify` - biblioteca para testes
- `robfig/cron/v3` - biblioteca em Go para criar CRONs nativamente
- `Bruno` - app externo para teste e organização da coleção de requisições

## Regras da stack

- Dependências devem sempre estar na versão mais atualizada possível.
- UUIDs devem estar na versão 7.
- Modelos LLMs e SLMs devem ser os exatos indicados na stack.

## Comandos

- `go run main.go` - Rodar localmente
- `go build -o api` - Build executável
- `docker-compose up` - Rodar com Docker
- `go mod tidy` - Limpar dependências

## Inicialização rápida

```bash
go mod download
go run main.go
```

## Porta

- A porta da API está presente na variável de ambiente `API_PORT`.
