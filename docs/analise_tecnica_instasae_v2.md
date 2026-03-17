# Análise Técnica Completa — instasae

**Autor:** Análise como Analista Sênior, Product Owner e Tech Lead
**Data:** 2026-03-16 — v2

**Changelog v2:**
- PROBLEMA 4 atualizado: usar Backblaze B2 existente para persistir mídia (aprovado pelo cliente)
- PROBLEMA 9 adicionado: mensagens compostas (texto + anexo) precisam ser splitadas em 2 mensagens sequenciais no Instagram
- PROBLEMA 7 confirmado: uma única Meta App para todos os clientes (aprovado)
- Todas as melhorias aprovadas sem modificações
- Todas as mitigações de risco aprovadas sem modificações

---

## 1. Parecer sobre as decisões técnicas

### 1.1 Linguagem: Go
**Decisão: Go** — Aprovado v1, sem alterações.

Binário único, ~15MB de imagem Docker, consumo mínimo de RAM na VPS que já roda Traefik + PostgreSQL + Redis + Chatwoot. Concorrência nativa para processar webhooks em paralelo.

### 1.2 Arquitetura: Monolito
**Decisão: Monolito — binário único** — Aprovado v1, sem alterações.

### 1.3 Framework HTTP: chi router
**Decisão: chi (go-chi/chi) v5** — Aprovado v1, sem alterações.

### 1.4 Banco de dados: PostgreSQL (existente)
**Decisão: PostgreSQL existente, schema separado** — Aprovado v1, sem alterações.

### 1.5 Driver PostgreSQL: pgx
**Decisão: pgx (jackc/pgx) v5** — Aprovado v1, sem alterações.

### 1.6 Cache e deduplicação: Redis (existente)
**Decisão: Redis existente** — Aprovado v1, sem alterações.

### 1.7 Migrations: golang-migrate
**Decisão: golang-migrate v4** — Aprovado v1, sem alterações.

### 1.8 Configuração: env vars + banco
**Decisão: env vars globais, config por conta no banco** — Aprovado v1, sem alterações.

### 1.9 Logging: slog (stdlib)
**Decisão: slog** — Aprovado v1, sem alterações.

### 1.10 Deploy: Docker com Traefik
**Decisão: Container Docker único, Traefik** — Aprovado v1, sem alterações.

### 1.11 Storage de mídia: Backblaze B2 (existente)
**Decisão: Backblaze B2 — bucket existente conectado ao Chatwoot**

Quando o instasae recebe mídia do Instagram (imagem, áudio, vídeo), a URL do CDN do Meta é temporária. O instasae vai baixar a mídia, enviar para o B2, e repassar a URL pública do B2 para o Chatwoot. Isso garante que a mídia persiste indefinidamente.

Para o fluxo inverso (Chatwoot → Instagram), se o agente anexar mídia no Chatwoot, a URL já é do B2/Chatwoot e é permanente — pode ser enviada direto para a Graph API.

**Alternativas descartadas:**
- **Repassar URL temporária do Meta:** Mídia fica indisponível depois de horas/dias. Agente abre conversa antiga e imagem quebrada.
- **Storage local no container:** Efêmero, perdido a cada deploy.
- **S3 novo:** Custo adicional e configuração quando B2 já existe e é S3-compatible.

---

## 2. Análise funcional completa

### 2.1 Fluxo validado

```
FLUXO DE ENTRADA (Instagram → Chatwoot)
=========================================

   Usuário envia DM no Instagram
              │
              ▼
   Meta envia POST webhook
   para instasae (HTTPS via Traefik)
              │
              ▼
   ┌─────────────────────────┐
   │  1. Responde 200 ao     │ ← IMEDIATO
   │     Meta                │
   │  2. Processa em         │
   │     goroutine async     │
   └─────────┬───────────────┘
              │
              ▼
   ┌─────────────────────────┐
   │  Validar X-Hub-Signature│
   │  -256 com App Secret    │
   └─────────┬───────────────┘
              │ (inválido → log + descarta)
              ▼
   ┌─────────────────────────┐
   │  Parsear payload:       │
   │  - object == "instagram"│
   │  - entry[].messaging[]  │
   │  - sender.id            │
   │  - recipient.id         │
   │  - message (text/media) │
   └─────────┬───────────────┘
              │
              ▼
   ┌─────────────────────────┐
   │  Verificar is_echo      │──→ true → IGNORAR
   └─────────┬───────────────┘
              │ false
              ▼
   ┌─────────────────────────┐
   │  Deduplicar via Redis   │──→ mid já existe → IGNORAR
   │  SET NX mid TTL=300     │
   └─────────┬───────────────┘
              │ novo
              ▼
   ┌─────────────────────────┐
   │  Buscar account:        │
   │  Redis cache →          │
   │  ou PostgreSQL           │
   │  WHERE ig_page_id =     │
   │  recipient.id           │
   └─────────┬───────────────┘
              │ (não encontrado → log + descarta)
              ▼
   ┌─────────────────────────┐
   │  Buscar ou criar        │
   │  contato no Chatwoot    │
   │  + buscar perfil IG     │
   └─────────┬───────────────┘
              │
              ▼
   ┌─────────────────────────┐
   │  Buscar ou criar        │
   │  conversação Chatwoot   │
   └─────────┬───────────────┘
              │
              ▼
   ┌─────────────────────────┐
   │  TEM MÍDIA?             │
   │  ┌─ SIM ──────────────┐ │
   │  │ 1. Download do CDN  │ │
   │  │ 2. Upload para B2   │ │
   │  │ 3. URL pública B2   │ │
   │  └────────────────────┘  │
   └─────────┬───────────────┘
              │
              ▼
   ┌─────────────────────────┐
   │  Enviar ao Chatwoot:    │
   │  POST /conversations/   │
   │  {id}/messages           │
   │  - text → content        │
   │  - media → attachment    │
   └─────────────────────────┘


FLUXO DE SAÍDA (Chatwoot → Instagram)
=========================================

   Agente responde no Chatwoot
              │
              ▼
   Chatwoot envia POST callback
              │
              ▼
   ┌─────────────────────────┐
   │  Filtrar:               │
   │  - event == message_    │
   │    created              │
   │  - message_type ==      │
   │    outgoing             │
   │  - private == false     │
   └─────────┬───────────────┘
              │
              ▼
   ┌─────────────────────────┐
   │  Buscar account +       │
   │  contact + ig_sender_id │
   └─────────┬───────────────┘
              │
              ▼
   ┌────────────────────────────────┐
   │  TEM ATTACHMENT + TEXT?        │
   │                                │
   │  Caso 1: Só texto             │
   │  → 1 chamada: text            │
   │                                │
   │  Caso 2: Só attachment        │
   │  → 1 chamada: attachment      │
   │                                │
   │  Caso 3: Texto + attachment   │
   │  → 2 chamadas sequenciais:    │
   │    1º attachment               │
   │    2º text                     │
   │  (Instagram não suporta       │
   │   texto + mídia na mesma msg) │
   └────────────┬───────────────────┘
                │
                ▼
   ┌─────────────────────────┐
   │  POST Graph API         │
   │  /{ig_page_id}/messages │
   │  + HUMAN_AGENT tag      │
   │  + Retry com backoff    │
   └─────────────────────────┘
   │  (erro → nota privada   │
   │   no Chatwoot)          │
   └─────────────────────────┘
```

### 2.2 Problemas e lacunas identificados

**PROBLEMA 1 — Payload do webhook diferente sem n8n** — Aprovado v1

**PROBLEMA 2 — Janela de resposta 24h/7d** — Aprovado v1

**PROBLEMA 3 — Tokens expiram em 60 dias** — Aprovado v1

**PROBLEMA 4 — URLs de mídia do Instagram são temporárias** — Atualizado v2
**Recomendação atualizada:** Baixar mídia do CDN temporário do Meta, fazer upload para Backblaze B2 (bucket existente do Chatwoot), e repassar URL pública do B2 para o Chatwoot. Isso é feito no MVP, não mais em V2.

**PROBLEMA 5 — Webhook verification do Meta** — Aprovado v1

**PROBLEMA 6 — Validação de assinatura** — Aprovado v1

**PROBLEMA 7 — Uma Meta App para múltiplas contas** — Aprovado v1 (confirmado: uma única Meta App)

**PROBLEMA 8 — Mapeamento contact_id Chatwoot → IGSID** — Aprovado v1

**PROBLEMA 9 — Mensagens compostas (texto + anexo) no Instagram** — Novo v2

O Instagram não suporta enviar texto e attachment na mesma chamada à API. Se o agente no Chatwoot escreve uma mensagem com texto + imagem, o callback do Chatwoot traz ambos. O instasae precisa:
1. Enviar o attachment primeiro (POST com `message.attachment`)
2. Enviar o texto em seguida (POST com `message.text`)

Ordem: mídia primeiro, texto depois — para que o cliente veja o contexto visual antes da legenda.

Se qualquer uma das duas chamadas falhar, logar qual falhou e enviar nota privada no Chatwoot.

### 2.3 Melhorias aprovadas

Todas as 7 melhorias da v1 aprovadas sem alteração:
1. Buscar perfil do sender no Instagram
2. Processamento assíncrono com goroutine
3. Health check + métricas
4. Retry com backoff
5. Nota privada no Chatwoot em caso de erro
6. Encriptação de tokens no banco
7. Graceful shutdown

---

## 3. Arquitetura técnica proposta

### 3.1 Stack definida (atualizada v2)

| Componente | Tecnologia | Versão |
|---|---|---|
| Linguagem | Go | 1.23+ |
| Router HTTP | chi (go-chi/chi) | v5 |
| Banco de dados | PostgreSQL | 16+ (existente) |
| Driver PostgreSQL | pgx (jackc/pgx) | v5 |
| Cache/Dedup | Redis | 7+ (existente) |
| Client Redis | go-redis (redis/go-redis) | v9 |
| Object Storage | Backblaze B2 | existente (S3-compatible) |
| Client B2/S3 | aws-sdk-go-v2/s3 | v2 |
| Migrations | golang-migrate | v4 |
| Logging | slog (stdlib) | — |
| Config | env vars (caarlos0/env) | v11 |
| HTTP client | net/http (stdlib) | — |
| Crypto (tokens) | crypto/aes (stdlib) | — |
| Reverse proxy | Traefik | existente |
| Container | Docker (alpine) | — |

### 3.2 Estrutura do projeto

```
instasae/
├── cmd/
│   └── instasae/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── server/
│   │   ├── server.go
│   │   └── routes.go
│   ├── handler/
│   │   ├── webhook_instagram.go
│   │   ├── webhook_chatwoot.go
│   │   ├── admin_accounts.go
│   │   └── health.go
│   ├── middleware/
│   │   ├── signature.go
│   │   ├── auth.go
│   │   └── logging.go
│   ├── service/
│   │   ├── instagram.go
│   │   ├── chatwoot.go
│   │   ├── media.go
│   │   └── account.go
│   ├── client/
│   │   ├── instagram_client.go
│   │   ├── chatwoot_client.go
│   │   └── b2_client.go
│   ├── repository/
│   │   ├── account_repo.go
│   │   ├── contact_repo.go
│   │   └── conversation_repo.go
│   ├── cache/
│   │   └── redis.go
│   ├── crypto/
│   │   └── encrypt.go
│   └── model/
│       ├── account.go
│       ├── contact.go
│       ├── conversation.go
│       ├── instagram.go
│       └── chatwoot.go
├── migrations/
│   ├── 001_create_accounts.up.sql
│   ├── 001_create_accounts.down.sql
│   ├── 002_create_contacts.up.sql
│   ├── 002_create_contacts.down.sql
│   ├── 003_create_conversations.up.sql
│   └── 003_create_conversations.down.sql
├── tests/
│   ├── handler/
│   │   ├── webhook_instagram_test.go
│   │   ├── webhook_chatwoot_test.go
│   │   └── admin_accounts_test.go
│   ├── service/
│   │   ├── instagram_test.go
│   │   ├── chatwoot_test.go
│   │   ├── media_test.go
│   │   └── account_test.go
│   ├── repository/
│   │   ├── account_repo_test.go
│   │   ├── contact_repo_test.go
│   │   └── conversation_repo_test.go
│   ├── fixtures/
│   │   ├── instagram_text_webhook.json
│   │   ├── instagram_image_webhook.json
│   │   ├── instagram_audio_webhook.json
│   │   ├── instagram_echo_webhook.json
│   │   ├── chatwoot_outgoing_text.json
│   │   ├── chatwoot_outgoing_attachment.json
│   │   ├── chatwoot_outgoing_text_with_attachment.json
│   │   ├── chatwoot_incoming_ignored.json
│   │   └── chatwoot_private_note.json
│   └── testutil/
│       └── helpers.go
├── docker-compose.yml
├── Dockerfile
├── .env.example
├── go.mod
├── go.sum
├── CLAUDE.md
└── docs/
    ├── 01_architecture.md
    ├── 02_database_model.md
    ├── 03_folder_structure.md
    ├── 04_api_routes.md
    ├── 05_business_rules.md
    ├── 06_local_development.md
    ├── 07_production_stack.md
    └── 08_external_integrations.md
```

### 3.3 Modelos do banco

| Tabela | Campos principais | Propósito |
|---|---|---|
| `accounts` | ig_page_id, ig_access_token (encrypted), chatwoot_account_id, chatwoot_inbox_id, chatwoot_api_token (encrypted), chatwoot_base_url, webhook_verify_token, token_expires_at, is_active | Conta Instagram ↔ inbox Chatwoot |
| `contacts` | account_id (FK), ig_sender_id, chatwoot_contact_id, chatwoot_contact_source_id, ig_username, ig_name | Usuário IG ↔ contato Chatwoot |
| `conversations` | account_id (FK), contact_id (FK), chatwoot_conversation_id, last_customer_message_at, is_active | Conversa ativa, controle de janela |

---

## 4. Riscos — Aprovados v1 sem alterações

1. Meta pode mudar formato do webhook
2. Token expirado = conta silenciosamente morta
3. Rate limit Graph API (200/hora/conta)
4. Perda de mensagem entre receber e enviar
5. Callback URL do Chatwoot desatualizada

---

## 5. Prioridades de implementação

### MVP (Fases 1-3, ~2 semanas)
1. Estrutura + banco + migrations
2. Webhook verification GET
3. Handler webhook com validação assinatura
4. Mensagens de texto (entrada e saída)
5. Mídia (imagem, áudio, vídeo) com download + upload B2
6. Split de mensagens compostas (texto + attachment → 2 chamadas)
7. API admin CRUD de accounts
8. Filtro is_echo + deduplicação Redis
9. Mapeamento contatos e conversas
10. Deploy Docker + Traefik

### V1 (Fase 4, semana 3)
1. Busca perfil sender (nome, username, avatar)
2. Retry com backoff
3. Nota privada no Chatwoot em caso de erro
4. Job verificação tokens
5. Health check + métricas
6. Encriptação tokens no banco
7. Controle janela 24h/7d com HUMAN_AGENT tag

### V2 (Fase 5, semana 4+)
1. Renovação automática de tokens
2. Story replies e story mentions
3. Reactions
4. Message delete sync
5. Métricas Prometheus
6. Read/delivery status
