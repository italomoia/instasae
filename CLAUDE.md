# CLAUDE.md — instasae

## What this project is

instasae is a bidirectional message bridge between Instagram Direct Messages and Chatwoot. It receives DM webhooks from Instagram, routes them to Chatwoot API channel inboxes, and sends agent replies back to Instagram via Graph API. Serves 10-50 Instagram accounts on a single Go binary.

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.23+ |
| HTTP Router | chi v5 |
| Database | PostgreSQL 16 (existing, separate DB) |
| DB Driver | pgx v5 (direct SQL, no ORM) |
| Cache | Redis 7 (existing, key prefix instasae:) |
| Object Storage | Backblaze B2 (S3-compatible, existing bucket) |
| Migrations | golang-migrate v4 |
| Logging | slog (stdlib) |
| Config | caarlos0/env v11 |
| Reverse Proxy | Traefik (existing) |
| Container | Docker + Alpine (~15MB image) |

## Folder structure (summary)

```
instasae/
├── cmd/instasae/main.go         # Entrypoint
├── internal/
│   ├── config/                  # Env var parsing
│   ├── server/                  # HTTP server + route registration
│   ├── handler/                 # HTTP handlers (thin)
│   ├── middleware/               # Signature validation, auth, logging
│   ├── service/                 # Business logic (core)
│   ├── client/                  # HTTP clients (Instagram, Chatwoot, B2)
│   ├── repository/              # Database queries (pgx)
│   ├── cache/                   # Redis operations
│   ├── crypto/                  # AES encrypt/decrypt
│   └── model/                   # Structs (domain + payloads)
├── migrations/                  # SQL files
├── tests/                       # Tests + fixtures
└── docs/                        # 8 foundation docs
```

## Database tables (summary)

| Table | Purpose |
|---|---|
| accounts | Maps IG page → Chatwoot inbox. Stores encrypted tokens. |
| contacts | Maps IG sender → Chatwoot contact. Caches profile info. |
| conversations | Maps active conversation. Tracks messaging window. |

## Environment variables

```
PORT                    # HTTP server port (8080)
LOG_LEVEL               # debug/info/warn/error
DATABASE_URL            # PostgreSQL connection string
REDIS_URL               # Redis connection string
META_APP_SECRET         # HMAC key for webhook signature validation
META_GRAPH_API_VERSION  # v25.0
B2_ENDPOINT             # S3-compatible endpoint
B2_REGION               # B2 region
B2_BUCKET               # Bucket name
B2_KEY_ID               # B2 key
B2_APPLICATION_KEY      # B2 secret
B2_PUBLIC_URL           # Public URL base
B2_PREFIX               # instasae (folder prefix)
ENCRYPTION_KEY          # AES-256 key (64 hex chars)
ADMIN_API_KEY           # Auth for admin endpoints
WEBHOOK_VERIFY_TOKEN    # Meta webhook verification
```

## How to run locally

```bash
docker compose up -d                    # Start dev DB + Redis
cp .env.example .env                    # Configure
go mod download                         # Install deps
migrate -database "$DATABASE_URL" -path migrations up
go run ./cmd/instasae/                  # Start server on :8080
```

## How to run tests

```bash
go test ./...                           # All tests
go test -v ./tests/service/...          # Service tests only
go test -cover ./...                    # With coverage
```

## Coding conventions

- Handlers are thin: decode request → call service → encode response
- Business logic lives in service/ only
- Database access lives in repository/ only — no SQL outside repo files
- External API calls live in client/ only
- All tokens in DB are encrypted via crypto/ package
- Redis keys always prefixed with `instasae:`
- B2 paths always prefixed with `instasae/`
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Interfaces for testability: repos, clients, cache all have interfaces
- Tests use fixtures from tests/fixtures/ — real payloads with redacted data

## TDD workflow

```
RED    → write test describing expected behavior → go test → FAIL
GREEN  → write minimum code to pass → go test → PASS
REFACTOR → improve code, tests still pass → go test → PASS
COMMIT → git commit -m "descriptive message"
```

## Main flows

```
INBOUND: Instagram webhook POST → validate signature → check is_echo →
         deduplicate (Redis) → find account → find/create contact →
         find/create conversation → download media → upload to B2 →
         send to Chatwoot API

OUTBOUND: Chatwoot callback POST → filter outgoing non-private →
          find account → find contact → check window →
          split if text+attachment → send to Instagram Graph API →
          on error: private note in Chatwoot
```

## Detailed docs

| Doc | Topic |
|---|---|
| docs/01_architecture.md | All technical decisions with justifications |
| docs/02_database_model.md | Tables, fields, types, constraints, indexes |
| docs/03_folder_structure.md | Every file and folder explained |
| docs/04_api_routes.md | All endpoints with payloads |
| docs/05_business_rules.md | All server-enforced rules (BR-XXX-NN) |
| docs/06_local_development.md | How to run locally |
| docs/07_production_stack.md | Dockerfile, deploy, update process |
| docs/08_external_integrations.md | Instagram API, Chatwoot API, B2 |

## Common hurdles

*(filled during development)*

## Design patterns

*(filled during development)*

## Post-implementation checklist

```
[ ] All tests pass: go test ./...
[ ] Health check returns OK in production
[ ] Meta webhook verification succeeds
[ ] Inbound text message flows IG → Chatwoot
[ ] Inbound image message flows with B2 upload
[ ] Outbound text message flows Chatwoot → IG
[ ] Outbound image message flows Chatwoot → IG
[ ] Composite message (text+image) splits correctly
[ ] is_echo messages are ignored
[ ] Duplicate webhooks are deduplicated
[ ] Unknown accounts are silently skipped
[ ] Token encryption/decryption works
[ ] Admin API CRUD works with API key auth
[ ] Private note appears on send failure
[ ] No real credentials in any committed file
```
