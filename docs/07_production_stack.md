# Production Stack — instasae

## About this document

Describes how instasae runs in production on Docker Swarm and how to deploy/update.

## Infrastructure

- **Orchestration:** Docker Swarm
- **Network:** IMSNet (external overlay network shared by all services)
- **Reverse proxy:** Traefik with `letsencryptresolver` for TLS
- **Container registry:** ghcr.io/italomoia/instasae:latest
- **Domain:** instasae.imsdigitais.com
- **PostgreSQL:** Existing container on IMSNet (`postgres:5432`)
- **Redis:** Existing container on IMSNet (`redis:6379/3` — index 3; 0=default, 2=n8n, 8=evolution)

## Prerequisites on the server

These steps are done once manually:

```bash
# 1. Create the instasae database on existing PostgreSQL
docker exec -it <postgres_container> psql -U postgres -c "CREATE DATABASE instasae;"
docker exec -it <postgres_container> psql -U postgres -c "CREATE USER instasae WITH PASSWORD 'STRONG_PASSWORD_HERE';"
docker exec -it <postgres_container> psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE instasae TO instasae;"
docker exec -it <postgres_container> psql -U postgres -d instasae -c "GRANT ALL ON SCHEMA public TO instasae;"

# 2. Configure DNS
# A record: instasae.imsdigitais.com → VPS_IP

# 3. Log in to GHCR (once per machine)
echo $GITHUB_TOKEN | docker login ghcr.io -u italomoia --password-stdin
```

## Dockerfile

```dockerfile
# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /instasae ./cmd/instasae/
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /instasae /usr/local/bin/instasae
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY migrations/ /migrations/

EXPOSE 8080

ENTRYPOINT ["instasae"]
```

Notes:
- `CGO_ENABLED=0` produces a static binary — no libc dependency.
- `-ldflags="-s -w"` strips debug symbols — smaller binary.
- `migrate` CLI is included for running migrations on deploy.
- Migrations SQL files are included in the image at `/migrations/`.
- Final image is ~15-25MB.

## Docker Swarm stack

File: `docker-compose.prod.yml`

```yaml
version: "3.7"
services:
  instasae:
    image: ghcr.io/italomoia/instasae:latest
    networks:
      - IMSNet
    environment:
      - PORT=8080
      - LOG_LEVEL=info
      - DATABASE_URL=postgres://instasae:${INSTASAE_DB_PASSWORD}@postgres:5432/instasae?sslmode=disable
      - REDIS_URL=redis://redis:6379/3
      - META_APP_SECRET=${META_APP_SECRET}
      - META_GRAPH_API_VERSION=v25.0
      - B2_ENDPOINT=${B2_ENDPOINT}
      - B2_REGION=${B2_REGION}
      - B2_BUCKET=${B2_BUCKET}
      - B2_KEY_ID=${B2_KEY_ID}
      - B2_APPLICATION_KEY=${B2_APPLICATION_KEY}
      - B2_PUBLIC_URL=${B2_PUBLIC_URL}
      - B2_PREFIX=instasae
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - ADMIN_API_KEY=${ADMIN_API_KEY}
      - WEBHOOK_VERIFY_TOKEN=${WEBHOOK_VERIFY_TOKEN}
    deploy:
      mode: replicated
      replicas: 1
      placement:
        constraints:
          - node.role == manager
      resources:
        limits:
          cpus: "0.5"
          memory: 128M
      labels:
        - traefik.enable=true
        - traefik.http.routers.instasae.rule=Host(`instasae.imsdigitais.com`)
        - traefik.http.routers.instasae.entrypoints=websecure
        - traefik.http.routers.instasae.priority=1
        - traefik.http.routers.instasae.tls.certresolver=letsencryptresolver
        - traefik.http.routers.instasae.service=instasae
        - traefik.http.services.instasae.loadbalancer.server.port=8080
        - traefik.http.services.instasae.loadbalancer.passHostHeader=true

networks:
  IMSNet:
    external: true
    name: IMSNet
```

Notes:
- Redis uses database index 3 (`/3`) — indexes 0=default, 2=n8n, 8=evolution.
- `postgres` and `redis` hostnames resolve via the IMSNet overlay network.
- Traefik labels configure HTTPS with automatic Let's Encrypt via `letsencryptresolver`.
- Resource limits: 0.5 CPU, 128MB RAM.

## Build and push (from dev machine)

```bash
# Build and push to GHCR
./deploy.sh

# Or manually:
docker build -t ghcr.io/italomoia/instasae:latest .
docker push ghcr.io/italomoia/instasae:latest
```

## First deploy (on VPS)

```bash
# 1. Set environment variables (add to /etc/environment or export)
export INSTASAE_DB_PASSWORD="STRONG_PASSWORD_HERE"
export META_APP_SECRET="..."
export B2_ENDPOINT="..."
# ... (all vars from .env.example)

# 2. Pull the image
docker pull ghcr.io/italomoia/instasae:latest

# 3. Run migrations
docker run --rm --network IMSNet \
  ghcr.io/italomoia/instasae:latest \
  sh -c "migrate -database 'postgres://instasae:${INSTASAE_DB_PASSWORD}@postgres:5432/instasae?sslmode=disable' -path /migrations up"

# 4. Deploy the stack
docker stack deploy -c docker-compose.prod.yml instasae

# 5. Verify
curl https://instasae.imsdigitais.com/health
# Expected: {"status":"ok",...}

# 6. Configure Meta webhook
# In Meta Developer Dashboard:
# Callback URL: https://instasae.imsdigitais.com/webhook/instagram
# Verify Token: value from WEBHOOK_VERIFY_TOKEN env var
# Subscribe to: messages
```

## Updating

```bash
# On dev machine:
./deploy.sh

# On VPS:

# 1. Pull new image
docker pull ghcr.io/italomoia/instasae:latest

# 2. Run new migrations (if any)
docker run --rm --network IMSNet \
  ghcr.io/italomoia/instasae:latest \
  sh -c "migrate -database 'postgres://instasae:${INSTASAE_DB_PASSWORD}@postgres:5432/instasae?sslmode=disable' -path /migrations up"

# 3. Update the service (rolling update)
docker service update --image ghcr.io/italomoia/instasae:latest instasae_instasae --force

# 4. Verify
curl https://instasae.imsdigitais.com/health
docker service logs --tail=20 instasae_instasae
```

## Useful commands

| Command | What it does |
|---|---|
| `docker service logs -f instasae_instasae` | Follow logs |
| `docker service logs --tail=100 instasae_instasae` | Last 100 lines |
| `docker service update --force instasae_instasae` | Restart (rolling) |
| `docker service scale instasae_instasae=0` | Stop |
| `docker service scale instasae_instasae=1` | Start |
| `docker stack ps instasae` | Status of all tasks |
| `docker stack rm instasae` | Remove the entire stack |

## Environment variable checklist

```
[ ] INSTASAE_DB_PASSWORD — strong password for instasae postgres user
[ ] PORT=8080
[ ] LOG_LEVEL=info
[ ] DATABASE_URL — constructed from INSTASAE_DB_PASSWORD in docker-compose.prod.yml
[ ] REDIS_URL — redis://redis:6379/3 (index 3)
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
