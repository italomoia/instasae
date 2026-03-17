# Architecture — instasae

## About this document

This document records every technical decision made for the instasae project, with justifications tied to the project's real context. No decision is arbitrary. When something is chosen, the alternatives that were considered and discarded are documented with concrete reasons.

## What this project does

instasae is a bidirectional message bridge between Instagram Direct Messages and Chatwoot. It receives webhook events from the Instagram Messaging API when customers send DMs to an Instagram professional account, processes them, and forwards the messages to a Chatwoot API channel inbox. When agents reply through Chatwoot, instasae receives the callback and sends the response back to Instagram via the Graph API.

The system serves 10-50 Instagram accounts (agency model), running on a single VPS alongside existing infrastructure.

## Existing infrastructure

| Service | Details |
|---|---|
| VPS | Already running, IP: ${VPS_IP} |
| Reverse Proxy | Traefik (already running, handles TLS) |
| PostgreSQL | 16+ (already running, used by Chatwoot) |
| Redis | 7+ (already running, used by Chatwoot) |
| Chatwoot | Self-hosted (already running) |
| Backblaze B2 | Existing bucket connected to Chatwoot |
| Docker | Already installed, services managed via Docker Compose |

## Architecture decisions

### Decision 1 — Programming Language
**Choice: Go 1.23+**

Go produces a single binary with zero runtime dependencies. The Docker image is ~15MB with Alpine. Memory consumption is ~15-30MB for this workload vs ~100-200MB for Python/Node.js alternatives. The VPS already runs Traefik + PostgreSQL + Redis + Chatwoot, so every MB counts. Go's goroutines handle concurrent webhook processing natively without thread pools or async frameworks.

**Discarded alternatives:**
- **Python (FastAPI):** Requires Python runtime + pip + virtualenv. Higher memory footprint. Good ecosystem for AI/scraping but instasae doesn't need either — it's an HTTP proxy with routing logic.
- **Node.js (Express/Fastify):** Heavy runtime for what is essentially receive-HTTP → query-DB → send-HTTP. npm dependency management adds complexity without value for this use case.
- **Rust:** Performance superior to Go but development time significantly higher. instasae processes at most a few hundred messages per hour — Go's performance is already 100x more than needed.

### Decision 2 — Architecture Pattern
**Choice: Monolith — single binary**

instasae has exactly two data flows (message in, message out) and one admin API. No independent domains justify service separation. A single binary with organized internal packages is the correct choice for this scope.

**Discarded alternatives:**
- **Microservices:** Network overhead between services, 3+ containers to deploy, Docker networking complexity. For 10-50 accounts this is overengineering with zero benefit.

### Decision 3 — HTTP Router
**Choice: chi (go-chi/chi) v5**

Lightweight router (zero dependencies beyond stdlib), idiomatic Go, compatible with `net/http`. Supports route grouping (webhook, admin, health) and middleware chaining cleanly.

**Discarded alternatives:**
- **Gin:** Opinionated framework with its own `gin.Context` instead of `http.Request`. Adds dependency without adding value for this case.
- **stdlib only (net/http):** No URL parameter routing or route grouping. Code becomes verbose for grouped routes with shared middleware.
- **Fiber:** Based on fasthttp, incompatible with `net/http` ecosystem middlewares. No need for extreme performance.

### Decision 4 — Database
**Choice: PostgreSQL (existing instance, separate database)**

The instasae database stores ~3 tables with lightweight data (accounts, contacts, conversations). PostgreSQL already runs for Chatwoot. Creating a new database on the same instance has zero additional infrastructure cost. Concurrent webhook writes are handled properly by PostgreSQL's MVCC.

**Discarded alternatives:**
- **SQLite:** Functional for the volume but has write concurrency limitations. With 10-50 accounts receiving simultaneous webhooks, PostgreSQL is more reliable at no extra cost.
- **Config file:** Unmanageable for 10-50 accounts that change over time. Restarting the service to add/remove an account is operationally unacceptable.
- **Redis as primary store:** No relational queries, no constraints, no standard backup tooling. Redis is used for cache, not as source of truth.

### Decision 5 — PostgreSQL Driver
**Choice: pgx (jackc/pgx) v5**

Native PostgreSQL driver for Go. More performant than lib/pq (deprecated). Native connection pooling via `pgxpool`. Direct SQL queries — no ORM overhead for 3 tables.

**Discarded alternatives:**
- **GORM:** Heavy ORM for 3 tables. Unnecessary abstraction, harder debugging of generated queries.
- **sqlc:** Code generation from SQL adds tooling step. For ~10-15 queries total, manual pgx is simpler.

### Decision 6 — Cache / Deduplication
**Choice: Redis (existing instance)**

Two concrete uses: (1) Cache account lookups by ig_page_id (TTL 5min) to avoid DB query on every webhook. (2) Webhook deduplication via SET NX on message.mid (TTL 5min) to prevent processing retried webhooks twice.

### Decision 7 — Object Storage for Media
**Choice: Backblaze B2 (existing bucket)**

Instagram CDN URLs for media attachments are temporary. instasae downloads the media and uploads to B2, then passes the permanent B2 URL to Chatwoot. B2 is S3-compatible so aws-sdk-go-v2 works directly.

**Discarded alternatives:**
- **Pass-through temporary URL:** Media breaks after hours/days. Agents opening old conversations see broken images.
- **Local storage in container:** Ephemeral, lost on every deploy.
- **New S3 bucket:** Additional cost when B2 already exists and is S3-compatible.

### Decision 8 — Migrations
**Choice: golang-migrate v4**

Standard Go migration tool. SQL files, no proprietary DSL. Supports PostgreSQL. Can be run programmatically on startup or via CLI.

### Decision 9 — Configuration
**Choice: Environment variables (global) + database (per-account)**

Global config (DB DSN, Redis URL, port, Meta App Secret, B2 credentials, encryption key) via env vars. Per-account config (IG tokens, Chatwoot credentials, inbox IDs) in the accounts table, managed via admin API.

### Decision 10 — Logging
**Choice: slog (log/slog) from Go stdlib**

Structured logging built into Go since 1.21. JSON output for log parsing tools. Zero external dependencies.

### Decision 11 — Deployment
**Choice: Docker container with Traefik routing**

Multi-stage Docker build: build with `golang:1.23-alpine`, runtime with `alpine:3.19`. Traefik labels for routing and automatic TLS. Docker Compose alongside existing services.

### Decision 12 — Meta App Model
**Choice: Single Meta App for all clients**

One Meta App owned by the agency. Each client authorizes the app via OAuth to access their Instagram account. All webhooks arrive at the same endpoint and are routed by `entry[].id` (Instagram Page ID). This is the standard pattern for agencies and platforms.

**Discarded alternatives:**
- **Separate Meta App per client:** Each client manages their own app, different webhook URLs and App Secrets per client. Massive operational overhead for the agency.

## Domains and URLs

| Subdomain | Purpose |
|---|---|
| `${INSTASAE_DOMAIN}` | instasae API (webhook + admin) |
| `${CHATWOOT_DOMAIN}` | Chatwoot instance (existing) |

## Architecture overview

```
                    ┌──────────────┐
                    │  Instagram   │
                    │  Graph API   │
                    └──────┬───────┘
                           │
            webhook POST   │   POST /messages
            (DM received)  │   (send reply)
                           │
                    ┌──────▼───────┐
                    │   Traefik    │
                    │ (TLS + route)│
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   instasae   │
                    │  (Go binary) │
                    │              │
                    │ ┌──────────┐ │
                    │ │ Webhook  │ │◄── Instagram webhooks
                    │ │ Handler  │ │
                    │ └────┬─────┘ │
                    │      │       │
                    │ ┌────▼─────┐ │
                    │ │ Service  │ │── routing, validation
                    │ │  Layer   │ │   dedup, media handling
                    │ └────┬─────┘ │
                    │      │       │
                    │ ┌────▼─────┐ │
                    │ │ Chatwoot │ │──► callback receives
                    │ │ Handler  │ │    agent replies
                    │ └──────────┘ │
                    │              │
                    │ ┌──────────┐ │
                    │ │ Admin API│ │── CRUD accounts
                    │ └──────────┘ │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       ┌──────▼──┐  ┌──────▼──┐  ┌─────▼────┐
       │PostgreSQL│  │  Redis  │  │Backblaze │
       │(accounts │  │ (cache +│  │   B2     │
       │ contacts │  │  dedup) │  │ (media)  │
       │ convos)  │  │         │  │          │
       └─────────┘  └─────────┘  └──────────┘
              │
       ┌──────▼───────┐
       │   Chatwoot   │
       │ (self-hosted) │
       │  API channel  │
       │    inbox      │
       └──────────────┘
```

## Technology summary

| Layer | Technology | Version |
|---|---|---|
| Language | Go | 1.23+ |
| HTTP Router | chi | v5 |
| Database | PostgreSQL | 16+ |
| DB Driver | pgx | v5 |
| Cache | Redis | 7+ |
| Redis Client | go-redis | v9 |
| Object Storage | Backblaze B2 | S3-compat |
| S3 Client | aws-sdk-go-v2/s3 | v2 |
| Migrations | golang-migrate | v4 |
| Logging | slog (stdlib) | — |
| Config | caarlos0/env | v11 |
| HTTP Client | net/http (stdlib) | — |
| Encryption | crypto/aes (stdlib) | — |
| Reverse Proxy | Traefik | existing |
| Container | Docker + Alpine | — |

## Resource limits

| Service | CPU | Memory | Notes |
|---|---|---|---|
| instasae | 0.5 cores | 64MB (limit 128MB) | Lightweight Go binary |
| PostgreSQL | shared | shared | Existing instance, new database |
| Redis | shared | shared | Existing instance |
| Backblaze B2 | — | — | External service |

## Important notes for development

1. **PostgreSQL is shared** with Chatwoot. Use a separate database (`instasae`) to avoid any table collision. Connection pool max should be conservative (10 connections).
2. **Redis is shared** with Chatwoot. Use a key prefix (`instasae:`) for all keys to avoid collision.
3. **Traefik is shared.** instasae gets its own subdomain via Docker labels.
4. **Backblaze B2 bucket is shared** with Chatwoot. Use a dedicated folder/prefix (`instasae/`) for uploaded media.
5. **Meta webhook requires HTTPS.** Traefik handles TLS automatically via Let's Encrypt.
6. **All tokens in the database are encrypted** with AES-256-GCM. The encryption key is an env var, never in the database.
7. **The Meta App Secret is a single global env var** since all accounts share the same Meta App.
