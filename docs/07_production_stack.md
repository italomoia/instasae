# Production Stack — instasae

## About this document

Describes how instasae runs in production and how to deploy/update.

## Prerequisites on the server

These steps are done once manually:

```bash
# 1. Create the instasae database on existing PostgreSQL
docker exec -it <postgres_container> psql -U postgres -c "CREATE DATABASE instasae;"
docker exec -it <postgres_container> psql -U postgres -c "CREATE USER instasae WITH PASSWORD 'STRONG_PASSWORD_HERE';"
docker exec -it <postgres_container> psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE instasae TO instasae;"
docker exec -it <postgres_container> psql -U postgres -d instasae -c "GRANT ALL ON SCHEMA public TO instasae;"

# 2. Configure DNS
# A record: ${INSTASAE_DOMAIN} → ${VPS_IP}

# 3. Create directory for the project
mkdir -p /opt/instasae
```

## Dockerfile

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /instasae ./cmd/instasae/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /instasae /usr/local/bin/instasae
COPY migrations/ /migrations/

EXPOSE 8080

ENTRYPOINT ["instasae"]
```

Notes:
- `CGO_ENABLED=0` produces a static binary — no libc dependency.
- `-ldflags="-s -w"` strips debug symbols — smaller binary.
- Migrations are included in the image for running on startup or via CLI.
- Final image is ~15-20MB.

## Docker Compose production

```yaml
services:
  instasae:
    image: instasae:latest
    build:
      context: .
      dockerfile: Dockerfile
    restart: unless-stopped
    environment:
      PORT: "8080"
      LOG_LEVEL: "info"
      DATABASE_URL: "postgres://instasae:STRONG_PASSWORD_HERE@postgres:5432/instasae?sslmode=disable"
      REDIS_URL: "redis://redis:6379/2"
      META_APP_SECRET: "${META_APP_SECRET}"
      META_GRAPH_API_VERSION: "v25.0"
      B2_ENDPOINT: "${B2_ENDPOINT}"
      B2_REGION: "${B2_REGION}"
      B2_BUCKET: "${B2_BUCKET}"
      B2_KEY_ID: "${B2_KEY_ID}"
      B2_APPLICATION_KEY: "${B2_APPLICATION_KEY}"
      B2_PUBLIC_URL: "${B2_PUBLIC_URL}"
      B2_PREFIX: "instasae"
      ENCRYPTION_KEY: "${ENCRYPTION_KEY}"
      ADMIN_API_KEY: "${ADMIN_API_KEY}"
      WEBHOOK_VERIFY_TOKEN: "${WEBHOOK_VERIFY_TOKEN}"
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.instasae.rule=Host(`${INSTASAE_DOMAIN}`)"
      - "traefik.http.routers.instasae.entrypoints=websecure"
      - "traefik.http.routers.instasae.tls.certresolver=letsencrypt"
      - "traefik.http.services.instasae.loadbalancer.server.port=8080"
    networks:
      - traefik_network
      - internal

networks:
  traefik_network:
    external: true
  internal:
    external: true
```

Notes:
- Redis uses database index 2 (`/2`) to avoid collision with Chatwoot.
- `postgres` and `redis` hostnames refer to the existing containers on the Docker network.
- Traefik labels configure HTTPS with automatic Let's Encrypt.
- Networks must match the existing Traefik and internal network names.

## Build and push

```bash
# Build on the server (simplest approach for single VPS)
cd /opt/instasae
git pull origin main
docker compose build instasae
```

## First deploy

```bash
# 1. Clone to server
cd /opt
git clone ${REPO_URL} instasae
cd instasae

# 2. Create .env with production values
cp .env.example .env
nano .env   # fill in all STRONG_PASSWORD, keys, tokens

# 3. Build the image
docker compose -f docker-compose.prod.yml build

# 4. Run migrations
docker compose -f docker-compose.prod.yml run --rm instasae \
  sh -c "migrate -database '$DATABASE_URL' -path /migrations up"

# 5. Start the service
docker compose -f docker-compose.prod.yml up -d

# 6. Verify
curl https://${INSTASAE_DOMAIN}/health
# Expected: {"status":"ok",...}

# 7. Configure Meta webhook
# In Meta Developer Dashboard:
# Callback URL: https://${INSTASAE_DOMAIN}/webhook/instagram
# Verify Token: value from WEBHOOK_VERIFY_TOKEN env var
# Subscribe to: messages
```

## Updating

```bash
cd /opt/instasae

# 1. Pull latest code
git pull origin main

# 2. Rebuild image
docker compose -f docker-compose.prod.yml build

# 3. Run new migrations (if any)
docker compose -f docker-compose.prod.yml run --rm instasae \
  sh -c "migrate -database '$DATABASE_URL' -path /migrations up"

# 4. Restart with new image (graceful shutdown)
docker compose -f docker-compose.prod.yml up -d

# 5. Verify
curl https://${INSTASAE_DOMAIN}/health
docker compose -f docker-compose.prod.yml logs --tail=20 instasae
```

## Useful commands

| Command | What it does |
|---|---|
| `docker compose -f docker-compose.prod.yml logs -f instasae` | Follow logs |
| `docker compose -f docker-compose.prod.yml logs --tail=100 instasae` | Last 100 lines |
| `docker compose -f docker-compose.prod.yml restart instasae` | Restart (graceful) |
| `docker compose -f docker-compose.prod.yml stop instasae` | Stop |
| `docker compose -f docker-compose.prod.yml ps` | Status |
| `docker compose -f docker-compose.prod.yml exec instasae sh` | Shell into container |

## Environment variable checklist

```
[ ] PORT=8080
[ ] LOG_LEVEL=info
[ ] DATABASE_URL — with STRONG password, pointing to existing postgres container
[ ] REDIS_URL — pointing to existing redis container, database index /2
[ ] META_APP_SECRET — from Meta Developer Dashboard → App Settings → Basic
[ ] META_GRAPH_API_VERSION — v25.0
[ ] B2_ENDPOINT — Backblaze B2 S3-compatible endpoint
[ ] B2_REGION — Backblaze region
[ ] B2_BUCKET — existing bucket name
[ ] B2_KEY_ID — B2 application key ID
[ ] B2_APPLICATION_KEY — B2 application key
[ ] B2_PUBLIC_URL — public URL for the bucket
[ ] B2_PREFIX — instasae
[ ] ENCRYPTION_KEY — 64 hex chars (32 bytes), generated with: openssl rand -hex 32
[ ] ADMIN_API_KEY — strong random string for admin API auth
[ ] WEBHOOK_VERIFY_TOKEN — string set in Meta webhook config
```
