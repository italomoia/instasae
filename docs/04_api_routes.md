# API Routes — instasae

## About this document

Lists every HTTP endpoint exposed by instasae, organized by concern.

## Authentication levels

| Level | Description |
|---|---|
| public | No authentication required. Used by Meta webhook verification. |
| meta_signed | Request must include valid X-Hub-Signature-256 header signed with Meta App Secret. |
| api_key | Request must include `X-API-Key` header matching the `ADMIN_API_KEY` env var. |

## Webhook routes

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/webhook/instagram` | public | Meta webhook verification handshake. Returns hub.challenge if hub.verify_token matches. |
| POST | `/webhook/instagram` | meta_signed | Receives Instagram messaging events. Responds 200 immediately, processes async. |
| POST | `/webhook/chatwoot` | api_key | Receives Chatwoot message_created callbacks when agents reply. |

Notes:

**GET /webhook/instagram**
Query params from Meta: `hub.mode` (must be "subscribe"), `hub.verify_token` (must match account's token or global verify token), `hub.challenge` (returned as response body).
Response: 200 with challenge value as plain text, or 403 if token mismatch.

**POST /webhook/instagram**
Headers: `X-Hub-Signature-256: sha256=<hmac>`, `Content-Type: application/json`
Body: Instagram webhook payload with `object: "instagram"` and `entry[].messaging[]` array.
Response: Always 200 (processing happens async in goroutine).
Processing: validate signature → parse → check is_echo → deduplicate → route to account → create/find contact → create/find conversation → handle media → send to Chatwoot.

**POST /webhook/chatwoot**
Headers: `X-API-Key: <key>`, `Content-Type: application/json`
Body: Chatwoot webhook payload with `event: "message_created"`.
Response: 200 on success, 400 if payload invalid, 404 if account not found.
Processing: filter outgoing non-private only → find account by inbox_id → find contact → determine message type (text only, attachment only, text+attachment) → send to Instagram (split if composite) → on error: send private note to Chatwoot.

## Admin API routes

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/accounts` | api_key | List all accounts with status info (active, token expiry). |
| GET | `/api/accounts/{id}` | api_key | Get single account details. |
| POST | `/api/accounts` | api_key | Create new account (connect IG account to Chatwoot inbox). |
| PUT | `/api/accounts/{id}` | api_key | Update account (change tokens, inbox, toggle active). |
| DELETE | `/api/accounts/{id}` | api_key | Soft-delete account (set is_active=false). |
| POST | `/api/accounts/{id}/refresh-token` | api_key | Manually trigger token refresh for an account. |
| GET | `/api/accounts/{id}/status` | api_key | Check account health: token validity, last message timestamp. |

Notes:

**POST /api/accounts** — Create account payload:
```json
{
  "ig_page_id": "17841451529395669",
  "ig_page_name": "Client Store",
  "ig_access_token": "IGAAINsJb4Rhh...",
  "chatwoot_base_url": "https://chat.example.com",
  "chatwoot_account_id": 1,
  "chatwoot_inbox_id": 5,
  "chatwoot_api_token": "qZgaibhBPQQq...",
  "webhook_verify_token": "my_custom_verify_token",
  "token_expires_at": "2026-05-15T00:00:00Z"
}
```

**PUT /api/accounts/{id}** — Any field can be updated individually. Tokens are re-encrypted on update.

**GET /api/accounts/{id}/status** — Returns:
```json
{
  "id": "uuid",
  "ig_page_name": "Client Store",
  "is_active": true,
  "token_expires_at": "2026-05-15T00:00:00Z",
  "token_days_remaining": 45,
  "contacts_count": 127,
  "active_conversations": 12,
  "last_inbound_message_at": "2026-03-16T14:30:00Z",
  "last_outbound_message_at": "2026-03-16T14:32:00Z"
}
```

## System routes

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | public | Returns 200 if DB and Redis are connected, 503 otherwise. |

Notes:

**GET /health** — Returns:
```json
{
  "status": "ok",
  "postgres": "connected",
  "redis": "connected",
  "accounts_active": 15,
  "uptime_seconds": 86400
}
```
