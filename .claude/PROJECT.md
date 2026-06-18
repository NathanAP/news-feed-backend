# Projeto: API de feed de notícias personalizado

## Foco deste projeto

- Backend + API Rest

## Ideia geral

Um feed de notícias hiper personalizado que coleta, filtra, traduz e resume notícias baseado nas preferências do usuário. O sistema:

- Gerencia usuários com autenticação Google.
- Permite cadastro de categorias temáticas (Metallica, Anime, Tech, etc).
- Descobre automaticamente feeds RSS das fontes informadas.
- Filtra notícias por palavras-chave + IA (Gemini 2.5 Flash).
- Traduz notícias através da IA (Gemini 2.5 Flash).
- Personaliza notícias através da IA (Gemini 2.5 Flash).
- Entrega notícia personalizada.

## Features

- **Descoberta automática de RSS**: Detecta feeds sem input manual, apenas através da URL principal
- **Filtragem dual-layer**: Palavras-chave rápidas + IA inteligente
- **Tradução de notícias**: Traduz notícias de acordo com as preferências do usuário
- **Personalização de notícias**: Personaliza notícias de acordo com as preferências do usuário
- **Endpoints REST**: API simples e intuitiva
- **Banco SQLite integrado**: Zero dependencies de infraestrutura

## Público alvo

Usuários que buscam personalizar seus feeds de notícias do seu jeito.

# Como funciona

## Fluxo principal

0. Usuário se cadastra através da sua conta Google.
1. Usuário cria categoria "Metallica" e uma fonte de notícias.
2. Sistema descobre automaticamente RSS de fontes relacionadas informadas pelo usuário.
3. Novas notícias são descobertas automaticamente.
    - Um processo percorre cada fonte cadastrada em busca de notícias recentes dela.
4. Identifica-se a qual usuário aquela notícia pertence.
    - Notícias sobre a categoria cadastrada aparecem em tempo real de forma personalizada àquele usuário.
    - Usuários com a mesma categoria e fonte cadastrada acabam recebendo diferentes versões por conta da hiper personalização ().
5. Verifica-se as prefêrencias do usuário que receberá a notícia e a personaliza.
6. Usuário vê a notícia e ela é marcada como lida.

## Fluxo de login

- Usuários são cadastrados exclusivamente pelo Google.
- Fluxo do login: frontend redireciona o usuário para `/v1/auth/google` → Google autentica → Google redireciona para o nosso callback (`/v1/auth/google/callback`) → nosso callback retorna um JSON com o JWT → frontend captura esse token e passa a usar como header (`Authorization: Bearer <token>`) nas próximas chamadas.
