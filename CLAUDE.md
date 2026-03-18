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
| docs/09_operations_guide.md | Account onboarding, monitoring, token renewal |

## Common hurdles

- **Port conflicts:** Dev ports 5433/6380 from original docs conflict with followup/jurispost projects on this machine. Actual dev ports: PostgreSQL=5435, Redis=6382. Always check docker-compose.yml for current ports.
- **Meta App secrets:** The app has TWO secrets. `META_APP_SECRET` (for webhook signature validation) is the Facebook App Secret from "App Settings > Basic" in Meta Developer Dashboard. The Instagram App Secret (shown on the Instagram product page) is used for OAuth token exchange (V2 feature, not MVP). Do NOT confuse them.
- **OAuth flow is V2:** Client onboarding via OAuth (routes `/connect`, `/oauth/callback`, `/oauth/success`) is not part of the MVP. For MVP, tokens are manually inserted via admin API. The Instagram App ID and Instagram App Secret are needed only for V2.
- **Docker Swarm:** VPS uses Docker Swarm, not plain docker-compose. Network is `IMSNet`, Traefik certresolver is `letsencryptresolver`. Deploy with `docker stack deploy`, update with `docker service update`.
- **Redis DB indexes:** 0=default, 2=n8n, 8=evolution. instasae uses index 3.
- **GHCR images:** Docker images are pushed to `ghcr.io/italomoia/instasae:latest`, not built locally on the VPS.
- **Instagram App Secret for webhooks:** When using "Instagram API with Business Login", the webhook signature uses the Instagram App Secret (from the Instagram product page), NOT the Facebook App Secret (from App Settings > Basic). The Facebook App Secret caused invalid webhook signature on every webhook in production.
- **Cache serialization and json:"-" tags:** Account model has `json:"-"` on token fields to hide them from API responses. The Redis cache was using `json.Marshal`, which stripped tokens. Fixed by switching to `encoding/gob` which ignores JSON tags.
- **message_edit events:** Instagram sends `message_edit` events (not `message`) when the account owner sends DMs from the Instagram app itself. These have no sender/recipient/message fields. The code correctly skips them (`msg == nil`), but they can be confusing in logs.
- **Swarm network not attachable:** Docker Swarm overlay networks don't allow `docker run --network`. Migrations must be run via `docker exec` on the postgres container or by making the network attachable.
- **Webhook subscription can drop:** Instagram page webhook subscriptions can silently deactivate. Must re-subscribe via `POST /{page_id}/subscribed_apps`. V2 should add periodic subscription health check.
- **HUMAN_AGENT tag requires App Review:** The tag is disabled by default (`ENABLE_HUMAN_AGENT_TAG=false`). Without it, replies work within 24h window only. Enable after Meta approves the permission.

## Design patterns

- **gob for internal cache, json for API responses:** Account tokens use `json:"-"` to hide from HTTP responses, but `gob` encoding preserves all exported fields for Redis cache.
- **Multipart form data for Chatwoot attachments:** Chatwoot Application API requires `multipart/form-data` with `attachments[]` field for inline media display. JSON API only supports text content.

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
