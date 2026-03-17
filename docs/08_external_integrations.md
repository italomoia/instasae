# External Integrations — instasae

## About this document

Documents every interaction with external services: what they are, how they work, endpoints used, payloads, configuration, and error handling.

## 1. Instagram Graph API (Messaging)

**What it is:** Meta's API for sending and receiving Instagram Direct Messages programmatically.

**How it works:**
- **Inbound:** Meta sends webhook POST events to instasae when a customer sends a DM to a connected Instagram professional account.
- **Outbound:** instasae sends POST requests to the Graph API to deliver agent replies to customers.

**Base URL:** `https://graph.instagram.com/{API_VERSION}`

**API Version:** v25.0 (pinned via env var `META_GRAPH_API_VERSION`)

### Webhook Verification (inbound, one-time setup)

**Endpoint:** `GET https://${INSTASAE_DOMAIN}/webhook/instagram`

Meta sends this during webhook setup in the Developer Dashboard:
```
GET /webhook/instagram?hub.mode=subscribe&hub.verify_token=YOUR_TOKEN&hub.challenge=CHALLENGE_STRING
```
instasae returns the `hub.challenge` value as plain text if the verify_token matches.

### Webhook Events (inbound, real-time)

**Endpoint:** `POST https://${INSTASAE_DOMAIN}/webhook/instagram`

**Headers from Meta:**
```
Content-Type: application/json
X-Hub-Signature-256: sha256=<HMAC_SHA256_OF_BODY>
```

**Payload — text message:**
```json
{
  "object": "instagram",
  "entry": [
    {
      "time": 1773581417513,
      "id": "17841451529395669",
      "messaging": [
        {
          "sender": {"id": "1282743370242395"},
          "recipient": {"id": "17841451529395669"},
          "timestamp": 1773581416309,
          "message": {
            "mid": "aWdfZAG1faXR...",
            "text": "Hello!"
          }
        }
      ]
    }
  ]
}
```

**Payload — image attachment:**
```json
{
  "object": "instagram",
  "entry": [
    {
      "time": 1773581417513,
      "id": "17841451529395669",
      "messaging": [
        {
          "sender": {"id": "1282743370242395"},
          "recipient": {"id": "17841451529395669"},
          "timestamp": 1773581416309,
          "message": {
            "mid": "aWdfZAG1faXR...",
            "attachments": [
              {
                "type": "image",
                "payload": {
                  "url": "https://lookaside.fbsbx.com/ig_messaging_cdn/..."
                }
              }
            ]
          }
        }
      ]
    }
  ]
}
```

**Payload — echo message (sent by account owner):**
```json
{
  "messaging": [
    {
      "sender": {"id": "17841451529395669"},
      "recipient": {"id": "1282743370242395"},
      "timestamp": 1773581416309,
      "message": {
        "mid": "aWdfZAG1faXR...",
        "is_echo": true,
        "text": "Reply from Instagram app"
      }
    }
  ]
}
```

### Send Text Message (outbound)

**Endpoint:** `POST https://graph.instagram.com/v25.0/{IG_PAGE_ID}/messages`

**Headers:**
```
Authorization: Bearer {IG_ACCESS_TOKEN}
Content-Type: application/json
```

**Payload:**
```json
{
  "recipient": {"id": "{IGSID}"},
  "message": {"text": "Hello from the agent!"},
  "tag": "HUMAN_AGENT"
}
```

**Success response:**
```json
{"recipient_id": "1282743370242395", "message_id": "m_..."}
```

### Send Attachment (outbound)

**Endpoint:** Same as above.

**Payload (image example):**
```json
{
  "recipient": {"id": "{IGSID}"},
  "message": {
    "attachment": {
      "type": "image",
      "payload": {"url": "https://public-url.com/image.jpg"}
    }
  },
  "tag": "HUMAN_AGENT"
}
```

Supported types: `image`, `audio`, `video`, `file`.

### Get User Profile (outbound)

**Endpoint:** `GET https://graph.instagram.com/v25.0/{IGSID}?fields=name,username,profile_pic`

**Headers:**
```
Authorization: Bearer {IG_ACCESS_TOKEN}
```

**Response:**
```json
{
  "name": "John Doe",
  "username": "johndoe",
  "profile_pic": "https://...",
  "id": "1282743370242395"
}
```

### Configuration

| Env var | Description |
|---|---|
| `META_APP_SECRET` | App Secret from Meta Developer Dashboard. Used for webhook signature validation. |
| `META_GRAPH_API_VERSION` | API version (v25.0). Pinned to avoid breaking changes. |

Per-account in database: `ig_access_token`, `ig_page_id`.

### Error handling

| Error | Cause | Action |
|---|---|---|
| 400 Invalid recipient | IGSID doesn't exist or blocked the account | Log + private note in Chatwoot |
| 400 Outside allowed window | More than 7 days since last customer message | Log + private note |
| 401 Invalid token | Token expired or revoked | Log ERROR + private note + mark account for token refresh |
| 429 Rate limit | More than 200 requests/hour/account | Retry with backoff |
| 500/503 Server error | Meta temporary issue | Retry with backoff (max 3 attempts) |

---

## 2. Chatwoot API

**What it is:** Self-hosted Chatwoot instance API for managing contacts, conversations, and messages.

**How it works:**
- **Inbound (to Chatwoot):** instasae creates contacts, conversations, and messages via Chatwoot's Application API.
- **Outbound (from Chatwoot):** Chatwoot sends webhook callbacks to instasae when agents send messages.

**Base URL:** Per-account, stored in `accounts.chatwoot_base_url`.

### Create Contact

**Endpoint:** `POST {CHATWOOT_BASE_URL}/api/v1/accounts/{ACCOUNT_ID}/contacts`

**Headers:**
```
api_access_token: {CHATWOOT_API_TOKEN}
Content-Type: application/json
```

**Payload:**
```json
{
  "inbox_id": 5,
  "name": "John Doe",
  "identifier": "ig_1282743370242395",
  "avatar_url": "https://profile-pic-url...",
  "custom_attributes": {
    "instagram_username": "johndoe",
    "instagram_id": "1282743370242395"
  }
}
```

**Response (relevant fields):**
```json
{
  "payload": {
    "contact": {
      "id": 42,
      "name": "John Doe",
      "contact_inboxes": [
        {
          "source_id": "abc123-uuid",
          "inbox": {"id": 5}
        }
      ]
    }
  }
}
```

The `source_id` from `contact_inboxes` is saved and used to create conversations.

### Create Conversation

**Endpoint:** `POST {CHATWOOT_BASE_URL}/api/v1/accounts/{ACCOUNT_ID}/conversations`

**Payload:**
```json
{
  "source_id": "abc123-uuid",
  "inbox_id": 5,
  "contact_id": 42,
  "status": "open"
}
```

### Create Message (incoming from customer)

**Endpoint:** `POST {CHATWOOT_BASE_URL}/api/v1/accounts/{ACCOUNT_ID}/conversations/{CONV_ID}/messages`

**Payload — text:**
```json
{
  "content": "Hello!",
  "message_type": "incoming"
}
```

**Payload — with attachment (image URL from B2):**
Multipart form data is used for attachments. Alternatively, the content can reference the media URL.

### Create Private Note (error notification)

**Endpoint:** Same as create message.

**Payload:**
```json
{
  "content": "⚠️ Message not delivered to Instagram: Token expired (code: 190)",
  "message_type": "outgoing",
  "private": true
}
```

### Callback Webhook (outbound from Chatwoot)

**Endpoint:** `POST https://${INSTASAE_DOMAIN}/webhook/chatwoot`

Configured in the Chatwoot inbox settings as the "Callback URL".

**Payload (message_created event):**
```json
{
  "event": "message_created",
  "id": 123,
  "content": "Hi there, how can I help?",
  "message_type": "outgoing",
  "private": false,
  "content_type": "text",
  "conversation": {
    "id": 45,
    "inbox_id": 5,
    "contact_last_seen_at": 0
  },
  "sender": {
    "id": 1,
    "name": "Agent Name",
    "type": "user"
  },
  "inbox": {
    "id": 5,
    "name": "Instagram DM"
  },
  "account": {
    "id": 1,
    "name": "My Agency"
  }
}
```

### Configuration

Per-account in database: `chatwoot_base_url`, `chatwoot_account_id`, `chatwoot_inbox_id`, `chatwoot_api_token`.

### Error handling

| Error | Cause | Action |
|---|---|---|
| 401 Unauthorized | Invalid or expired API token | Log ERROR |
| 404 Not Found | Contact or conversation deleted | Log + skip |
| 422 Unprocessable | Invalid payload (missing fields) | Log ERROR with payload for debugging |
| 500/503 Server error | Chatwoot temporary issue | Retry with backoff (max 3) |

---

## 3. Backblaze B2 (S3-compatible)

**What it is:** Object storage for persisting media files (images, audio, video) downloaded from Instagram's temporary CDN.

**How it works:** instasae downloads media from the Meta CDN URL, uploads to B2 via S3-compatible API, and uses the public B2 URL in Chatwoot messages.

**Endpoint:** S3-compatible, configured via env vars.

**Upload path pattern:** `{B2_PREFIX}/{account_id}/{YYYY-MM-DD}/{uuid}.{ext}`

Example: `instasae/abc123/2026-03-16/550e8400-e29b.jpg`

**Public URL pattern:** `{B2_PUBLIC_URL}/{path}`

### Configuration

| Env var | Description |
|---|---|
| `B2_ENDPOINT` | S3-compatible endpoint URL |
| `B2_REGION` | B2 region |
| `B2_BUCKET` | Bucket name (shared with Chatwoot) |
| `B2_KEY_ID` | Application key ID |
| `B2_APPLICATION_KEY` | Application key secret |
| `B2_PUBLIC_URL` | Public base URL for the bucket |
| `B2_PREFIX` | Folder prefix (instasae) to isolate from Chatwoot files |

### Error handling

| Error | Cause | Action |
|---|---|---|
| Upload fails | B2 unreachable, auth issue | Forward text content without media, log ERROR |
| Download fails | Meta CDN URL expired mid-download | Forward text content without media, log WARNING |
| File too large | >25MB | Skip media, forward text, log WARNING |

---

## Configuration checklist

### Meta Developer Dashboard
```
[ ] App created with Messenger + Webhooks products
[ ] instagram_basic permission granted
[ ] instagram_manage_messages permission granted (Advanced Access)
[ ] human_agent permission approved via App Review
[ ] Webhook callback URL: https://${INSTASAE_DOMAIN}/webhook/instagram
[ ] Webhook verify token matches WEBHOOK_VERIFY_TOKEN env var
[ ] Subscribed to: messages field under Instagram
[ ] Page subscriptions enabled for each client's IG page
```

### Chatwoot
```
[ ] API channel inbox created per client
[ ] Callback URL set to: https://${INSTASAE_DOMAIN}/webhook/chatwoot
[ ] API access token generated (user or bot)
[ ] Account ID and Inbox ID noted for each client
```

### Backblaze B2
```
[ ] Bucket exists (shared with Chatwoot)
[ ] Application key with read+write access to bucket
[ ] Public file access enabled on bucket
[ ] Key ID and Application Key stored in env vars
```
