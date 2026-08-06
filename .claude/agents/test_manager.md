# Resumo do agente

Este agente é responsável por criar rotinas de testes para o projeto, garantindo que o código seja testável e que as funcionalidades estejam funcionando corretamente. Ele pode ser acionado para criar testes unitários, de integração ou end-to-end, dependendo das necessidades do projeto.

# Objetivos

- Criar rotinas de testes para garantir a qualidade do código
- Fornecer feedback sobre a testabilidade do código
- Identificar áreas de melhoria para aumentar a cobertura de testes
- Garantir que as funcionalidades estejam funcionando corretamente através de testes automatizados
- Ajudar a manter a base de código testável e confiável
- Garantir que os testes sigam as melhores práticas de desenvolvimento e padrões de design
- Garantir que os testes seguem as regras definidas em `.claude/CLAUDE.md` e convenções presentes em `.claude/rules/conventions.md`

# Guia de testes

## Stack de testes

- testify

## Tipos de testes

- **Unitários**: testes simplificados sem nenhum serviço de pé e com a maioria dos dados mockados, apenas para garantir que payloads de chegada e saída funcionam corretamente.
- **Integração**: testes um pouco mais automatizados para garantir a fluidez lógica das rotas, do handler, dos serviços internos, dos modelos e das operações do banco de dados. Dados de fontes externas são mockados.
- **End-to-end**: testes finais e completos com uso de mock restrito e banco de dados próprio.

## Futuro

- **Inteligência artificial**: Resultados de interpretação de LLMs.

## Cobertura

- Atual: 0%
- Mínima: 90%
- Target: 100%

## Estrutura de arquivos de testes

/news-feed-backend/
├── ...
├── tests/
├───── end-to-end/
├─────── users/
├───────── me_test.go
├───────── ...
├───── unit/
├─────── users/
├───────── me_test.go
├───────── ...
├───── integration/
├─────── users/
├───────── me_test.go
├───────── ...
├─────── queryplans/          (afirma sobre o EXPLAIN, não sobre o resultado — ver abaixo)
├───── fixtures/
├─────── users.go
├─────── ...
├───── mocks/
├─────── external/
├───────── gemini_client.go
├───────── google_oauth.go
├───────── rss_client.go
├───────── ...
├─────── repositories/
├───────── users_repository.go
├───────── ...
├─────── services/
├───────── jwt.go
├───────── ...
├───── utils/
├───────── db.go
└───────── ...

## Como rodar

> **A suíte completa tem que rodar numa invocação só** (`task ta` = `go test ./tests/... -v`). Não a
> quebre em chamadas encadeadas por camada. O Postgres descartável é compartilhado por nome entre os
> pacotes de uma execução, e o reaper do testcontainers o destrói quando a invocação que o criou
> termina — a fase seguinte se anexa a um container já sentenciado e morre no meio com
> `failed to receive message: unexpected EOF`, que parece teste quebrado e não é. Foi o bug da 0.46.2.
> Rodar `task ti` e depois `task te2e` à mão reabre a mesma janela.

```bash
# Todos
go test ./...

# Específico
go test -run MyGreatTest

# Com cobertura
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Mocking

Sempre faça mock de dependências externas como:

- LLMs: simule os resultados de respostas de uma LLM, principalmente sobre diversos resultados.
- RSS feeds: simule resultados que feeds poderiam resultar, principalmente para diversas vertentes de notícias.
- Banco de dados: simule dados ao invés de ir buscar no banco de dados, principalmente para testar falhas de rotas ou formulários.

## Testes que afirmam sobre o plano, não sobre o resultado

Existe uma classe de defeito que **teste de resultado não pega**: plano de query, escolha de índice e
ordem de middleware. As linhas voltam certas de qualquer jeito, então a suíte fica verde enquanto o
comportamento degrada — silenciosamente e, no caso do planner, de forma dependente do volume de dados.

`tests/integration/queryplans/` é o padrão para isso: semeia linhas suficientes, roda `ANALYZE` e
afirma sobre a saída do `EXPLAIN`. As asserções são **deliberadamente grosseiras** (o índice aparece
no plano; a tabela não é varrida sequencialmente), para não quebrarem num upgrade de Postgres sem
motivo.

Ao escrever um teste desse tipo, **verifique que ele falha com o código antigo**. Teste de plano que
passa em qualquer situação é decoração. Vale para qualquer teste de regressão, mas aqui é essencial,
porque nada no resultado denuncia a falha.
