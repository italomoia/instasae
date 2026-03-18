# Plano de Ação — instasae

---

## Fase 0 — Documentação (antes de qualquer código)

### Passo 0.1 — Criar os 8 documentos em /docs
| Doc | Conteúdo | Status |
|---|---|---|
| 01_architecture.md | Decisões técnicas | ✅ Pronto |
| 02_database_model.md | Schema completo | ✅ Pronto |
| 03_folder_structure.md | Estrutura de pastas | ✅ Pronto |
| 04_api_routes.md | Endpoints | ✅ Pronto |
| 05_business_rules.md | Regras de negócio | ✅ Pronto |
| 06_local_development.md | Setup local | ✅ Pronto |
| 07_production_stack.md | Deploy | ✅ Pronto |
| 08_external_integrations.md | APIs externas | ✅ Pronto |

### Passo 0.2 — Criar CLAUDE.md
✅ Pronto

### Passo 0.3 — Criar .env.example
✅ Pronto

### Passo 0.4 — Criar prompt do assistente de desenvolvimento
✅ Pronto

---

## Fase 1 — Fundação do projeto (esqueleto sem funcionalidade)

### Passo 1.1 — Inicializar o projeto Go
Criar repo, `go mod init`, instalar dependências (chi, pgx, go-redis, aws-sdk-go-v2, golang-migrate, caarlos0/env). Configurar `.gitignore`.

### Passo 1.2 — Criar estrutura de pastas
Todos os diretórios e arquivos stub conforme `03_folder_structure.md`. Cada arquivo com package declaration e imports básicos.

### Passo 1.3 — Implementar config
`internal/config/config.go` — struct com todas as env vars, parsing via caarlos0/env, validação de campos obrigatórios.

### Passo 1.4 — Implementar server + routes
`internal/server/server.go` — HTTP server com graceful shutdown.
`internal/server/routes.go` — registrar rotas stub (retornando 501 Not Implemented).

### Passo 1.5 — Implementar main.go
Bootstrap: load config → connect PostgreSQL → connect Redis → create server → register routes → start.

### Passo 1.6 — Criar docker-compose.yml dev
PostgreSQL + Redis em portas deslocadas. Verificar que `docker compose up -d` + `go run ./cmd/instasae/` funciona.

### Passo 1.7 — Criar migrations SQL
3 arquivos up + 3 down conforme `02_database_model.md`. Rodar com golang-migrate.

### Passo 1.8 — Criar model structs
Todas as structs de domínio (Account, Contact, Conversation) e payloads externos (Instagram webhook, Chatwoot webhook/API).

### Passo 1.9 — Criar interfaces
Interfaces para Repository, Client, Cache. Permitir mocking nos testes.

**Ao final da Fase 1:** Projeto compila, servidor sobe, banco migrado, rotas retornam 501, estrutura completa mas vazia.

---

## Fase 2 — Testes primeiro (TDD — RED)

### Passo 2.1 — Testes do handler: webhook Instagram
- Webhook verification GET (token válido → challenge, inválido → 403)
- POST com assinatura válida → 200
- POST com assinatura inválida → descartado
- POST com payload mal-formado → 200 (não falhar para o Meta)

### Passo 2.2 — Testes do handler: webhook Chatwoot
- POST outgoing message → processado
- POST incoming message → ignorado
- POST private note → ignorado
- POST sem X-API-Key → 401

### Passo 2.3 — Testes do handler: admin accounts
- POST criar conta → 201
- POST conta duplicada (ig_page_id) → 409
- GET listar contas → 200
- PUT atualizar → 200
- DELETE (soft) → 200
- Sem X-API-Key → 401

### Passo 2.4 — Testes do service: Instagram inbound
- Mensagem de texto → cria contato + conversa + mensagem no Chatwoot
- Mensagem com imagem → download + upload B2 + mensagem com attachment
- Mensagem is_echo → ignorada
- Mensagem duplicada (mid no Redis) → ignorada
- Recipient ID desconhecido → ignorada com log
- Falha ao criar contato no Chatwoot → log error

### Passo 2.5 — Testes do service: Chatwoot outbound
- Texto simples → POST Graph API com text
- Attachment simples → POST Graph API com attachment
- Texto + attachment → 2 POSTs sequenciais (attachment primeiro)
- Janela expirada → não envia, nota privada
- Falha no Graph API → nota privada no Chatwoot

### Passo 2.6 — Testes do service: media
- Download + upload B2 → retorna URL pública
- Download falha → retorna erro
- Upload falha → retorna erro
- Arquivo >25MB → retorna erro

### Passo 2.7 — Testes do service: account
- Criar com dados válidos → OK
- Criar com ig_page_id duplicado → erro
- Tokens são encriptados antes do banco
- Tokens são decriptados ao ler do banco

### Passo 2.8 — Testes do repository
- CRUD accounts (com banco de teste)
- CRUD contacts
- CRUD conversations
- Queries com filtros (by ig_page_id, by ig_sender_id, active only)

### Passo 2.9 — Testes de cache
- SET NX dedup → primeira vez true, segunda false
- Cache account → hit e miss
- TTL funciona

### Passo 2.10 — Testes do crypto
- Encrypt → decrypt → valor original
- Key diferente → decrypt falha
- Valor encriptado é diferente do original

**Ao final da Fase 2:** Suíte completa de testes, todos falhando (RED). Fixtures prontas.

---

## Fase 3 — Implementação (fazer os testes passarem — GREEN)

### Passo 3.1 — crypto (encrypt/decrypt)
Implementar AES-256-GCM. Testes de crypto passam.

### Passo 3.2 — cache (Redis operations)
Implementar dedup e account cache. Testes de cache passam.

### Passo 3.3 — repository (database queries)
Implementar CRUD para as 3 tabelas. Testes de repo passam.

### Passo 3.4 — client: Instagram
Implementar send message, send attachment, get profile. Mocks nos testes.

### Passo 3.5 — client: Chatwoot
Implementar create contact, create conversation, create message, create private note. Mocks nos testes.

### Passo 3.6 — client: B2
Implementar upload via S3 SDK. Mocks nos testes.

### Passo 3.7 — service: media
Implementar download + upload + URL pública. Testes passam.

### Passo 3.8 — service: account
Implementar CRUD com encrypt/decrypt. Testes passam.

### Passo 3.9 — service: Instagram inbound
Implementar fluxo completo: parse → validate → dedup → route → contact → conversation → media → Chatwoot. Testes passam.

### Passo 3.10 — service: Chatwoot outbound
Implementar fluxo: parse callback → find account → find contact → check window → split → send → error handling. Testes passam.

### Passo 3.11 — middleware
Implementar signature validation, API key auth, logging.

### Passo 3.12 — handlers
Conectar handlers aos services. Testes de handler passam.

### Passo 3.13 — health check
Implementar /health com verificação de PostgreSQL e Redis.

**Ao final da Fase 3:** MVP funcional, todos os testes passando (GREEN). `go test ./...` = 100%.

---

## Fase 4 — Robustez (V1)

### Passo 4.1 — Busca de perfil do sender
Ao criar contato, buscar name, username, profile_pic via Instagram API.

### Passo 4.2 — Retry com backoff
Implementar retry (1s, 2s, 4s, max 3 tentativas) para chamadas a Instagram API e Chatwoot API.

### Passo 4.3 — Nota privada em caso de erro
Implementar envio de nota privada no Chatwoot quando envio ao Instagram falha.

### Passo 4.4 — Job de verificação de tokens
Goroutine com ticker (6h) que verifica tokens próximos de expirar.

### Passo 4.5 — Controle de janela 24h/7d
Implementar check de last_customer_message_at antes de enviar.

### Passo 4.6 — Métricas e health check avançado
Contadores de webhooks processados, mensagens enviadas, erros. Endpoint /health com detalhes.

---

## Fase 5 — Deploy e refinamentos (V1 + V2)

### Passo 5.1 — Dockerfile multi-stage
Build + runtime Alpine. Testar `docker build`.

### Passo 5.2 — Docker Compose produção
Com labels Traefik, networks existentes, env vars.

### Passo 5.3 — Primeiro deploy
Seguir docs/07_production_stack.md. Verificar health, webhook verification, fluxo completo.

### Passo 5.4 — Configurar Meta webhook
Apontar webhook URL para instasae. Verificar handshake. Subscrever a messages.

### Passo 5.5 — Criar primeira conta via admin API
Configurar Chatwoot inbox API, criar conta no instasae, testar fluxo ponta a ponta.

### Passo 5.6 — Refinamentos V2 (futuro)
- Renovação automática de tokens
- Story replies e story mentions
- Reactions
- Message delete sync
- Métricas Prometheus

---

## Resumo de estimativas

| Fase | Descrição | Status |
|---|---|---|
| Fase 0 | Documentação | ✅ Concluída |
| Fase 1 | Fundação (esqueleto) | ✅ Concluída |
| Fase 2 | Testes (RED) | ✅ Concluída |
| Fase 3 | Implementação (GREEN) | ✅ Concluída |
| Fase 4 | Robustez (V1) | ✅ Concluída |
| Fase 5 | Deploy + refinamentos | ✅ Concluída (V1 em produção) |
| **Total** | | **MVP operacional** |

Desenvolvimento realizado com Claude Code seguindo TDD. V1 em produção desde março/2026.
