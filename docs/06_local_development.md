# Local Development — instasae

## About this document

Step-by-step guide to run instasae locally. Anyone (or any AI agent) should be able to get the project running by following only this document.

## Prerequisites

```bash
# Go 1.23+
go version   # must show 1.23 or higher

# Docker + Docker Compose
docker --version
docker compose version

# golang-migrate CLI (for running migrations manually)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Optional: ngrok or similar for testing Meta webhooks locally
# ngrok http 8080
```

## docker-compose.yml (development)

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: instasae
      POSTGRES_PASSWORD: instasae_dev
      POSTGRES_DB: instasae
    ports:
      - "5433:5432"    # Port 5433 to avoid conflict with host PostgreSQL
    volumes:
      - instasae_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U instasae"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6380:6379"    # Port 6380 to avoid conflict with host Redis
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  instasae_pgdata:
```

Notes:
- Ports are offset (5433, 6380) to avoid conflicts with PostgreSQL and Redis already running on the host for Chatwoot.
- In production, instasae connects to the existing PostgreSQL and Redis instances — no separate containers.

## Environment variables

Copy `.env.example` to `.env` and fill in:

```bash
# Server
PORT=8080
LOG_LEVEL=debug    # debug, info, warn, error

# PostgreSQL
DATABASE_URL=postgres://instasae:instasae_dev@localhost:5433/instasae?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6380/0

# Meta (Instagram)
META_APP_SECRET=your_meta_app_secret_here
META_GRAPH_API_VERSION=v25.0

# Backblaze B2 (S3-compatible)
B2_ENDPOINT=https://s3.us-west-004.backblazeb2.com
B2_REGION=us-west-004
B2_BUCKET=your-bucket-name
B2_KEY_ID=your_key_id
B2_APPLICATION_KEY=your_application_key
B2_PUBLIC_URL=https://f004.backblazeb2.com/file/your-bucket-name
B2_PREFIX=instasae

# Encryption (32 bytes hex for AES-256)
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

# Admin API
ADMIN_API_KEY=your_admin_api_key_here

# Global webhook verify token (fallback if account-specific not set)
WEBHOOK_VERIFY_TOKEN=your_verify_token_here
```

## How to start

```bash
# 1. Clone the repository
git clone ${REPO_URL} instasae
cd instasae

# 2. Start development databases
docker compose up -d
# Wait for health checks:
docker compose ps   # both should show "healthy"

# 3. Copy environment file
cp .env.example .env
# Edit .env with your values

# 4. Install Go dependencies
go mod download

# 5. Run database migrations
migrate -database "${DATABASE_URL}" -path migrations up

# 6. Start the application
go run ./cmd/instasae/
# Output: "server started on :8080"
```

## Verifying everything works

```bash
# Health check
curl http://localhost:8080/health
# Expected: {"status":"ok","postgres":"connected","redis":"connected",...}

# Create a test account (requires ADMIN_API_KEY from .env)
curl -X POST http://localhost:8080/api/accounts \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_admin_api_key_here" \
  -d '{
    "ig_page_id": "test123",
    "ig_page_name": "Test Account",
    "ig_access_token": "test_token",
    "chatwoot_base_url": "https://chat.example.com",
    "chatwoot_account_id": 1,
    "chatwoot_inbox_id": 1,
    "chatwoot_api_token": "test_chatwoot_token",
    "webhook_verify_token": "test_verify"
  }'
# Expected: 201 with account JSON

# List accounts
curl http://localhost:8080/api/accounts \
  -H "X-API-Key: your_admin_api_key_here"
# Expected: 200 with array of accounts

# Test webhook verification (simulates Meta handshake)
curl "http://localhost:8080/webhook/instagram?hub.mode=subscribe&hub.verify_token=test_verify&hub.challenge=challenge123"
# Expected: challenge123
```

## Running tests

```bash
# All tests
go test ./...

# Verbose output
go test -v ./...

# Specific package
go test -v ./tests/service/...

# With coverage
go test -cover ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

Note: Repository tests require the dev PostgreSQL to be running (docker compose up). Service and handler tests use mocks and don't require external services.

## Useful commands

| Command | What it does |
|---|---|
| `go run ./cmd/instasae/` | Start the server in dev mode |
| `go build -o instasae ./cmd/instasae/` | Build the binary |
| `go test ./...` | Run all tests |
| `go test -v -run TestName ./tests/...` | Run specific test |
| `migrate -database "$DATABASE_URL" -path migrations up` | Apply pending migrations |
| `migrate -database "$DATABASE_URL" -path migrations down 1` | Rollback last migration |
| `migrate -database "$DATABASE_URL" -path migrations version` | Show current migration version |
| `docker compose up -d` | Start dev databases |
| `docker compose down` | Stop dev databases |
| `docker compose down -v` | Stop and delete dev data |

## Port reference

| Service | Port | Notes |
|---|---|---|
| instasae | 8080 | Application HTTP server |
| PostgreSQL (dev) | 5433 | Mapped from container 5432 → host 5433 |
| Redis (dev) | 6380 | Mapped from container 6379 → host 6380 |

## Testing webhooks locally

To test real Instagram webhooks locally, use ngrok:

```bash
ngrok http 8080
# Copy the HTTPS URL (e.g. https://abc123.ngrok.io)
# Use this as webhook URL in Meta Developer Dashboard:
# https://abc123.ngrok.io/webhook/instagram
```
