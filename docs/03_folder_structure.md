# Folder Structure — instasae

## About this document

Maps every file and folder in the instasae project with explanations of what each one does.

## Repository root

```
instasae/
├── cmd/                    # Application entrypoints
├── internal/               # Private application code (Go convention)
├── migrations/             # SQL migration files
├── tests/                  # Test files with fixtures
├── docs/                   # Foundation documentation (8 docs)
├── CLAUDE.md               # Executive index for AI agent
├── docker-compose.yml      # Local development environment
├── docker-compose.prod.yml # Production stack
├── Dockerfile              # Multi-stage production build
├── .env.example            # Environment variable template
├── go.mod                  # Go module definition
├── go.sum                  # Go dependency checksums
└── README.md               # Project overview
```

## cmd/ — application entrypoints

```
cmd/
└── instasae/
    └── main.go             # Bootstrap: load config, connect DB/Redis, start server
```

### What main.go does
Loads configuration from env vars, initializes PostgreSQL pool, Redis client, B2 client, creates all repositories/services/handlers, registers routes, starts HTTP server with graceful shutdown on SIGTERM.

## internal/ — application code

```
internal/
├── config/
│   └── config.go           # Struct with all env vars, parsed via caarlos0/env
├── server/
│   ├── server.go           # HTTP server setup, graceful shutdown logic
│   └── routes.go           # All route registrations grouped by concern
├── handler/
│   ├── webhook_instagram.go # GET (verification) + POST (message events)
│   ├── webhook_chatwoot.go  # POST (callback from Chatwoot on agent reply)
│   ├── admin_accounts.go    # CRUD endpoints for account management
│   └── health.go            # GET /health — DB + Redis connectivity check
├── middleware/
│   ├── signature.go         # Validates X-Hub-Signature-256 from Meta
│   ├── auth.go              # API key validation for admin endpoints
│   └── logging.go           # Request/response logging with slog
├── service/
│   ├── instagram.go         # Process incoming IG webhook → create/route to Chatwoot
│   ├── chatwoot.go          # Process Chatwoot callback → send via IG Graph API
│   ├── media.go             # Download media from URL → upload to B2 → return public URL
│   └── account.go           # Business logic for account CRUD
├── client/
│   ├── instagram_client.go  # HTTP client for Instagram Graph API (send messages, get profile)
│   ├── chatwoot_client.go   # HTTP client for Chatwoot API (create contact, conversation, message)
│   └── b2_client.go         # S3-compatible client for Backblaze B2 uploads
├── repository/
│   ├── account_repo.go      # SQL queries for accounts table
│   ├── contact_repo.go      # SQL queries for contacts table
│   └── conversation_repo.go # SQL queries for conversations table
├── cache/
│   └── redis.go             # Cache operations (account lookup, dedup SET NX)
├── crypto/
│   └── encrypt.go           # AES-256-GCM encrypt/decrypt for tokens at rest
└── model/
    ├── account.go           # Account struct + creation/update params
    ├── contact.go           # Contact struct
    ├── conversation.go      # Conversation struct
    ├── instagram.go         # Instagram webhook payload structs
    └── chatwoot.go          # Chatwoot webhook + API payload structs
```

### What each folder does

**config/** — Single struct that maps every env var to a typed field. Parsed once at startup. Validation fails fast if required vars are missing.

**server/** — HTTP server lifecycle. `server.go` handles `ListenAndServe` + graceful shutdown (wait for in-flight requests on SIGTERM). `routes.go` registers all route groups with their middlewares.

**handler/** — HTTP handlers. Thin layer: decode request, call service, encode response. No business logic. Each handler file corresponds to one route group.

**middleware/** — Request interceptors. `signature.go` is applied only to the Instagram webhook route. `auth.go` is applied to admin routes. `logging.go` is applied globally.

**service/** — Business logic. `instagram.go` is the core: receives parsed webhook, validates, deduplicates, looks up account/contact/conversation, handles media, sends to Chatwoot. `chatwoot.go` handles the reverse flow. `media.go` handles download+upload of media files. `account.go` handles CRUD validation.

**client/** — External HTTP clients. Each client encapsulates the API contract of one external service. Includes retry logic, error wrapping, timeout configuration.

**repository/** — Database access. Direct SQL with pgx. Each repo covers one table. Methods return model structs. No raw SQL outside of repository files.

**cache/** — Redis operations. Account cache (GET/SET with TTL), webhook deduplication (SET NX with TTL).

**crypto/** — Token encryption/decryption. Used by repository layer when reading/writing token fields.

**model/** — Data structures. Go structs for domain models (Account, Contact, Conversation) and external payloads (Instagram webhook, Chatwoot webhook, API request/response bodies).

## migrations/ — SQL migration files

```
migrations/
├── 001_create_accounts.up.sql
├── 001_create_accounts.down.sql
├── 002_create_contacts.up.sql
├── 002_create_contacts.down.sql
├── 003_create_conversations.up.sql
└── 003_create_conversations.down.sql
```

Each migration has an `up` (apply) and `down` (rollback) file. Plain SQL, no DSL.

## tests/ — test suite

```
tests/
├── handler/
│   ├── webhook_instagram_test.go
│   ├── webhook_chatwoot_test.go
│   └── admin_accounts_test.go
├── service/
│   ├── instagram_test.go
│   ├── chatwoot_test.go
│   ├── media_test.go
│   └── account_test.go
├── repository/
│   ├── account_repo_test.go
│   ├── contact_repo_test.go
│   └── conversation_repo_test.go
├── fixtures/
│   ├── instagram_text_webhook.json
│   ├── instagram_image_webhook.json
│   ├── instagram_audio_webhook.json
│   ├── instagram_video_webhook.json
│   ├── instagram_echo_webhook.json
│   ├── chatwoot_outgoing_text.json
│   ├── chatwoot_outgoing_attachment.json
│   ├── chatwoot_outgoing_text_with_attachment.json
│   ├── chatwoot_incoming_ignored.json
│   └── chatwoot_private_note.json
└── testutil/
    └── helpers.go            # Shared test utilities, mock factories
```

Tests mirror the internal/ structure. Fixtures are real payloads captured from Instagram and Chatwoot with sensitive data replaced. Repository tests use a test database.

## Naming conventions

| Context | Convention | Example |
|---|---|---|
| Files | snake_case | `webhook_instagram.go` |
| Go packages | lowercase single word | `handler`, `service`, `model` |
| Go structs | PascalCase | `Account`, `WebhookPayload` |
| Go functions | PascalCase (exported), camelCase (private) | `ProcessMessage`, `parsePayload` |
| Go interfaces | PascalCase with -er suffix when applicable | `AccountRepository`, `MediaUploader` |
| DB tables | snake_case plural | `accounts`, `contacts` |
| DB columns | snake_case | `ig_page_id`, `created_at` |
| Env vars | UPPER_SNAKE_CASE | `DATABASE_URL`, `META_APP_SECRET` |
| Routes | kebab-case with resource plurals | `/api/accounts`, `/webhook/instagram` |
| Migration files | NNN_description.up.sql | `001_create_accounts.up.sql` |
| Test files | same_as_source_test.go | `instagram_test.go` |
| JSON fixtures | snake_case descriptive | `instagram_text_webhook.json` |
