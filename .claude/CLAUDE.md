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
    - entendimento da tarefa.
    - melhoria da clareza escrita na especificação da tarefa.
    - planejamento de programação da tarefa.
    - execução.
    - atualização nos testes unitários, de integração e end-to-end.
    - atualização de arquivos de controle de versão.
    - criar um arquivo na pasta `versions` para resumir o que foi feito.
- Sempre atualize a versão do arquivo `ROADMAP.md` conforme a versão desenvolvida.
- Ao seguir o arquivo `ROADMAP.md`, desenvolva uma versão de cada vez e confirme comigo antes de avançar para a próxima etapa.
- Ao analisar as srpints do arquivo `ROADMAP.md`, você tem liberdade para criticar ou corrigir tamanho de sprint, altas complexidades de código ou regras de negócio, assim como quebras de fluxos já escritos, conflito de convenções e até mesmo dificuldades futuras. Isso fará com que nós possamos pensar juntos em soluções e deixará a aplicação melhor. Seu papel nisso é fundamental.
- Ao final de uma atualização de código sempre me traga um resumo sobre o que foi feito.
- Ao final de uma atualização do código, quando aplicável, me traga uma forma de como testar o que foi feito.
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

## Arquivos e pastas

- `CLAUDE.md`: contém um resumo geral e técnico do projeto.
- `PROJECT.md`: contém um resumo de como o projeto funciona (filosofia, fluxos, features, restrições, etc).
- `ROADMAP.md`: contém o roadmap do projeto, que também pode ser visto como uma lista de TODO.
- `agents/`: contém os agentes que dão suporte e estão presentes no desenvolvimento do projeto.
- `rules/`: contém um conjunto de regras para ajudar no desenvolvimento do projeto.
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
- `Gemini 2.5 Flash` - agente básico
- `Docker + docker-compose` - orquestração
- `testify` - biblioteca para testes
- `Bruno` - app externo para teste e organização da coleção de requisições

## Regras da stack

- Dependências devem sempre estar na versão mais atualizada possível.
- UUIDs devem estar na versão 7.

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
