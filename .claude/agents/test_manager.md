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
└───────── ...

## Como rodar

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
